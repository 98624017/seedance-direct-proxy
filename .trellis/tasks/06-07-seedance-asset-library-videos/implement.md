# Seedance 资产库异步任务实施计划

## Checklist

1. `seedance-direct-proxy`：新增资产请求类型和解析逻辑。
   - 支持 `model=seedance-asset`。
   - 从 `prompt` 读取资产名。
   - 从 `input_reference`、`image`、`images[0]`、`files[0]` 读取图片 URL。
   - 图片 URL 按 `input_reference -> image -> images[0] -> files[0]` 优先级取第一张，多图记录 `ignored_image_count`。
   - 只接受 `http://` / `https://` 公网 URL，拒绝 multipart/base64/data URL。
   - 拒绝空 host、localhost、显式回环 IP、私网 IP、链路本地地址；不做 DNS 解析。

2. `seedance-direct-proxy`：新增上游资产库 client 方法。
   - `CreateAsset` 调 `/resources/user/Resources`。
   - 资产任务 ID 使用 `asset_req_<unix>_<12位小写hex>`。
   - `QueryAsset` 调 `/resources/user/ResourcesList` 并按短追踪后缀 `__ar_<random>` 查找，根据任务年龄默认扫描 10/20/50 页。
   - 资源展示名按最后一次出现的 `__ar_` 剥离追踪后缀。
   - 上游 `Name` 最大 50 个 Unicode 字符；下游资产显示名最大 33 个 Unicode 字符，超长在代理层返回 400。
   - 状态归一化采用保守失败判定：明确失败才失败，无 `AssetId` 默认处理中。
   - 定义资产上游响应结构。

3. `seedance-direct-proxy`：在 HTTP 层复用 `/v1/videos`。
   - POST 资产模型走资产创建分支。
   - GET `asset_req_` 任务 ID 走资产查询分支。
   - 普通视频任务路径保持原样。

4. `seedance-direct-proxy`：补测试。
   - 资产 POST 调用正确上游路径、token 和请求体。
   - 资产 GET 从资源列表返回 `AssetId`。
   - 多图输入只取第一张并记录 `ignored_image_count`。
   - URL 安全校验拒绝 localhost/私网/回环/链路本地地址。
   - 普通视频测试不回归。

5. `new-api`：检查是否需要改动。
   - Sora/OpenAI Video adaptor 需要为 `seedance-asset` 增加资产分支，校验只要求 `model`、资源名称/资产显示名和公网图片 URL。
   - `EstimateBilling` 对资产模型返回空倍率，不叠加 `seconds`、`size` 和参考视频倍率。
   - 若现有 Sora/OpenAI Video adaptor 能完整透传资产响应 metadata，响应转换可尽量少改。
   - 若查询响应丢失 `AssetId`，调整转换逻辑保留 metadata 或增加顶层字段。
   - 确认资产模型按次计费，不叠加视频时长、分辨率和参考视频倍率。
   - 不修改 NewAPI 默认模型/价格表。

6. `new-api`：如需改动则补测试。
   - `/v1/videos` 资产模型能进入 task relay。
   - `GET /v1/videos/:task_id` 能返回包含 `AssetId` 的任务数据。

7. 文档更新。
   - 更新 `README.md` 或新增资产任务示例。
   - 说明资产列表能力先通过现有任务记录/任务日志过滤实现，不提供专用列表端点。

8. `seedance-direct-proxy`：新增无状态资产删除转发。
   - 新增 `POST /api/task/token/asset/delete`，和 NewAPI 上游动作接口保持同名。
   - 从 `Authorization: Bearer ...` 提取上游 token，复用 `baseUrl|token` 兼容逻辑。
   - 请求体只支持 `{"task_id":"asset_req_xxx"}`。
   - 严格校验 `task_id` 格式。
   - 先调 `/resources/user/ResourcesList` 按任务 ID 查真实资源 ID。
   - 调上游资产库 `DELETE /resources/user/Resources`，请求体 `{"Id": resource_id}`。
   - 使用 `ASSET_UPSTREAM_BASE_URL`，不能误走视频生成 host。
   - 失败时透传上游错误信息。

