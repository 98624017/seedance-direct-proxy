# Seedance NewAPI 唯一调用文档

本文档面向下游客户端和下游 NewAPI 实例，统一说明如何通过已接入 Seedance 渠道的 NewAPI 调用视频生成、真人形象资产创建、资产查询、任务列表筛选和资产删除。

后续请以本文档为唯一接入依据，不再单独维护真人资产 API 文档。

## 1. 基础信息

Base URL 可选：

```text
https://api.light-ai.cloud
https://ay.light-ai.cloud
```


请求鉴权：

```http
Authorization: Bearer <NEWAPI_API_KEY>
```

这里使用的是 NewAPI 分发给调用方的 API Key，不是 Seedance 真实上游 token。

JSON 请求统一使用：

```http
Content-Type: application/json
```

## 2. 能力总览

| 能力 | 方法 | 路径 | 说明 |
|---|---|---|---|
| 创建视频任务 | `POST` | `/v1/videos` | 使用 NewAPI 对外视频模型名 |
| 创建真人形象资产 | `POST` | `/v1/videos` | 使用虚拟模型 `seedance-asset` |
| 查询单个视频/资产任务 | `GET` | `/v1/videos/{task_id}` | 视频任务和资产任务共用 |
| 查询当前 API Key 的任务列表 | `GET` | `/api/task/token/self` | 返回当前 key 创建的所有异步任务 |
| 删除真人形象资产 | `POST` | `/api/task/token/asset/delete` | 只删除当前 key 创建的成功资产 |

核心约定：

- 普通视频生成和真人资产创建都走 `/v1/videos` 异步任务机制。
- `model="seedance-asset"` 表示创建真人形象资产，不是视频生成模型。
- 资产创建成功后，查询任务会得到 `asset_id` 和 `asset_uri`。
- 视频生成时把 `asset_uri` 放入 `files`，例如 `asset://asset-xxxx`。
- `/api/task/token/self` 是任务历史列表，已删除资产也可能返回；展示可用资产时必须过滤 `deleted=true`。

## 3. 创建视频任务

### 3.1 请求

```http
POST /v1/videos HTTP/1.1
Host: api.light-ai.cloud
Authorization: Bearer <NEWAPI_API_KEY>
Content-Type: application/json
```

完整 URL：

```text
https://api.light-ai.cloud/v1/videos
```

### 3.2 请求体字段

| 字段 | 类型 | 必填 | 说明 |
|---|---:|---:|---|
| `model` | string | 是 | NewAPI 对外视频模型名；NewAPI 后台已映射到 Seedance 上游模型 |
| `prompt` | string | 是 | 视频生成提示词 |
| `duration` | string | 否 | 输出时长，单位秒，范围 `"4"` 到 `"15"`，默认 `"4"` |
| `seconds` | string | 否 | OpenAI Videos 风格时长字段；未传 `duration` 时作为 `duration` 使用 |
| `aspect_ratio` | string | 否 | 输出比例，默认 `"16:9"` |
| `files` | string 或 string[] | 否 | 参考素材，支持公网 URL、`asset://资产ID` |
| `generate_audio` | boolean/string | 否 | 是否生成音频，默认 `true` |
| `watermark` | boolean/string | 否 | 是否加水印，默认 `false` |
| `resolution` | string | 否 | 清晰度，默认 `"480p"` |

可选值：

| 字段 | 可选值 / 范围 | 默认值 |
|---|---|---|
| `duration` | `"4"` 到 `"15"` | `"4"` |
| `seconds` | `"4"` 到 `"15"`；未传 `duration` 时生效 | 无 |
| `aspect_ratio` | `"21:9"`、`"16:9"`、`"4:3"`、`"1:1"`、`"3:4"`、`"9:16"` | `"16:9"` |
| `generate_audio` | `true`、`false`、`"true"`、`"false"` | `true` |
| `watermark` | `true`、`false`、`"true"`、`"false"` | `false` |
| `resolution` | `"480p"`、`"720p"`、`"1080p"` | `"480p"` |

### 3.3 参考素材

`files` 是推荐字段，可以同时传入：

- 公网图片 URL。
- 公网视频 URL。
- 公网音频 URL。
- 资源库地址，例如 `asset://asset-xxxx`。



注意：

