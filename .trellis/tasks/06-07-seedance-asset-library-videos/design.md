# Seedance 资产库异步任务设计

## Architecture

本需求复用现有 `/v1/videos` 任务链路，不新增下游公开端点。

```text
Client
  -> NewAPI POST /v1/videos
  -> NewAPI TokenAuth + Distribute + RelayTask
  -> Sora/OpenAI Video task adaptor
  -> seedance-direct-proxy POST /v1/videos
  -> Seedance /resources/user/Resources

NewAPI TaskPollingLoop
  -> Sora/OpenAI Video task adaptor FetchTask
  -> seedance-direct-proxy GET /v1/videos/{asset_req_id}
  -> Seedance /resources/user/ResourcesList

Client
  -> NewAPI POST /api/task/token/asset/delete
  -> NewAPI TokenAuthReadOnly + task ownership check
  -> seedance-direct-proxy POST /api/task/token/asset/delete
  -> seedance-direct-proxy ResourcesList resolves task_id to resource_id
  -> Seedance DELETE /resources/user/Resources
```

## Request Contract

资产创建仍使用 `/v1/videos`：

```json
{
  "model": "seedance-asset",
  "prompt": "林春芽",
  "input_reference": "https://example.com/person.png"
}
```

兼容图片字段：

- `input_reference`
- `image`
- `images[0]`
- `files[0]`

`prompt` 作为资源名称/资产显示名，例如真人形象名。它会映射到上游资产库请求体的 `Name`，不是 NewAPI 用户名或上游创建者名称。若为空，仍遵循现有 NewAPI 视频任务校验，返回缺少 prompt 的错误。

资产图片只接受公网 URL：

- URL 必须以 `http://` 或 `https://` 开头。
- URL host 不能为空，显式 localhost、回环 IP、私网 IP、链路本地地址必须拒绝。
- 不支持 multipart 文件、本地文件路径、base64 或 data URL。
- 如果请求中没有可用 URL，代理返回 `400 invalid_request`。
- MVP 不做 DNS 解析、不做 DNS 重绑定防护、不预取图片内容。
- 图片字段优先级固定为 `input_reference -> image -> images[0] -> files[0]`。
- 同一请求传入多张图片时，只取第一张创建一个资产，并在响应 metadata 中记录 `ignored_image_count`。

## Proxy Behavior

`seedance-direct-proxy` 在 `POST /v1/videos` 中识别资产模型：

1. 解析资产请求。
2. 生成公开任务 ID，例如 `asset_req_<unix>_<random>`，其中 `<unix>` 为创建时间戳，`<random>` 为 12 位小写 hex。
3. 将资源名称和短追踪后缀合成上游 `Name`，例如 `林春芽__ar_abcdef123456`。
4. 上游资产库 `Name` 最大 50 个 Unicode 字符；短追踪后缀占 17 字符，所以下游资产显示名最大 33 个 Unicode 字符。
5. 调用上游：

```http
POST /resources/user/Resources
token: <upstream token>
Content-Type: application/json
```

```json
{
  "Name": "林春芽__ar_abcdef123456",
  "OssPath": "https://example.com/person.png"
}
```

6. 返回 OpenAI Video 风格异步任务对象：

```json
{
  "id": "asset_req_1780830000_xxx",
  "task_id": "asset_req_1780830000_xxx",
  "object": "video",
  "model": "seedance-asset",
  "status": "queued",
  "progress": 0,
  "metadata": {
    "seedance": {
      "kind": "asset",
      "name": "林春芽",
      "oss_path": "https://example.com/person.png",
      "ignored_image_count": 0
    }
  }
}
```

## Query Behavior

`seedance-direct-proxy` 在 `GET /v1/videos/{id}` 中识别 `asset_req_` 前缀：

1. 从 `asset_req_<unix>_<random>` 解析创建时间，按任务年龄计算资源列表扫描页数：
   - 0-10 分钟：默认 10 页。
   - 10-60 分钟：默认 20 页。
   - 60 分钟以后：默认 50 页。
   - 页数通过环境变量调整。
2. 调用上游资源列表接口：

```http
POST /resources/user/ResourcesList
token: <upstream token>
Content-Type: application/json
```

```json
{"Page": 1}
```

3. 从任务 ID 提取 `<random>`，在返回列表中查找 `Name` 以完整短追踪后缀 `__ar_<random>` 结尾的资源，不能使用包含匹配。
4. 从上游 `Name` 中按最后一次出现的 `__ar_` 剥离追踪后缀，作为下游展示名称。
5. 根据上游资源项归一化状态：
   - 未找到或已找到但还没有 `AssetId`，且无明确失败信息：`in_progress`，`progress=50`。
   - `AssetId` 非空：`completed`。
   - 上游资源列表接口失败，或资源项 `Message`/`StatusText` 明确包含失败语义：`failed`。