9. `new-api`：新增低复杂度资产删除 API。
   - 注册 `POST /api/task/token/asset/delete`。
   - 使用 `TokenAuthReadOnly()`，不走 `/v1/videos` relay router，不触发计费。
   - 请求体 MVP 只支持 `{"task_id":"asset_req_xxx"}`。
   - 按 `user_id + token_id + task_id` 查任务，防止删除同用户其他 key 或其他用户资产。
   - 校验任务是 `seedance-asset`、`status=SUCCESS`、未 deleted。
   - 从任务 `PrivateData.UpstreamTaskID` 读取上游任务 ID；旧数据回退 `TaskID`。
   - 调渠道上游 `POST /api/task/token/asset/delete`。
   - 删除成功后更新任务 `data.deleted=true` 与 `metadata.seedance.deleted=true`，保留任务历史。

10. 删除功能测试。
   - 代理删除接口先按 `task_id` 查资源列表，再正确调用上游 `/resources/user/Resources`，body 为 `{"Id":123}`。
   - 代理删除接口拒绝不合法资产任务 ID。
   - NewAPI 删除接口只删除当前 token 创建的资产任务。
   - NewAPI 删除接口拒绝非资产任务、未成功任务、已删除任务。
   - NewAPI 删除成功后 `/api/task/token/self` 仍能看到历史任务，但 data 中包含 deleted 标记。

## Validation Commands

在 `seedance-direct-proxy`：

```bash
timeout 120s go test ./...
```

在 `/home/feng/project/new-api`，如有代码改动：

```bash
timeout 120s go test ./relay/... ./controller/... ./service/... ./model/...
```

删除功能最小验证：

```bash
timeout 120s go test ./controller -run 'Asset|TaskToken' -count=1
timeout 120s go test ./...
```

如果改动范围较大，运行：

```bash
timeout 120s go test ./...
```

## Risky Files

- `internal/httpapi/server.go`
- `internal/seedance/client.go`
- `internal/seedance/types.go`
- `internal/openai/types.go`
- `/home/feng/project/new-api/relay/channel/task/sora/adaptor.go`
- `/home/feng/project/new-api/relay/relay_task.go`
- `/home/feng/project/new-api/router/api-router.go`
- `/home/feng/project/new-api/controller/task.go`
- `/home/feng/project/new-api/model/task.go`

## Rollback Points

- 资产分支以虚拟模型名和 `asset_req_` 前缀隔离，回滚时删除资产分支不影响普通视频。
- 不修改数据库结构时，NewAPI 侧回滚风险较低。
- 若需要改 NewAPI 转换逻辑，保持普通视频响应测试覆盖。
- 删除能力以独立 `POST /api/task/token/asset/delete` 隔离，回滚时删除这条路由及相关测试即可，不影响资产创建/查询。

## Jimeng Upstream Addendum Checklist

11. `seedance-direct-proxy`：新增上游选择配置。
   - 新增 `VIDEO_UPSTREAM_PROVIDER`，默认 `jimeng`。
   - 新增 `JIMENG_UPSTREAM_BASE_URL`，默认 `https://api.aizhw.cc`。
   - `legacy` 保持现有老视频上游和资产库行为。
   - `jimeng` 仅让普通视频创建/查询走新上游。
   - 未知 `VIDEO_UPSTREAM_PROVIDER` 回退到 `jimeng`，并记录 warning 日志。

