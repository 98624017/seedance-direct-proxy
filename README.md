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
    "https://example.com/ref-3.jpg"
  ]
}
```

兼容参考素材字段：

```text
images, image, input_reference, input_video, video_url, reference_video, audio, audios
```

这些字段最终都会作为上游 multipart `files` 字段转发。

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
| `MAX_REFERENCE_FILES` | `12` |
| `MAX_SINGLE_MEDIA_BYTES` | `52428800` |
| `MAX_TOTAL_MEDIA_BYTES` | `209715200` |
| `MEDIA_PREFETCH_CONCURRENCY` | `6` |
| `MEDIA_FETCH_TIMEOUT_SECONDS` | `75` |
| `UPSTREAM_CREATE_TIMEOUT_SECONDS` | `180` |
| `UPSTREAM_QUERY_TIMEOUT_SECONDS` | `30` |

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
      "https://example.com/ref-2.jpg"
    ]
  }'
```

查询：

```bash
curl 'http://127.0.0.1:3000/v1/videos/18' \
  -H 'Authorization: Bearer <Seedance上游token>'
```

## 性能说明

创建任务使用真正流式 multipart：

- 素材 URL 并发预取。
- multipart part 按输入顺序写入。
- 文件内容从素材 HTTP response body 直接复制到上游 multipart writer。
- 不写磁盘，不先完整下载所有素材。