6. 返回 OpenAI Video 风格响应，并在 metadata 中携带资产信息。
7. 资产成功时，同时返回顶层 `asset_id` 和 `metadata.seedance.asset_id`，下游可直接读取顶层字段。

## Delete Behavior

资产删除使用 NewAPI 后台 API 风格，不新增 `DELETE /v1/videos/{task_id}`，降低对 OpenAI Videos relay 路由的改动面。

下游请求：

```http
POST /api/task/token/asset/delete
Authorization: Bearer <NewAPI API Key>
Content-Type: application/json
```

```json
{
  "task_id": "asset_req_1780830000_xxx"
}
```

MVP 只接受 `task_id`。不接受下游直接传 `resource_id` 删除；`asset_id` 反查删除暂不实现。

NewAPI 删除流程：

1. 使用 `TokenAuthReadOnly()` 读取当前 `user_id` 和 `token_id`。
2. 按 `user_id + token_id + task_id` 查询本地任务，确保只操作当前 API Key 创建的任务。
3. 校验任务是已成功的 `seedance-asset` 资产任务：
   - `status=SUCCESS`。
   - `data.model == "seedance-asset"`，或 `properties.origin_model_name == "seedance-asset"`，或 `data.metadata.seedance.kind == "asset"`。
   - `data.deleted` 和 `data.metadata.seedance.deleted` 均不为 `true`。
4. 从任务 `PrivateData.UpstreamTaskID` 读取上游任务 ID，旧数据无该字段时回退 `TaskID`。
5. 调 `seedance-direct-proxy` 的无状态删除转发接口 `POST /api/task/token/asset/delete`。
6. 删除成功后更新原任务 `data`，保留历史记录并标记 `deleted=true`、`deleted_at`。
7. 返回删除结果，供下游确认该资产不再可用。

删除成功响应：

```json
{
  "success": true,
  "data": {
    "task_id": "asset_req_1780830000_xxx",
    "deleted": true,
    "deleted_at": 1780830000,
    "asset_id": "asset-xxx",
    "asset_uri": "asset://asset-xxx"
  }
}
```

`seedance-direct-proxy` 保持无状态，提供和 NewAPI 同名的上游动作接口：

```http
POST /api/task/token/asset/delete
Authorization: Bearer <Seedance upstream token>
Content-Type: application/json
```

```json
{"task_id":"asset_req_1780830000_xxx"}
```

代理按 `task_id` 查询 `/resources/user/ResourcesList`，找到真实资源 ID 后调用上游：

```http
DELETE /resources/user/Resources
token: <upstream token>
Content-Type: application/json
```

```json
{"Id": 123}
```

代理只校验 `task_id` 为严格 `asset_req_<unix>_<12位小写hex>` 格式，并透传上游业务失败。权限判断全部在 NewAPI。

删除后的列表语义：

- `/api/task/token/self` 仍是任务历史，可以返回已删除资产任务。
- 下游展示“可用真人资产”时必须排除 `data.deleted=true` 或 `data.metadata.seedance.deleted=true`。
- 删除不触发退款；资产曾经成功创建，删除是后续资源生命周期操作。

## NewAPI Behavior

NewAPI 尽量少改：

- 通过模型配置让 `seedance-asset` 能选中 Seedance 代理渠道。
- 继续使用 `POST /v1/videos` 和 `GET /v1/videos/:task_id`。
- 继续通过 `model.Task` 保存任务，利用 `user_id` 实现用户隔离。
- 继续通过 `TaskPollingLoop` 异步更新任务。
- 资产任务作为虚拟模型按次计费，提交成功即扣费；失败或超时沿用现有任务退款逻辑。
- 资产模型不使用视频时长、分辨率或参考视频倍率。
- Sora/OpenAI Video task adaptor 需要识别 `seedance-asset`：
  - 校验只要求 `model`、资源名称/资产显示名和公网图片 URL。
  - `EstimateBilling` 对资产任务返回空倍率，避免套用 `seconds`、`size` 或参考视频倍率。
  - `Action` 可继续使用现有生成类 action，除非实现中需要常量区分。
- MVP 不新增专用资产列表 API；已通过资产通过现有任务记录按资产模型和成功状态过滤，单个资产通过 `GET /v1/videos/:task_id` 查询。
- 不把 `seedance-asset` 写入 NewAPI 默认模型/价格表；由管理员在后台手动配置模型、渠道和按次价格。

NewAPI 需要的小改动：

