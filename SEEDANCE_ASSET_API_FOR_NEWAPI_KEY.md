# Seedance 真人形象资产 API 使用说明

本文面向只持有 NewAPI API Key 的客户。你不需要 NewAPI 登录账号，只需要用你的 API Key 调用接口。

## 基本信息

Base URL 使用服务商提供的 NewAPI 地址：

```text
https://你的-newapi-域名
```

所有请求都使用 API Key 鉴权：

```http
Authorization: Bearer <你的 NewAPI API Key>
```

资产模型固定为：

```text
seedance-asset
```

## 创建真人形象资产

```http
POST /v1/videos
Content-Type: application/json
Authorization: Bearer <你的 NewAPI API Key>
```

请求体：

```json
{
  "model": "seedance-asset",
  "prompt": "林春芽",
  "input_reference": "https://example.com/person.png"
}
```

字段说明：

| 字段 | 必填 | 说明 |
|---|---:|---|
| `model` | 是 | 固定填 `seedance-asset` |
| `prompt` | 是 | 资源名称/真人形象名称 |
| `input_reference` | 是 | 可公网访问的真人图片 URL |

图片 URL 也可放在以下字段中，优先级为：

```text
input_reference -> image -> images[0] -> files[0]
```

一次请求只创建一个资产；如果传入多张图片，只会使用第一张。

创建成功后会返回异步任务：

```json
{
  "id": "task_xxx",
  "task_id": "task_xxx",
  "object": "video",
  "model": "seedance-asset",
  "status": "queued",
  "progress": 0
}
```

请保存返回的 `id` 或 `task_id`，后续用它查询结果。

## 查询资产处理结果

```http
GET /v1/videos/{task_id}
Authorization: Bearer <你的 NewAPI API Key>
```

示例：

```bash
curl 'https://你的-newapi-域名/v1/videos/task_xxx' \
  -H 'Authorization: Bearer sk-xxxx'
```

处理中：

```json
{
  "id": "task_xxx",
  "task_id": "task_xxx",
  "object": "video",
  "model": "seedance-asset",
  "status": "in_progress",
  "progress": 50
}
```

处理成功：

```json
{
  "id": "task_xxx",
  "task_id": "task_xxx",
  "object": "video",
  "model": "seedance-asset",
  "status": "completed",
  "progress": 100,
  "asset_id": "asset-****",
  "asset_uri": "asset://asset-****",
  "metadata": {
    "seedance": {
      "kind": "asset",
      "asset_id": "asset-****",
      "asset_uri": "asset://asset-****",
      "name": "林春芽"
    }
  }
}
```

返回字段说明：

| 字段 | 说明 |
|---|---|
| `asset_id` | 上游原始资产 ID，例如 `asset-****` |
| `asset_uri` | 可直接用于视频生成 `files` 的资源库地址，例如 `asset://asset-****` |

视频生成时请优先使用 `asset_uri`：

```json
{
  "model": "doubao-seedance-2-0-260128",
  "prompt": "让这个真人形象站在剧场舞台中央讲话",
  "duration": "4",
  "aspect_ratio": "16:9",
  "resolution": "480p",
  "generate_audio": "false",
  "watermark": "false",
  "files": [
    "asset://asset-****",
    "https://example.com/stage.jpg"
  ]
}
```

`files` 可以同时传入资产库地址和公网参考素材 URL。代理会把它们按顺序转发给上游视频生成接口。

视频生成创建成功后会返回普通视频任务：

```json
{
  "id": "422",
  "task_id": "422",
  "object": "video",
  "model": "doubao-seedance-2-0-260128",
  "status": "queued",
  "progress": 0
}
```

继续使用同一个查询接口：

```http
GET /v1/videos/{task_id}
Authorization: Bearer <你的 NewAPI API Key>
```

处理中可能会先返回 `queued` / `待处理`，这只表示上游还在排队，不代表失败：

```json
{
  "id": "422",
  "task_id": "422",
  "object": "video",
  "status": "queued",
  "progress": 0
}
```

生成完成后返回视频地址：

```json
{
  "id": "422",
  "task_id": "422",
  "object": "video",
  "status": "completed",
  "progress": 100,
  "url": "https://example.com/video.mp4",
  "video_url": "https://example.com/video.mp4"
}
```

视频地址通常是上游临时下载地址，建议生成后尽快保存。

## 查询当前 API Key 创建的任务列表

如果你需要查看当前 API Key 创建过的所有异步任务，使用：

```http
GET /api/task/token/self
Authorization: Bearer <你的 NewAPI API Key>
```

示例：

