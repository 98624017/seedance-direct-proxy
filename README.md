# Seedance Direct Proxy

Go 直连代理，用 OpenAI Videos 风格接口转发 Seedance 视频任务，直接访问：

```text
http://119.45.252.34:8618
```

默认转发到 Jimeng 新上游。面向下游文档和产品展示时，该 Seedance/Jimeng 视频模型系列命名为 **JM 系列**。老 Seedance 直连视频上游和资产库能力仍保留，可通过 `VIDEO_UPSTREAM_PROVIDER=legacy` 显式切回。

## API

创建任务：

```http
POST /v1/videos
Authorization: Bearer <Seedance上游token>
Content-Type: application/json
```

查询任务：

```http
GET /v1/videos/{task_id}
Authorization: Bearer <Seedance上游token>
```

创建真人形象资产任务：

```http
POST /v1/videos
Authorization: Bearer <Seedance上游token>
Content-Type: application/json
```

```json
{
  "model": "seedance-asset",
  "prompt": "林春芽",
  "input_reference": "https://example.com/person.png"
}
```

查询仍使用：

```http
GET /v1/videos/{task_id}
Authorization: Bearer <Seedance上游token>
```

成功后响应会同时返回顶层 `asset_id`、`asset_uri` 和 `metadata.seedance.asset_id`、`metadata.seedance.asset_uri`。

注意：默认 `VIDEO_UPSTREAM_PROVIDER=jimeng` 时不支持 `seedance-asset`，也不支持 `asset://...`。新上游直接在视频任务里传公网图片 URL。只有显式设置 `VIDEO_UPSTREAM_PROVIDER=legacy` 时，资产库创建/查询/删除分支才可用。

删除真人形象资产资源：

```http
POST /api/task/token/asset/delete
Authorization: Bearer <Seedance上游token>
Content-Type: application/json
```

```json
{
  "task_id": "asset_req_1780830000_abcdef123456"
}
```

这个接口用于兼容 NewAPI 的上游动作接口。代理按 `task_id` 查询资产库资源，再调用资产库 `DELETE /resources/user/Resources` 删除真实资源。下游用户权限判断应由 NewAPI 完成，不要把这个代理接口直接暴露给最终用户。

健康检查：

```http
GET /healthz
```

## 创建请求

默认 Jimeng 新上游只支持 JSON 公网 URL 素材。下游推荐统一使用 `files`：

```json
{
  "model": "jimeng-video-seedance-2.0-vip",
  "prompt": "@1 图片中的人物开始跳舞",
  "duration": 4,
  "resolution": "720p",
  "reference_mode": "omni",
  "files": [
    "https://example.com/person.jpg"
  ]
}
```

代理会把 `files` 原样转为新上游 `file_paths`，不会下载、转存或 multipart 上传。新上游模式兼容 `input_reference`、`file_paths`、`filePaths` 作为输入字段，但对下游文档只推荐 `files`。

`reference_mode` 只支持英文枚举：

- `text_to_video`：0 个 URL。
- `omni`：1-12 个 URL。
- `first_frame`：正好 1 个 URL。
- `last_frame`：正好 1 个 URL。
- `both_frames`：正好 2 个 URL，顺序为首帧、尾帧，允许两个 URL 相同。

缺少 `reference_mode` 时，有 URL 默认 `omni`，无 URL 默认 `text_to_video`。`duration` 缺失默认 `4`，并以 JSON number 发给新上游；`resolution` 缺失默认 `720p`；`ratio/aspect_ratio` 缺失时不补默认比例。

### JM 系列真实验证记录

2026-06-26 使用 NewAPI -> 本 Go 代理 -> Jimeng 上游真实链路验证：