- Sora/OpenAI Video adaptor 的校验和计费估算需要为 `seedance-asset` 分支绕过视频参数与倍率。
- Sora/OpenAI Video adaptor 的 `ParseTaskResult` 当前只把 `completed` 映射为成功，资产代理返回同样状态即可复用。
- `ConvertToOpenAIVideo` 需要保留顶层 `asset_id` 和 metadata，方便用户从任务查询结果中拿 `AssetId`。
- 新增 `POST /api/task/token/asset/delete`，使用 `TokenAuthReadOnly()`，不走 relay 分发和计费。
- 新增按 `user_id + token_id + task_id` 获取单个任务的模型方法，或复用现有查询后补 token 校验。
- 新增任务 data 标记 deleted 的更新逻辑。

## Compatibility

- 普通视频模型不走资产分支，保持现有视频创建和查询行为。
- 资产模型是显式虚拟模型，不会误伤真实视频模型。
- 下游端点不变，SDK/客户端只需把模型名和图片 URL 换成资产任务约定。
- 下游资产查询能力优先复用现有任务查询/日志能力；删除能力新增后台 API 风格路由 `/api/task/token/asset/delete`。
- 需要实例管理员配置 `seedance-asset`，否则 NewAPI 可能无法选中渠道或完成计费。
- 资产响应保持 `object: "video"` 以兼容 `/v1/videos` 客户端，通过 `asset_id` 和 `metadata.seedance.kind="asset"` 表达资产语义。

## Trade-offs

- 优点：最大化复用 NewAPI 现有鉴权、渠道、任务、计费、轮询、用户隔离。
- 缺点：`/v1/videos` 语义变宽，资产任务不是视频任务；需要文档明确 `model=seedance-asset` 是资产模式。
- 风险：上游资源列表分页和搜索能力未知。MVP 根据任务年龄从 10 页逐步增加到 20/50 页，后续若上游支持按名称查询再优化。

## Operations

- 删除资产对下游开放时，仅通过 NewAPI API Key 权限校验后的后台 API 入口开放。
- 资产任务长期没有 `AssetId` 时，按 NewAPI 现有任务超时机制失败并退款。
- 上游资产状态枚举未知时采用保守策略：除明确失败外，无 `AssetId` 一律视为处理中。
- 日志不要输出上游 token。
- 图片 URL 校验为轻量防护，主要拦截显式危险 host/IP；完整 SSRF 防护不在本阶段范围。

## Jimeng Upstream Addendum

2026-06-26 新增新上游接入。老视频上游、老资产库上游和资产删除转发必须保留，新增能力通过整实例配置切换启用。

配置：

```text
VIDEO_UPSTREAM_PROVIDER=jimeng   # 默认，普通视频创建/查询走新上游
VIDEO_UPSTREAM_PROVIDER=legacy   # 老渠道恢复后显式切回
JIMENG_UPSTREAM_BASE_URL=https://api.aizhw.cc
```

如果 `VIDEO_UPSTREAM_PROVIDER` 是未知值，代理回退到 `jimeng` 并记录 warning 日志。当前老渠道不可用，错误配置时优先保证新渠道可用。

新上游数据流：

```text
Client/NewAPI POST /v1/videos
  -> seedance-direct-proxy
  -> POST /v1/videos/tasks

Client/NewAPI GET /v1/videos/{task_id}
  -> seedance-direct-proxy
  -> GET /v1/videos/tasks/{task_id}
```

请求改写：

- `model` 必填，直接透传，不做模型映射，也不补默认模型。
- `prompt` 直接透传，不自动改写 `@图片1`、`@图片2` 等素材引用占位符；新渠道下游文档只推荐 `@1`、`@2`。
- `ratio` 优先透传；未传 `ratio` 时 `aspect_ratio` 映射为 `ratio`；两者都缺失时不补默认值，交给新上游默认。
- `resolution` 透传；缺失时默认 `720p`。
- `duration` 优先，未传时使用 `seconds`，两者都缺失时默认 `4`；下游传字符串或数字都接受，提交给新上游时统一为 JSON number。
- 代理不校验 `duration` 范围或模型时长上限，相关错误交给新上游返回后统一归一化。
- `resolution`、`reference_mode` 如果存在则透传。
- `reference_mode` 缺失时由代理补默认值：有参考 URL 为 `omni`，无参考 URL 为 `text_to_video`。
- `reference_mode` 是新渠道正式暴露给下游的能力，至少覆盖 `omni`、`first_frame`、`last_frame`、`both_frames`、`text_to_video`。
- `reference_mode` 只支持英文枚举，不做中文别名转换。
- `reference_mode=omni` 时 URL 数量必须为 1-12 个。
- `reference_mode=text_to_video` 时 URL 数量必须为 0。
- `reference_mode=first_frame` 或 `last_frame` 时 URL 数量必须正好 1 个。
- `reference_mode=both_frames` 时，`file_paths[0]` 是首帧，`file_paths[1]` 是尾帧；URL 数量必须正好 2 个。
- `files`、`input_reference`、`file_paths`、`filePaths` 四类参考素材字段收集为新上游 `file_paths` 语义。
- 收集顺序固定为 `files -> input_reference -> file_paths -> filePaths`，保留重复 URL 和原始顺序；`both_frames` 可用同一 URL 同时作为首帧和尾帧。
- 面向下游的新渠道文档只推荐 `files`；`input_reference`、`file_paths`、`filePaths` 仅作为兼容输入保留。
- 公网 URL 原样转发为 `file_paths`，代理不下载、不转存、不 multipart 上传。
- URL 校验沿用现有轻量公网 URL 安全校验：只接受 `http://` / `https://`，拒绝空 host、localhost、显式回环 IP、私网 IP、链路本地地址；不做 DNS 解析和重绑定防护。