12. `internal/openai`：补新上游请求解析/校验。
   - 复用现有 JSON 解析和参考素材收集逻辑。
   - `model` 必填，不补默认模型。
   - `prompt` 原样透传，不改写 `@图片1` 等素材引用占位符。
   - `ratio` 优先透传；未传 `ratio` 时 `aspect_ratio` 转换为 `ratio`；两者都缺失时不补默认值。
   - `resolution` 透传，缺失时默认 `720p`。
   - `duration` 优先，缺省时使用 `seconds`，两者都缺失时默认 `4`，提交给新上游时统一为 JSON number。
   - 不校验 `duration` 范围或模型时长上限，交给新上游处理。
   - 缺少 `reference_mode` 时，有参考 URL 自动补 `omni`，无参考 URL 自动补 `text_to_video`。
   - 显式传入的 `reference_mode` 透传，覆盖 `omni`、`first_frame`、`last_frame`、`both_frames`、`text_to_video`。
   - `reference_mode` 只支持英文枚举，不做中文别名转换。
   - `omni` 必须 1-12 个 URL，否则返回 400。
   - `text_to_video` 不能带 URL，否则返回 400。
   - `first_frame` / `last_frame` 必须正好 1 个 URL，否则返回 400。
   - `both_frames` 必须正好 2 个 URL，顺序为首帧、尾帧，否则返回 400。
   - `files`、`input_reference`、`file_paths`、`filePaths` 四类字段收集为 URL 列表。
   - 字段收集顺序为 `files -> input_reference -> file_paths -> filePaths`，保留重复 URL 和原始顺序。
   - 新上游模式拒绝 `asset://...`。
   - 新上游模式拒绝 `model=seedance-asset`。
   - 新上游模式只接受公网 `http://` / `https://` URL，不支持本地路径、base64、data URL。
   - URL 校验沿用现有轻量公网 URL 安全校验，拒绝 localhost、回环、私网、链路本地和空 host。

13. `internal/seedance` 或独立 client 文件：新增 Jimeng client 方法。
   - `CreateJimeng` 调 `POST /v1/videos/tasks`。
   - `QueryJimeng` 调 `GET /v1/videos/tasks/{task_id}`。
   - 使用 `Authorization: Bearer <token>`，而不是老渠道 `token` header。
   - 公网 URL 原样写入 `file_paths`，不下载、不转存。
   - 创建响应归一化为 OpenAI Video 风格。
   - 查询响应将 `result.url` 映射为 `url` / `video_url`。
   - `result_expired=true` 保持 completed，并用 metadata/error 表达结果过期。
   - `completed` 但缺少 `result.url` 且未标记过期时仍保持 completed，用 metadata/error 表达缺少结果 URL，不映射为 failed。
   - 创建失败、非 2xx、超时、非法 JSON 不生成伪任务，返回 HTTP 错误。
   - 查询 `status=failed` 返回 HTTP 200 任务对象，并填充 `error.message`。
   - 查询未知状态映射为 `in_progress`，metadata 保留原始状态。
   - 429 透传为限流错误，不做自动重试。

14. `internal/httpapi`：按配置分流。
   - `POST /v1/videos` 在 `legacy` 下保持现有逻辑。
   - `POST /v1/videos` 在 `jimeng` 下拒绝资产模型，普通视频走 Jimeng 创建。
   - 默认配置等同 `jimeng`，因此默认拒绝 `model=seedance-asset`。
   - `GET /v1/videos/{task_id}` 在 `legacy` 下保持现有数字任务/资产任务逻辑。
   - `GET /v1/videos/{task_id}` 在 `jimeng` 下直接按字符串任务 ID 查询 Jimeng，不走数字 ID 校验，但拒绝空值和包含 `/` 的路径。
   - `POST /api/task/token/asset/delete` 保持现有老资产库删除转发。