- 公网 URL 必须能被代理服务访问。
- 不要传本地文件路径、内网 URL、需要登录态的 URL、base64 或 data URL。
- 真人资产创建返回的是 `asset_id: "asset-xxxx"` 和 `asset_uri: "asset://asset-xxxx"`；视频生成时建议直接传 `asset_uri`。
- 普通视频生成不需要先把公网 URL 转成资产，公网 URL 可以直接放在 `files` 里。
- 兼容参考素材字段也接受常见 OpenAI 风格对象，例如 `{"url":"..."}`、`{"image_url":{"url":"..."}}`。

### 3.4 支持模型

推荐模型：

```text
veofast
veo
```

对外模型名说明：

| 对外模型名 | NewAPI 映射到的上游模型 | 上线状态 | 说明 |
|---|---|---|---|
| `veofast` | `doubao-seedance-2-0-260128-2` | 已上线 | 快速版；最多 9 图、3 音视频参考；单个音视频大于 4 秒，总秒数小于 15 秒；预计 3-8 分钟 |
| `veo` | `doubao-seedance-2-0-260128-3` | 已上线 | 满血版；最多 9 图、3 音视频参考；单个音视频大于 4 秒，总秒数小于 15 秒；预计 8-10 分钟 |
| `veofastcn-480p` | `doubao-seedance-2-0-fast-260128` | 视实例配置而定 | 快速版 480p；最多 9 图、3 视频、3 音频；预计 3-5 分钟 |
| `veofastcn-720p` | `doubao-seedance-2-0-fast-260128` | 视实例配置而定 | 快速版 720p；最多 9 图、3 视频、3 音频；预计 3-5 分钟 |
| `veocn-480p` | `doubao-seedance-2-0-260128` | 视实例配置而定 | 满血版 480p；最多 9 图、3 视频、3 音频；预计 5-10 分钟 |
| `veocn-720p` | `doubao-seedance-2-0-260128` | 视实例配置而定 | 满血版 720p；最多 9 图、3 视频、3 音频；预计 5-10 分钟 |

是否可调用以当前 NewAPI 实例后台配置为准。
调用方请求时只需要填写左侧“对外模型名”，不要直接填写 `doubao-*` 上游原始模型名。

### 3.5 请求示例

文生视频：

```bash
curl -X POST 'https://api.light-ai.cloud/v1/videos' \
  -H 'Authorization: Bearer <NEWAPI_API_KEY>' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "veofast",
    "prompt": "A cinematic shot of a banana on a clean table, slow camera movement, soft studio light",
    "duration": "4",
    "aspect_ratio": "16:9",
    "generate_audio": true,
    "resolution": "480p"
  }'
```

公网参考图生成视频：

```bash
curl -X POST 'https://api.light-ai.cloud/v1/videos' \
  -H 'Authorization: Bearer <NEWAPI_API_KEY>' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "veofast",
    "prompt": "Make the person in the reference image speak on a theatre stage",
    "duration": "4",
    "aspect_ratio": "16:9",
    "files": [
      "https://cdn.example.com/person.png",
      "https://cdn.example.com/stage.jpg"
    ],
    "generate_audio": false,
    "watermark": false,
    "resolution": "480p"
  }'
```

使用真人资产生成视频：

```bash
curl -X POST 'https://api.light-ai.cloud/v1/videos' \
  -H 'Authorization: Bearer <NEWAPI_API_KEY>' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "veocn-480p",
    "prompt": "让这个真人形象站在剧场舞台中央讲话，说同志们辛苦了",
    "duration": "4",
    "aspect_ratio": "16:9",
    "files": [
      "asset://asset-xxxx",
      "https://cdn.example.com/stage.jpg"
    ],
    "generate_audio": false,
    "watermark": false,
    "resolution": "480p"
  }'
```

## 4. 创建真人形象资产

真人形象资产用于把一张真人图片加入上游资源库，后续视频生成时通过 `asset://...` 引用。

### 4.1 请求

资产创建仍使用：

```http
POST /v1/videos
Authorization: Bearer <NEWAPI_API_KEY>
Content-Type: application/json
```

请求体：

```json
{
  "model": "seedance-asset",
  "prompt": "林春芽",
  "input_reference": "https://cdn.example.com/person.png"
}
```

字段说明：

| 字段 | 必填 | 说明 |
|---|---:|---|
| `model` | 是 | 固定填 `seedance-asset` |
| `prompt` | 是 | 资源名称/真人形象名称 |
| `input_reference` | 是 | 可公网访问的真人图片 URL |

图片 URL 也可以放在以下字段中，优先级固定为：

```text
input_reference -> image -> images[0] -> files[0]
```

一次请求只创建一个资产。如果传入多张图片，只会使用第一张，其余数量会记录在 `metadata.seedance.ignored_image_count`。

