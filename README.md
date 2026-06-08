# Seedance Direct Proxy

Go 直连代理，用 OpenAI Videos 风格接口转发 Seedance 视频任务，直接访问：

```text
http://119.45.252.34:8618
```

目标是减少 Cloudflare Worker / Zeabur / Caddy 等中间层带来的创建任务延迟。

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

这个接口用于兼容 NewAPI 的上游动作接口。代理按 `task_id` 查询资产库资源，再调用资产库 `DELETE /resourcesapi/user/Resources` 删除真实资源。下游用户权限判断应由 NewAPI 完成，不要把这个代理接口直接暴露给最终用户。

健康检查：

```http
GET /healthz
```

## 创建请求

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

这些字段最终都会作为上游 multipart `files` 文本字段直接转发，支持公网 URL 和上游资源库地址 `asset://资产id`。代理不再先下载素材再以二进制文件上传。

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
| `UPSTREAM_BASE_URL` | `http://119.45.252.34:8618` |
| `ASSET_UPSTREAM_BASE_URL` | `http://119.45.42.208:8620` |
| `MAX_REFERENCE_FILES` | `12` |
| `UPSTREAM_CREATE_TIMEOUT_SECONDS` | `180` |
| `UPSTREAM_QUERY_TIMEOUT_SECONDS` | `30` |
| `ASSET_LIST_BASE_PAGES` | `10` |
| `ASSET_LIST_MEDIUM_PAGES` | `20` |
| `ASSET_LIST_MAX_PAGES` | `50` |

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
.github/workflows/seedance-direct-proxy-ghcr.yml
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
<Seedance上游token>
```

资产库任务使用同一个 `/v1/videos` 入口。NewAPI 里需要手动配置虚拟模型：

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
  -H 'Authorization: Bearer <Seedance上游token>' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "doubao-seedance-2-0-260128-2",
    "prompt": "@图片1 和@图片2 两个角色在@图片3 场景对打",
    "aspect_ratio": "16:9",
    "duration": "4",
    "resolution": "720p",
    "generate_audio": true,
    "watermark": false,
    "files": [
      "https://example.com/ref-1.jpg",
      "asset://asset-xxxx"
    ]
  }'
```

查询：

```bash
curl 'http://127.0.0.1:3000/v1/videos/18' \
  -H 'Authorization: Bearer <Seedance上游token>'
```

创建资产：

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

创建视频任务时，代理只构造 multipart 表单并把 `files` 的值按原顺序写给上游；素材下载、资源库地址解析和可访问性校验由 Seedance 上游负责。

创建真人形象资产任务时，代理仍调用资产库 `/resources/user/Resources`，把图片 URL 写入 JSON 字段 `OssPath`。资产查询成功后，`asset_id` 是上游原始资产 ID，`asset_uri` 是可直接放进视频生成 `files` 的 `asset://...` 地址。

删除真人形象资产资源时，使用 `POST /api/task/token/asset/delete` 并传 `task_id`。代理会先按任务 ID 查询资产库资源，再调用资产库 `/resourcesapi/user/Resources` 删除真实资源。

注意：视频生成接口和资产库接口不在同一个上游 host。视频生成使用 `UPSTREAM_BASE_URL`，资产创建/查询使用 `ASSET_UPSTREAM_BASE_URL`；如果未配置，资产库默认使用 `http://119.45.42.208:8620`。