- 模型：`jimeng-video-seedance-2.0-vip`。
- 请求 1：`duration=4`、`resolution=720p`、`reference_mode=omni`，`files` 同时包含 1 个公开图片 URL 和 1 个公开 mp4 URL。
- 结果 1：创建任务成功返回 `queued`，后台轮询进入 `in_progress`，最终因上游账号积分或权益不足返回 `failed`。
- 请求 2：只传 1 个公开图片 URL，不传参考视频。
- 结果 2：创建任务成功返回 `queued`，后台轮询进入 `in_progress`，最终因上游内部依赖 DNS 解析失败返回 `failed`，错误为 `getaddrinfo EAI_AGAIN commerce-api-sg.capcut.com`。
- 请求 3：`files` 传 2 个不同公开猫图 URL，中文提示词 `两只猫一起在公园里玩耍`，`duration=4`、`resolution=720p`、`reference_mode=omni`。
- 结果 3：创建任务成功返回 `queued`，后台轮询进入 `in_progress` 后完成，最终 `status=completed`、`progress=100`，`url` / `video_url` 返回可访问 mp4；结果文件 HEAD 为 `200 video/mp4`。
- 请求 4：`files` 传 2 个公开猫图 URL、1 个 PNG 角色图 URL、1 个公开 mp4 URL，中文提示词要求两只猫在参考视频场景里和角色互动。
- 结果 4：创建任务成功返回 `queued`，后台轮询进入 `in_progress`；上游 `metadata.jimeng.input_files` 按顺序识别为 `image,image,image,video`，最终因上游生成服务繁忙返回 `failed`，错误为 `生成服务暂时繁忙，请稍后重试。`。
- 上游查询响应会在 `metadata.jimeng.input_files` 中返回素材识别结果，图片 URL 标记为 `type=image`，视频 URL 标记为 `type=video`。
- 本次 `jimeng-video-seedance-2.0-vip` 失败响应里上游返回 `metadata.jimeng.cost=200`；该值仅表示上游返回的任务消耗信息，代理不自行计算。
- 使用当前 NewAPI 源码查询时，`id` 和 `task_id` 都保持 NewAPI 公开任务 ID，不暴露上游真实任务 ID。

### Legacy 创建请求

文档和示例主推 `files`：

```json
{
  "model": "doubao-seedance-2-0-260128-2",
  "prompt": "@图片1 和@图片2 两个角色在@图片3 场景对打",
  "aspect_ratio": "16:9",
  "duration": "4",
  "resolution": "720p",
  "generate_audio": true,
  "watermark": false,
  "files": [
    "https://example.com/ref-1.jpg",
    "https://example.com/ref-2.jpg",
    "asset://asset-xxxx"
  ]
}
```

兼容参考素材字段：

```text
images, image, input_reference, input_video, video_url, reference_video, audio, audios
```

这些字段最终都会进入上游 multipart `files`。字段值可以是字符串、字符串数组，或常见 OpenAI 风格对象（例如 `{"url":"..."}`、`{"image_url":{"url":"..."}}`）。公网 `http://` / `https://` URL 会由代理下载后作为文件 part 上传；上游资源库地址 `asset://资产id` 会按原值作为文本字段透传。

## 模型

本代理不做模型校验，默认把 `model` 透传给 Seedance 上游。

以下兼容模型会在本层映射为 Seedance 原始模型，并覆盖 `resolution`：

| 传入模型 | 上游模型 | `resolution` |
|---|---|---|
| `doubao-seedance-2-0-fast-260128-480p` | `doubao-seedance-2-0-fast-260128` | `"480p"` |
| `doubao-seedance-2-0-260128-480p` | `doubao-seedance-2-0-260128` | `"480p"` |
| `doubao-seedance-2-0-fast-260128-720p` | `doubao-seedance-2-0-fast-260128` | `"720p"` |
| `doubao-seedance-2-0-260128-720p` | `doubao-seedance-2-0-260128` | `"720p"` |

Seedance 原始模型包括：

```text
doubao-seedance-2-0-fast-260128
doubao-seedance-2-0-260128
doubao-seedance-2-0-260128-1
doubao-seedance-2-0-260128-2
doubao-seedance-2-0-260128-3
```

## 默认值

| 字段 | 默认值 |
|---|---|
| `duration` | `"4"` |
| `aspect_ratio` | `"16:9"` |
| `generate_audio` | `true` |
| `watermark` | `false` |
| `resolution` | `"480p"` |

兼容字段：

- `seconds`：未传 `duration` 时作为 `duration`
- `size=720x1280` / `1024x1792`：转为 `aspect_ratio="9:16"`
- `size=1280x720` / `1792x1024`：转为 `aspect_ratio="16:9"`

## 配置

| 环境变量 | 默认值 |
|---|---|
| `PORT` | `3000` |
| `VIDEO_UPSTREAM_PROVIDER` | `jimeng` |
| `JIMENG_UPSTREAM_BASE_URL` | `https://api.aizhw.cc` |
| `UPSTREAM_BASE_URL` | `http://119.45.252.34:8618` |
| `ASSET_UPSTREAM_BASE_URL` | `http://119.45.42.208:8620` |
| `MAX_REFERENCE_FILES` | `12` |
| `MAX_SINGLE_MEDIA_BYTES` | `52428800` |
| `MAX_TOTAL_MEDIA_BYTES` | `209715200` |
| `MEDIA_PREFETCH_CONCURRENCY` | `6` |
| `MEDIA_FETCH_TIMEOUT_SECONDS` | `75` |
| `UPSTREAM_CREATE_TIMEOUT_SECONDS` | `180` |
| `UPSTREAM_QUERY_TIMEOUT_SECONDS` | `30` |
| `ASSET_LIST_BASE_PAGES` | `10` |
| `ASSET_LIST_MEDIUM_PAGES` | `20` |
| `ASSET_LIST_MAX_PAGES` | `50` |