15. 测试覆盖。
   - 默认配置调用新上游 `/v1/videos/tasks`。
   - 缺少 `model` 返回 400。
   - `VIDEO_UPSTREAM_PROVIDER=legacy` 时仍调用老上游 `/seedanceapi/common/File/All`。
   - `VIDEO_UPSTREAM_PROVIDER=jimeng` 时创建请求调用 `/v1/videos/tasks`。
   - 未知 `VIDEO_UPSTREAM_PROVIDER` 回退到 Jimeng 路径。
   - `aspect_ratio`、`files`、`input_reference`、`file_paths`、`filePaths` 被正确转成 `ratio`、`file_paths`。
   - 缺少 `ratio` / `aspect_ratio` 时不补默认比例。
   - 缺少 `resolution` 时默认提交 `720p`。
   - 字符串或数字形式的 `duration` / `seconds` 都转换成 JSON number 类型 `duration`。
   - 缺少 `duration` / `seconds` 时默认提交 `duration=4`。
   - `duration` 超范围不在代理层提前拒绝，上游错误被归一化返回。
   - 多素材字段混用时按固定顺序收集，保留重复 URL 和原始顺序。
   - URL 安全校验覆盖 localhost、回环 IP、私网 IP、链路本地地址、空 host、非 HTTP(S) scheme。
   - 缺少 `reference_mode` 时按是否存在 URL 自动补默认值。
   - 显式 `reference_mode` 能透传到新上游。
   - 中文 `reference_mode` 不被转换；下游文档只写英文枚举。
   - `omni` 少于 1 个或多于 12 个 URL 都返回 400。
   - `text_to_video` 携带 URL 返回 400。
   - `first_frame` / `last_frame` 少于或多于 1 个 URL 都返回 400。
   - `both_frames` 少于或多于 2 个 URL 都返回 400。
   - prompt 中 `@图片1` 等文本不被自动改写。
   - Jimeng 创建响应返回 `id/task_id/object/status/progress/created_at/metadata`。
   - Jimeng 查询 completed 时填充 `url` / `video_url`。
   - Jimeng 查询 completed 但缺少结果 URL 且未标记过期时保持 completed，metadata/error 表示缺少结果 URL。
   - Jimeng 支持非数字字符串任务 ID 查询。
   - Jimeng 查询 failed 时填充 `error.message`。
   - Jimeng 查询 `result_expired=true` 时保持 completed，URL 为空，metadata/error 表示过期。
   - Jimeng 创建非 2xx、429、超时、非法 JSON 时返回明确 HTTP 错误。
   - Jimeng 查询未知状态时返回 `in_progress` 并保留原始状态。
   - Jimeng 模式下 `asset://...` 返回 400。
   - Jimeng 模式下 `model=seedance-asset` 返回 400。
   - 默认配置下 `model=seedance-asset` 返回 400；`legacy` 模式下老资产测试继续通过。
   - 老资产创建、查询、删除测试继续通过。

16. 文档更新。
   - `README.md` 增加 `VIDEO_UPSTREAM_PROVIDER` / `JIMENG_UPSTREAM_BASE_URL`。
   - 说明新上游模式只支持 URL 原样转发为 `file_paths`。
   - 说明新上游模式不支持 `seedance-asset` 和 `asset://...`。
   - 新增独立下游 NewAPI 调用文档，覆盖新渠道模型、`files` URL 素材、`reference_mode`、首帧/尾帧/首尾帧示例、状态查询、结果过期和错误处理。
   - 下游文档只表达 `files`，不推荐 `input_reference`、`file_paths`、`filePaths`。

## Before `task.py start`

- 用户已确认最终资产模型名为 `seedance-asset`。
- 用户已确认 `AssetId` 同时返回顶层 `asset_id` 和 `metadata.seedance.asset_id`。
- 用户已确认新老上游采用整实例配置切换，默认 `jimeng`，老上游值 `legacy`。
- 用户已确认 `VIDEO_UPSTREAM_PROVIDER` 配错时回退到 `jimeng`。
- 用户已确认默认 `jimeng` 下 `model=seedance-asset` 被拒绝。
- 用户已确认新上游模式下拒绝 `asset://...` 和 `model=seedance-asset`。
- 用户已确认新上游只支持 URL，且公网 URL 原样转发为 `file_paths`。
- 用户已确认新上游缺少 `reference_mode` 时由代理自动补默认值。
- 用户已确认新渠道 `reference_mode` 能力需要暴露给下游，并在项目完成后单独写调用文档。
- 用户已确认 `reference_mode` 只支持英文枚举，不兼容中文别名。
- 用户已确认 `omni` 必须 1-12 个 URL。
- 用户已确认 `text_to_video` 不能带 URL。
- 用户已确认 `first_frame` / `last_frame` 必须正好 1 个 URL。
- 用户已确认 `both_frames` 必须正好 2 个 URL，顺序为首帧、尾帧。
- 用户已确认 `jimeng` 模式下任务 ID 使用字符串，查询跳过老渠道数字 ID 校验。
- 用户已确认 prompt 原样透传，不自动改写素材引用占位符。
- 用户已确认素材字段按 `files -> input_reference -> file_paths -> filePaths` 收集，保留重复 URL 和原始顺序。
- 用户已确认新上游 URL 校验沿用现有轻量公网 URL 安全校验。
- 用户已确认 `duration` 提交给新上游时统一为 JSON number。
- 用户已确认 `duration` 范围和模型时长上限交给新上游校验。
- 用户已确认 `duration` 缺失时默认 `4`。
- 用户已确认 `ratio` / `aspect_ratio` 缺失时不补默认值。
- 用户已确认 `resolution` 缺失时默认 `720p`。
- 用户已确认 `model` 必填，不补默认模型。
- 用户已确认 completed 但缺少结果 URL 时仍保持 completed，不映射为 failed。
- 用户批准从规划进入实现。