图片 URL 限制：

- 必须是 `http://` 或 `https://`。
- host 不能为空。
- 不能是 `localhost`、回环 IP、私网 IP 或链路本地地址。
- 不支持 multipart、本地路径、base64、data URL。

### 4.2 创建响应

创建成功后返回异步任务。此时通常还没有 `asset_id`，需要继续查询任务。

```json
{
  "id": "asset_req_1780830000_abcdef123456",
  "task_id": "asset_req_1780830000_abcdef123456",
  "object": "video",
  "model": "seedance-asset",
  "status": "queued",
  "progress": 0,
  "created_at": 1780830000,
  "metadata": {
    "seedance": {
      "kind": "asset",
      "name": "林春芽",
      "oss_path": "https://cdn.example.com/person.png",
      "ignored_image_count": 0
    }
  }
}
```

请保存返回的 `id` 或 `task_id`，后续用它查询资产处理结果。

## 5. 查询单个任务

视频任务和资产任务都使用同一个查询接口：

```http
GET /v1/videos/{task_id}
Authorization: Bearer <NEWAPI_API_KEY>
```

示例：

```bash
curl 'https://api.light-ai.cloud/v1/videos/<task_id>' \
  -H 'Authorization: Bearer <NEWAPI_API_KEY>'
```

查询时必须使用创建响应里的 `id` 或 `task_id` 原值，不要自行改成上游内部 ID。

### 5.1 视频任务响应

排队中：

```json
{
  "id": "422",
  "task_id": "422",
  "object": "video",
  "status": "queued",
  "progress": 0,
  "created_at": 1780830000
}
```

生成中：

```json
{
  "id": "422",
  "task_id": "422",
  "object": "video",
  "status": "in_progress",
  "progress": 50,
  "metadata": {
    "seedance": {
      "Status": 1,
      "StatusText": "处理中"
    }
  }
}
```

已完成：

```json
{
  "id": "422",
  "task_id": "422",
  "object": "video",
  "model": "veofast",
  "status": "completed",
  "progress": 100,
  "url": "https://cdn.example.com/result.mp4",
  "video_url": "https://cdn.example.com/result.mp4"
}
```

完成后建议优先读取 `video_url`，没有时回退到 `url`。视频地址通常是上游临时下载地址，建议生成后尽快保存。

失败：

```json
{
  "id": "422",
  "task_id": "422",
  "object": "video",
  "status": "failed",
  "progress": 100,
  "error": {
    "code": "3",
    "message": "token不足"
  }
}
```

失败时优先展示：

1. `error.message`
2. `metadata.seedance.Message`
3. `metadata.seedance.StatusText`

### 5.2 资产任务响应

处理中：

```json
{
  "id": "asset_req_1780830000_abcdef123456",
  "task_id": "asset_req_1780830000_abcdef123456",
  "object": "video",
  "model": "seedance-asset",
  "status": "in_progress",
  "progress": 50
}
```

处理成功：

```json
{
  "id": "asset_req_1780830000_abcdef123456",
  "task_id": "asset_req_1780830000_abcdef123456",
  "object": "video",
  "model": "seedance-asset",
  "status": "completed",
  "progress": 100,
  "asset_id": "asset-xxxx",
  "asset_uri": "asset://asset-xxxx",
  "metadata": {
    "seedance": {
      "kind": "asset",
      "asset_id": "asset-xxxx",
      "asset_uri": "asset://asset-xxxx",
      "name": "林春芽"
    }
  }
}
```

字段说明：

| 字段 | 说明 |
|---|---|
| `asset_id` | 上游原始资产 ID，例如 `asset-xxxx` |
| `asset_uri` | 可直接用于视频生成 `files` 的资源库地址，例如 `asset://asset-xxxx` |

视频生成时优先使用 `asset_uri`。如果只拿到 `asset_id`，可以自行拼成：

```text
asset://<asset_id>
```

## 6. 任务状态

| `status` | 说明 |
|---|---|
| `queued` | 任务已进入队列 |
| `in_progress` | 任务处理中 |
| `completed` | 任务已完成；视频任务读 `video_url` / `url`，资产任务读 `asset_uri` |
| `failed` | 任务失败，查看 `error.message` 或 `metadata.seedance.Message` |

建议轮询间隔：5-10 秒。

资产任务说明：

- 未找到资源项或暂时没有 `asset_id` 时，会返回 `in_progress`，不代表失败。
- 资产任务长时间没有完成时，按 NewAPI 现有任务超时机制处理。