新上游模式限制：

- 只支持 JSON 公网 URL 素材。
- `model=seedance-asset` 返回 400，提示改用视频模型并直接传真人图片 URL。
- 因默认 provider 为 `jimeng`，默认配置下资产库模型不可用；老资产库只在显式 `VIDEO_UPSTREAM_PROVIDER=legacy` 时启用。
- `asset://...` 返回 400，提示新上游不支持旧资产引用。
- multipart、本地文件、base64、data URL 不在 MVP 范围。

创建响应归一化：

```json
{
  "id": "8e8c4f3a2d6b4c9f",
  "task_id": "8e8c4f3a2d6b4c9f",
  "object": "video",
  "model": "jimeng-video-seedance-2.0-vip",
  "status": "queued",
  "progress": 0,
  "created_at": 1781952000,
  "metadata": {
    "jimeng": {
      "status": "pending"
    }
  }
}
```

查询响应归一化：

- `jimeng` 模式下查询任务 ID 是字符串，不走老渠道数字 ID 解析；仅拒绝空值和包含 `/` 的路径。
- 新上游 `pending` -> `queued`。
- 新上游 `processing` -> `in_progress`。
- 新上游 `completed` -> `completed`。
- 新上游 `failed` -> `failed`。
- 未知状态保守映射为 `in_progress`。
- `result.url` 同时映射为顶层 `url` 和 `video_url`。
- `result_expired=true` 保持 `status=completed`，不填 `url` / `video_url`，在 `metadata.jimeng.result_expired=true` 和 `error.message` 中提示结果已过期。
- `completed` 但没有 `result.url` 且没有 `result_expired=true` 时，仍保持 `completed`，不填 `url` / `video_url`，在 `metadata.jimeng.missing_result_url=true` 和 `error.message` 中提示缺少结果 URL；不能映射为 `failed`，避免下游退款语义和上游扣费不一致。

错误归一化：

- 创建接口失败表示任务没有成功提交，返回 HTTP 错误，不生成伪任务。
- 查询接口连不上新上游、上游非 2xx、响应 JSON 解析失败，返回 HTTP 错误，由 NewAPI 轮询侧按现有失败/重试策略处理。
- 查询接口返回任务对象且 `status=failed`，返回 HTTP 200 的任务对象，`status=failed`、`progress=100`，并填充 `error.message`。
- 上游错误字段可能是字符串或对象；代理只向下游暴露摘要，不泄漏 token。
- 上游 429 直接透传为限流错误，不做自动重试。
- 未知任务状态不直接失败，映射为 `in_progress` 并保留 `metadata.jimeng.raw_status`。

兼容与回滚：

- 默认 `jimeng`，因为当前老渠道不可用；老渠道恢复时可显式设置 `VIDEO_UPSTREAM_PROVIDER=legacy` 切回。
- 未知 `VIDEO_UPSTREAM_PROVIDER` 回退到 `jimeng`，并记录 warning。
- 切老上游需设置 `VIDEO_UPSTREAM_PROVIDER=legacy`。
- 回滚时改回 `legacy` 或移除 `VIDEO_UPSTREAM_PROVIDER`。
- 新上游文档建议 3-5 秒轮询；代理保持无状态，不主动节流。
- 项目完成后新增独立下游 NewAPI 调用文档，面向新渠道说明 `files` URL 素材、`reference_mode`、首帧/尾帧/首尾帧、任务查询、结果过期和错误处理。该文档不替代老渠道资产库文档，也不对外推荐兼容字段。