```bash
curl 'https://你的-newapi-域名/api/task/token/self?p=1&page_size=20' \
  -H 'Authorization: Bearer sk-xxxx'
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

查询成功后会返回分页结构，核心字段通常在 `data.items`：

```json
{
  "success": true,
  "data": {
    "total": 2,
    "items": [
      {
        "task_id": "asset_req_xxx",
        "status": "SUCCESS",
        "properties": {
          "origin_model_name": "seedance-asset"
        },
        "data": {
          "model": "seedance-asset",
          "status": "completed",
          "asset_id": "asset-****",
          "asset_uri": "asset://asset-****",
          "metadata": {
            "seedance": {
              "kind": "asset",
              "name": "林春芽",
              "asset_uri": "asset://asset-****",
              "deleted": false
            }
          }
        }
      }
    ]
  }
}
```

### 查询所有视频任务

不加模型过滤时，`/api/task/token/self` 返回当前 API Key 创建的所有异步任务，包括视频生成任务和资产任务：

```bash
curl 'https://你的-newapi-域名/api/task/token/self?p=1&page_size=20' \
  -H 'Authorization: Bearer sk-xxxx'
```

如果只想看成功任务，可加：

```bash
curl 'https://你的-newapi-域名/api/task/token/self?status=SUCCESS&p=1&page_size=20' \
  -H 'Authorization: Bearer sk-xxxx'
```

### 筛选真人形象资产

当前列表接口没有单独的 `model=seedance-asset` 查询参数。筛选真人资产时，建议客户端先请求成功任务，再在返回的 `items` 里按模型和 metadata 过滤：

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
    && (data.asset_uri || metadata.asset_uri);
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

然后把该值放到视频生成请求的 `files` 中。

## 删除当前 API Key 创建的真人资产

如果需要删除已创建成功的真人资产，使用 NewAPI 的 API Key 动作接口：

```http
POST /api/task/token/asset/delete
Authorization: Bearer <你的 NewAPI API Key>
Content-Type: application/json
```

请求体只需要传资产任务 ID，也就是创建真人资产时返回的 `id` / `task_id`：

```json
{
  "task_id": "asset_req_xxx"
}
```

这个接口是 NewAPI 后台动作接口，使用 `POST`，不是 OpenAI Videos 风格的 `DELETE /v1/videos/{task_id}`。

示例：

```bash
curl -X POST 'https://你的-newapi-域名/api/task/token/asset/delete' \
  -H 'Authorization: Bearer sk-xxxx' \
  -H 'Content-Type: application/json' \
  -d '{"task_id":"asset_req_xxx"}'
```

成功响应：

```json
{
  "success": true,
  "message": "",
  "data": {
    "task_id": "asset_req_xxx",
    "deleted": true,
    "deleted_at": 1780830000,
    "asset_id": "asset-****",
    "asset_uri": "asset://asset-****"
  }
}
```

删除规则：

- 只能删除当前请求使用的 API Key 创建的资产任务；同一账号下其他 API Key 创建的任务不能删。
- 只能删除已经成功的 `seedance-asset` 资产任务。
- 请求体不支持直接传 `asset_id`、`asset_uri` 或上游 `resource_id`；NewAPI 会先校验当前 API Key 对 `task_id` 的归属，再把对应的上游任务 ID 转发给上游删除。
- 删除不会移除任务历史，`/api/task/token/self` 仍可能返回这条任务，但 `data.deleted` 和 `data.metadata.seedance.deleted` 会变为 `true`。
- 客户端筛选“可用真人资产”时必须排除 `deleted=true` 的任务。
- 删除资产不等于取消任务，也不会触发退款；它只是删除已创建成功的上游资源库资产。

### 如何拿到可删除的 `task_id`

如果本地没有保存创建资产时返回的 `task_id`，可以先查询当前 API Key 的成功任务：

```bash
curl 'https://你的-newapi-域名/api/task/token/self?status=SUCCESS&p=1&page_size=20' \
  -H 'Authorization: Bearer sk-xxxx'
```

然后在返回的 `data.items` 中筛选：

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

删除时传：

```json
{
  "task_id": "<上面筛选出的 task.task_id>"
}
```

### 常见失败情况

接口失败时仍返回 NewAPI 后台 API 风格结构：

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

## 限制说明

- 图片必须是公网可访问 URL。
- 不支持本地文件、multipart 文件上传、base64 或 data URL。
- 资产处理是异步的，创建接口不会立即返回 `asset_id` 或 `asset_uri`。
- `/api/task/token/self` 只返回当前 API Key 创建的新任务；补丁上线前没有写入 `token_id` 的历史任务查不到。
- 如果任务失败或超时，请重新创建资产或联系服务商。