## 7. 查询当前 API Key 的任务列表

如果需要查看当前 API Key 创建过的所有异步任务，使用：

```http
GET /api/task/token/self
Authorization: Bearer <NEWAPI_API_KEY>
```

示例：

```bash
curl 'https://api.light-ai.cloud/api/task/token/self?p=1&page_size=20' \
  -H 'Authorization: Bearer <NEWAPI_API_KEY>'
```

这个接口按当前请求使用的 API Key 过滤任务，只返回这个 key 创建的新任务；不会返回同一账号下其他 API Key 创建的任务。

常用查询参数：

| 参数 | 说明 |
|---|---|
| `p` | 页码 |
| `page_size` / `size` | 每页数量 |
| `task_id` | 按任务 ID 过滤 |
| `status` | 按任务状态过滤 |
| `action` | 按任务动作过滤 |
| `platform` | 按任务平台过滤 |
| `start_timestamp` | 开始时间戳 |
| `end_timestamp` | 结束时间戳 |

查询所有视频和资产任务：

```bash
curl 'https://api.light-ai.cloud/api/task/token/self?p=1&page_size=20' \
  -H 'Authorization: Bearer <NEWAPI_API_KEY>'
```

只查询成功任务：

```bash
curl 'https://api.light-ai.cloud/api/task/token/self?status=SUCCESS&p=1&page_size=20' \
  -H 'Authorization: Bearer <NEWAPI_API_KEY>'
```

返回结构核心字段通常在 `data.items`：

```json
{
  "success": true,
  "data": {
    "total": 2,
    "items": [
      {
        "task_id": "asset_req_1780830000_abcdef123456",
        "status": "SUCCESS",
        "properties": {
          "origin_model_name": "seedance-asset"
        },
        "data": {
          "model": "seedance-asset",
          "status": "completed",
          "asset_id": "asset-xxxx",
          "asset_uri": "asset://asset-xxxx",
          "metadata": {
            "seedance": {
              "kind": "asset",
              "name": "林春芽",
              "asset_uri": "asset://asset-xxxx",
              "deleted": false
            }
          }
        }
      }
    ]
  }
}
```

### 7.1 筛选可用真人资产

当前列表接口没有单独的 `model=seedance-asset` 查询参数。客户端应先请求成功任务，再在返回的 `items` 中按模型和 metadata 过滤：

```js
const assets = page.data.items.filter((task) => {
  const data = task.data || {};
  const metadata = data.metadata?.seedance || {};

  return task.status === "SUCCESS"
    && (
      data.model === "seedance-asset"
      || task.properties?.origin_model_name === "seedance-asset"
      || metadata.kind === "asset"
    )
    && !data.deleted
    && !metadata.deleted
    && (data.asset_uri || metadata.asset_uri || data.asset_id || metadata.asset_id);
});
```

每个可用真人资产优先读取：

```text
task.data.asset_uri
```

如果顶层没有，再读取：

```text
task.data.metadata.seedance.asset_uri
```

如果只拿到 `asset_id`，需要自行拼成：

```text
asset://<asset_id>
```

### 7.2 已删除资产是否还显示

会保留任务历史，所以 `/api/task/token/self` 仍可能返回已删除资产任务。但删除成功后任务数据里会标记：

```json
{
  "deleted": true,
  "deleted_at": 1780830000
}
```

客户端展示“可用真人资产”时必须排除：

- `task.data.deleted === true`
- `task.data.metadata.seedance.deleted === true`

## 8. 删除真人形象资产

删除资产使用 NewAPI 后台动作接口：

```http
POST /api/task/token/asset/delete
Authorization: Bearer <NEWAPI_API_KEY>
Content-Type: application/json
```

请求体只传资产任务 ID，也就是创建真人资产时返回的 `id` / `task_id`：

```json
{
  "task_id": "asset_req_1780830000_abcdef123456"
}
```

示例：

```bash
curl -X POST 'https://api.light-ai.cloud/api/task/token/asset/delete' \
  -H 'Authorization: Bearer <NEWAPI_API_KEY>' \
  -H 'Content-Type: application/json' \
  -d '{"task_id":"asset_req_1780830000_abcdef123456"}'
```

成功响应：

```json
{
  "success": true,
  "message": "",
  "data": {
    "task_id": "asset_req_1780830000_abcdef123456",
    "deleted": true,
    "deleted_at": 1780830000,
    "resource_id": 123,
    "asset_id": "asset-xxxx",
    "asset_uri": "asset://asset-xxxx"
  }
}
```