`VIDEO_UPSTREAM_PROVIDER` 可选值：

- `jimeng`：默认值，普通视频创建/查询走 Jimeng 新上游。
- `legacy`：老渠道恢复后显式切回，普通视频走 `/seedanceapi/common/File/All`，资产库能力走 `/resources/user/*`。

如果配置为未知值，代理会回退到 `jimeng` 并记录 warning 日志。

## 运行

裸二进制：

```bash
go run ./cmd/seedance-direct-proxy
```

构建：

```bash
go build -o seedance-direct-proxy ./cmd/seedance-direct-proxy
PORT=3000 ./seedance-direct-proxy
```

Docker：

```bash
docker build -t seedance-direct-proxy .
docker run --rm -p 3000:3000 seedance-direct-proxy
```

GitHub Container Registry 托管镜像：

```bash
docker run --rm -p 3000:3000 ghcr.io/98624017/seedance-direct-proxy:latest
```

镜像由仓库根目录的 GitHub Actions workflow 构建：

```text
.github/workflows/ghcr.yml
```

## NewAPI 配置

渠道类型：

```text
OpenAI
```

Base URL：

```text
http://<你的服务器IP或域名>:3000
```

Key：

```text
<Jimeng 或 Seedance 上游 token>
```

默认 Jimeng 模式下，请在 NewAPI 中配置 JM 系列视频模型，真人图直接作为公网 URL 放入 `files`。`seedance-asset` 和 `asset://...` 只适用于 `VIDEO_UPSTREAM_PROVIDER=legacy`。

Legacy 资产库任务使用同一个 `/v1/videos` 入口。NewAPI 里需要手动配置虚拟模型：

```text
seedance-asset
```

按次价格、渠道模型权限和分组由 NewAPI 后台配置；本项目不要求写入 NewAPI 默认模型/价格表。

如果你的 NewAPI 仍使用 `baseUrl|token` 格式，本代理也兼容：

```text
http://119.45.252.34:8618|<Seedance上游token>
```

代理只会取 `|` 右侧作为上游 token。

## curl 示例

```bash
curl -X POST 'http://127.0.0.1:3000/v1/videos' \
  -H 'Authorization: Bearer <Jimeng上游token>' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "jimeng-video-seedance-2.0-vip",
    "prompt": "@1 图片中的人物开始跳舞",
    "ratio": "4:3",
    "duration": 4,
    "resolution": "720p",
    "reference_mode": "omni",
    "files": [
      "https://example.com/ref-1.jpg"
    ]
  }'
```

查询：

```bash
curl 'http://127.0.0.1:3000/v1/videos/8e8c4f3a2d6b4c9f' \
  -H 'Authorization: Bearer <Jimeng上游token>'
```

Legacy 创建资产：

```bash
curl -X POST 'http://127.0.0.1:3000/v1/videos' \
  -H 'Authorization: Bearer <Seedance上游token>' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "seedance-asset",
    "prompt": "林春芽",
    "input_reference": "https://example.com/person.png"
  }'
```

## 转发说明

创建视频任务时，代理会按输入顺序构造 multipart 表单：公网 `http://` / `https://` URL 会先下载并作为文件 part 上传，`asset://...` 资源库地址会作为文本 `files` 字段透传。

创建真人形象资产任务时，代理仍调用资产库 `/resources/user/Resources`，把图片 URL 写入 JSON 字段 `OssPath`。资产查询成功后，`asset_id` 是上游原始资产 ID，`asset_uri` 是可直接放进视频生成 `files` 的 `asset://...` 地址。

资产显示名来自请求里的 `prompt`，最多 33 个 Unicode 字符。代理会在上游资源名后追加短追踪后缀 `__ar_<12位hex>`，确保完整上游 `Name` 不超过资产库 50 字符限制。

删除真人形象资产资源时，使用 `POST /api/task/token/asset/delete` 并传 `task_id`。代理会先按任务 ID 查询资产库资源，再调用资产库 `/resources/user/Resources` 删除真实资源。

注意：视频生成接口和资产库接口不在同一个上游 host。视频生成使用 `UPSTREAM_BASE_URL`，资产创建/查询使用 `ASSET_UPSTREAM_BASE_URL`；如果未配置，资产库默认使用 `http://119.45.42.208:8620`。