## Real Upstream Validation

2026-06-26 使用临时本地链路验证：

```text
NewAPI :39101 -> seedance-direct-proxy :39100 -> https://api.aizhw.cc
```

验证配置：

- NewAPI 渠道类型：`OpenAI`，渠道名使用 `JM系列 Seedance via local Go proxy`。
- Go 代理：`VIDEO_UPSTREAM_PROVIDER=jimeng`。
- 模型：`jimeng-video-seedance-2.0-vip`。
- 请求 1：`duration=4`、`resolution=720p`、`reference_mode=omni`，`files` 包含 1 个公开图片 URL 和 1 个公开 mp4 URL。
- 请求 2：只传 1 个公开图片 URL，不传参考视频。
- 请求 3：`files` 传 2 个不同公开猫图 URL，中文提示词 `两只猫一起在公园里玩耍`，`duration=4`、`resolution=720p`、`reference_mode=omni`。
- 请求 4：`files` 传 2 个公开猫图 URL、用户提供的 PNG 角色图 URL、1 个公开 mp4 URL，中文提示词要求两只猫在参考视频场景里和角色互动。

结果：

- 请求 1 创建任务成功，NewAPI 返回公开 `task_xxx`，状态 `queued`。
- NewAPI 后台轮询成功经由本 Go 代理查询 Jimeng，上游状态先进入 `in_progress`。
- 请求 1 最终上游返回失败：`即梦账号积分或权益不足，请联系管理员处理。`
- 失败响应保持 HTTP 200 的 OpenAI Video 任务对象：`status=failed`、`progress=100`、`error.message` 为上游错误。
- 上游返回 `metadata.jimeng.input_files`，能识别混合素材类型：图片 URL 为 `type=image`，mp4 URL 为 `type=video`。
- `jimeng-video-seedance-2.0-vip` 本次失败响应返回 `metadata.jimeng.cost=200`。
- 请求 2 创建和轮询链路同样打通，上游返回 `metadata.jimeng.input_files[0].type=image`；本次最终失败为上游内部依赖 DNS 错误 `getaddrinfo EAI_AGAIN commerce-api-sg.capcut.com`。
- 请求 3 创建任务成功，NewAPI 返回公开任务 ID `task_h8WaNw6h5EkEmCgxptSUtXAd4cVZQgDA`，轮询约 5 分钟后完成，`status=completed`、`progress=100`，`url` / `video_url` 返回同一个可访问 mp4。
- 请求 3 的结果文件 HEAD 返回 `200 video/mp4`，大小约 3.6 MB；`metadata.jimeng.input_files` 中两个输入均为 `type=image`，`cost=200`。
- 请求 4 创建任务成功，NewAPI 返回公开任务 ID `task_IXxTIccNVB5qtuAHmFdWxKNnb37JQM64`，上游识别素材顺序为 `image,image,image,video`；最终返回 `status=failed`、`progress=100`，错误为 `生成服务暂时繁忙，请稍后重试。`。
- 使用当前 NewAPI 源码查询时，`id` 和 `task_id` 都保持 NewAPI 公开任务 ID；旧本地二进制曾出现 `task_id` 暴露上游任务 ID，验证时不要使用过期二进制。