删除规则：

- 只能删除当前请求使用的 API Key 创建的资产任务。
- 只能删除已经成功的 `seedance-asset` 资产任务。
- 请求体只支持 `task_id`，不支持直接传 `asset_id`、`asset_uri` 或上游 `resource_id`。
- 删除不会移除任务历史，`/api/task/token/self` 仍可能返回这条任务。
- 删除成功后，任务会被标记 `deleted=true`、`deleted_at=<unix秒>`。
- 删除资产不等于取消任务，也不会触发退款；它只是删除已创建成功的上游资源库资产。

如果本地没有保存资产 `task_id`，可以先查成功任务并筛选可删除资产：

```js
const deletableAssets = page.data.items.filter((task) => {
  const data = task.data || {};
  const metadata = data.metadata?.seedance || {};

  return task.status === "SUCCESS"
    && (
      data.model === "seedance-asset"
      || task.properties?.origin_model_name === "seedance-asset"
      || metadata.kind === "asset"
    )
    && !data.deleted
    && !metadata.deleted
    && task.task_id;
});
```

常见失败结构：

```json
{
  "success": false,
  "message": "task not found"
}
```

常见原因：

| 场景 | 说明 |
|---|---|
| `task not found` | `task_id` 不存在，或不是当前 API Key 创建的任务 |
| `only successful asset tasks can be deleted` | 资产任务还没成功，不能删除 |
| `task is not a seedance asset` | 传入的是普通视频任务，不是真人资产任务 |
| `asset already deleted` | 这个资产已经删除过 |
| `seedance asset delete failed` | 上游删除失败，可能是资产已不存在、渠道 key 失效或上游服务异常 |

## 9. 下游 NewAPI 接本实例作为上游

如果链路是：

```text
客户 API Key -> 下游 NewAPI -> 本层 NewAPI -> seedance-direct-proxy -> 真实 Seedance 上游
```

推荐下游 NewAPI 按本文档把“本层 NewAPI”当作 OpenAI Videos 兼容上游接入：

- 视频生成继续转发 `POST /v1/videos` 和 `GET /v1/videos/{task_id}`。
- 真人资产创建也转发 `POST /v1/videos`，模型填 `seedance-asset`。
- 删除资产转发 `POST /api/task/token/asset/delete`。
- 下游 NewAPI 自己仍应基于客户 API Key 做权限、计费和任务归属隔离。
- 下游 NewAPI 调本层 NewAPI 时使用“本层 NewAPI 分配给下游 NewAPI 的 API Key”。

不要让最下游客户直接传真实 Seedance 上游 token。

## 10. 最小接入清单

1. 配置 API Base URL：推荐 `https://api.light-ai.cloud`，也可使用 `https://ay.light-ai.cloud`
2. 配置请求头：`Authorization: Bearer <NEWAPI_API_KEY>`
3. 创建普通视频：`POST /v1/videos`，使用 NewAPI 对外视频模型名，例如 `veofast`、`veo`、`veofastcn-480p`、`veocn-480p`。
4. 创建真人资产：`POST /v1/videos`，使用 `model="seedance-asset"`。
5. 查询单个任务：`GET /v1/videos/{task_id}`。
6. 查询当前 key 的任务历史：`GET /api/task/token/self`。
7. 筛选可用真人资产：按 `seedance-asset` / `metadata.seedance.kind="asset"` 过滤，并排除 `deleted=true`。
8. 视频生成引用资产：把 `asset_uri` 放到 `files`，例如 `"asset://asset-xxxx"`。
9. 删除资产：`POST /api/task/token/asset/delete`，请求体传 `{"task_id":"asset_req_xxx"}`。
10. 视频完成后从 `video_url` 或 `url` 读取视频地址。

## 11. 调用注意事项

- 默认时长为 4 秒，默认生成音频，默认清晰度为 `480p`。
- 如果同时传入 `duration` 和 `seconds`，以 `duration` 为准。
- 建议客户端显式传入 `duration`、`aspect_ratio`、`resolution`，避免不同客户端默认值不一致。
- `files` 可以混合传入 `asset://...` 和公网 URL。
- NewAPI 返回的 `task_id` 是客户端应保存和查询的任务 ID。
- `metadata.seedance` 主要用于排障，不建议把上游原始字段作为主业务字段强依赖。
- 如果 `status="failed"` 且 `error.message` 是余额、token 或账号类错误，说明请求已到达上游，但上游账号不可用或余额不足。
