# JM/Jimeng 失败任务上游请求与报错整理

整理时间：2026-06-26

本文件用于反馈给上游排查。所有任务均通过本地链路：

```text
NewAPI :39101 -> seedance-direct-proxy :39100 -> https://api.aizhw.cc
```

创建任务上游接口：

```http
POST https://api.aizhw.cc/v1/videos/tasks
Content-Type: application/json
Authorization: Bearer <已省略>
```

查询任务上游接口：

```http
GET https://api.aizhw.cc/v1/videos/tasks/{upstream_task_id}
Authorization: Bearer <已省略>
```

说明：

- 以下 `upstream_request_body` 是本 Go 层改写后发给上游的 JSON 请求体。
- 本层在 Jimeng 模式下会把下游 `files` 改写为上游 `file_paths`，`duration` 统一为 JSON number。
- 以下任务均已在上游查询结果中返回 `raw_status=failed`。
- 请求体不包含 token；本文档未写入任何密钥。

## 汇总

| 序号 | NewAPI 公开任务 ID | 上游任务 ID | 模型 | 素材 | 上游报错 |
|---|---|---|---|---|---|
| 1 | `task_5OBVNnNe7RUDuChZD603qCKRfNQI16mi` | `abf45fa0716911f1be4ef90809f90822` | `jimeng-video-seedance-2.0-fast-vip` | 1 图 + 1 视频 | `即梦账号积分或权益不足，请联系管理员处理。` |
| 2 | `task_1RjwW8QAF6YEgCkly3zZEi5xdAd6zRmC` | `e0ccdb70716a11f1be4ef90809f90822` | `jimeng-video-seedance-2.0-vip` | 1 图 + 1 视频 | `即梦账号积分或权益不足，请联系管理员处理。` |
| 3 | `task_5sXzsQtnIQUarSZTBuUYVb2OZM3MJCoE` | `a4314dd0716b11f1be4ef90809f90822` | `jimeng-video-seedance-2.0-vip` | 1 图 | `getaddrinfo EAI_AGAIN commerce-api-sg.capcut.com` |
| 4 | `task_wY1gmz2VPZIPyjW7nCYpEoTpjSqZsDCL` | `2b6a0400717011f1be4ef90809f90822` | `jimeng-video-seedance-2.0-vip` | 3 图 + 1 视频 | `即梦账号积分或权益不足，请联系管理员处理。` |
| 5 | `task_IXxTIccNVB5qtuAHmFdWxKNnb37JQM64` | `b60a9a20717011f1be4ef90809f90822` | `jimeng-video-seedance-2.0-vip` | 3 图 + 1 视频 | `生成服务暂时繁忙，请稍后重试。` |

## 1. fast-vip：1 图 + 1 视频，权益不足

NewAPI 公开任务 ID：

```text
task_5OBVNnNe7RUDuChZD603qCKRfNQI16mi
```

上游任务 ID：

```text
abf45fa0716911f1be4ef90809f90822
```

上游请求体：

```json
{
  "model": "jimeng-video-seedance-2.0-fast-vip",
  "prompt": "Use @1 as the character reference and @2 as the motion reference. Create a 4 second cinematic clip with the same person doing a gentle turn.",
  "resolution": "720p",
  "duration": 4,
  "reference_mode": "omni",
  "file_paths": [
    "https://picsum.photos/id/237/768/512",
    "https://samplelib.com/preview/mp4/sample-5s.mp4"
  ]
}
```

上游查询失败信息：

```json
{
  "status": "failed",
  "progress": "失败",
  "cost": 156,
  "error": "即梦账号积分或权益不足，请联系管理员处理。",
  "input_files": [
    {
      "index": 0,
      "name": "512",
      "type": "image",
      "url": "https://picsum.photos/id/237/768/512"
    },
    {
      "index": 1,
      "name": "sample-5s.mp4",
      "type": "video",
      "url": "https://samplelib.com/preview/mp4/sample-5s.mp4"
    }
  ]
}
```

## 2. vip：1 图 + 1 视频，权益不足

NewAPI 公开任务 ID：

```text
task_1RjwW8QAF6YEgCkly3zZEi5xdAd6zRmC
```

上游任务 ID：

```text
e0ccdb70716a11f1be4ef90809f90822
```

上游请求体：

```json
{
  "model": "jimeng-video-seedance-2.0-vip",
  "prompt": "Use @1 as the character reference and @2 as the motion reference. Create a 4 second cinematic clip with the same person doing a gentle turn.",
  "resolution": "720p",
  "duration": 4,
  "reference_mode": "omni",
  "file_paths": [
    "https://picsum.photos/id/237/768/512",
    "https://samplelib.com/preview/mp4/sample-5s.mp4"
  ]
}
```

上游查询失败信息：

```json
{
  "status": "failed",
  "progress": "失败",
  "cost": 200,
  "error": "即梦账号积分或权益不足，请联系管理员处理。",
  "input_files": [
    {
      "index": 0,
      "name": "512",
      "type": "image",
      "url": "https://picsum.photos/id/237/768/512"
    },
    {
      "index": 1,
      "name": "sample-5s.mp4",
      "type": "video",
      "url": "https://samplelib.com/preview/mp4/sample-5s.mp4"
    }
  ]
}
```

## 3. vip：只传 1 图，CapCut DNS 失败

NewAPI 公开任务 ID：

```text
task_5sXzsQtnIQUarSZTBuUYVb2OZM3MJCoE
```

上游任务 ID：

```text
a4314dd0716b11f1be4ef90809f90822
```

上游请求体：

```json
{
  "model": "jimeng-video-seedance-2.0-vip",
  "prompt": "Use @1 as the character reference. Create a 4 second cinematic clip with the same person doing a gentle turn.",
  "resolution": "720p",
  "duration": 4,
  "reference_mode": "omni",
  "file_paths": [
    "https://picsum.photos/id/237/768/512"
  ]
}
```

上游查询失败信息：

```json
{
  "status": "failed",
  "progress": "失败",
  "cost": 200,
  "error": "getaddrinfo EAI_AGAIN commerce-api-sg.capcut.com",
  "input_files": [
    {
      "index": 0,
      "name": "512",
      "type": "image",
      "url": "https://picsum.photos/id/237/768/512"
    }
  ]
}
```

## 4. vip：2 猫图 + 真人图 + 视频，权益不足

NewAPI 公开任务 ID：

```text
task_wY1gmz2VPZIPyjW7nCYpEoTpjSqZsDCL
```

上游任务 ID：

```text
2b6a0400717011f1be4ef90809f90822
```

上游请求体：

```json
{
  "model": "jimeng-video-seedance-2.0-vip",
  "prompt": "让 @1 和 @2 两只猫，与 @3 的真人角色一起出现在 @4 参考视频的场景里互动玩耍。两只猫自然靠近人物、追逐、转身，保持真实光影、镜头运动和视频场景氛围。",
  "resolution": "720p",
  "duration": 4,
  "reference_mode": "omni",
  "file_paths": [
    "https://upload.wikimedia.org/wikipedia/commons/thumb/a/af/A_Calico_cat.jpg/960px-A_Calico_cat.jpg",
    "https://upload.wikimedia.org/wikipedia/commons/thumb/c/c5/Odd_Eyed_Black_Cat_looks_at_viewer.jpg/960px-Odd_Eyed_Black_Cat_looks_at_viewer.jpg",
    "https://upload.wikimedia.org/wikipedia/commons/thumb/8/8d/President_Barack_Obama.jpg/960px-President_Barack_Obama.jpg",
    "https://filesamples.com/samples/video/mp4/sample_640x360.mp4"
  ]
}
```

上游查询失败信息：

```json
{
  "status": "failed",
  "progress": "失败",
  "cost": 200,
  "error": "即梦账号积分或权益不足，请联系管理员处理。",
  "input_files": [
    {
      "index": 0,
      "name": "960px-A_Calico_cat.jpg",
      "type": "image",
      "url": "https://upload.wikimedia.org/wikipedia/commons/thumb/a/af/A_Calico_cat.jpg/960px-A_Calico_cat.jpg"
    },
    {
      "index": 1,
      "name": "960px-Odd_Eyed_Black_Cat_looks_at_viewer.jpg",
      "type": "image",
      "url": "https://upload.wikimedia.org/wikipedia/commons/thumb/c/c5/Odd_Eyed_Black_Cat_looks_at_viewer.jpg/960px-Odd_Eyed_Black_Cat_looks_at_viewer.jpg"
    },
    {
      "index": 2,
      "name": "960px-President_Barack_Obama.jpg",
      "type": "image",
      "url": "https://upload.wikimedia.org/wikipedia/commons/thumb/8/8d/President_Barack_Obama.jpg/960px-President_Barack_Obama.jpg"
    },
    {
      "index": 3,
      "name": "sample_640x360.mp4",
      "type": "video",
      "url": "https://filesamples.com/samples/video/mp4/sample_640x360.mp4"
    }
  ]
}
```

## 5. vip：2 猫图 + PNG 角色图 + 视频，生成服务繁忙

NewAPI 公开任务 ID：

```text
task_IXxTIccNVB5qtuAHmFdWxKNnb37JQM64
```

上游任务 ID：

```text
b60a9a20717011f1be4ef90809f90822
```

上游请求体：

```json
{
  "model": "jimeng-video-seedance-2.0-vip",
  "prompt": "让 @1 和 @2 两只猫，与 @3 的角色一起出现在 @4 参考视频的场景里互动玩耍。两只猫自然靠近角色、追逐、转身，角色温和回应它们，保持真实光影、镜头运动和视频场景氛围。",
  "resolution": "720p",
  "duration": 4,
  "reference_mode": "omni",
  "file_paths": [
    "https://upload.wikimedia.org/wikipedia/commons/thumb/a/af/A_Calico_cat.jpg/960px-A_Calico_cat.jpg",
    "https://upload.wikimedia.org/wikipedia/commons/thumb/c/c5/Odd_Eyed_Black_Cat_looks_at_viewer.jpg/960px-Odd_Eyed_Black_Cat_looks_at_viewer.jpg",
    "https://d.uguu.se/UQtgkoMT.png",
    "https://filesamples.com/samples/video/mp4/sample_640x360.mp4"
  ]
}
```

上游查询失败信息：

```json
{
  "status": "failed",
  "progress": "失败",
  "cost": 200,
  "error": "生成服务暂时繁忙，请稍后重试。",
  "input_files": [
    {
      "index": 0,
      "name": "960px-A_Calico_cat.jpg",
      "type": "image",
      "url": "https://upload.wikimedia.org/wikipedia/commons/thumb/a/af/A_Calico_cat.jpg/960px-A_Calico_cat.jpg"
    },
    {
      "index": 1,
      "name": "960px-Odd_Eyed_Black_Cat_looks_at_viewer.jpg",
      "type": "image",
      "url": "https://upload.wikimedia.org/wikipedia/commons/thumb/c/c5/Odd_Eyed_Black_Cat_looks_at_viewer.jpg/960px-Odd_Eyed_Black_Cat_looks_at_viewer.jpg"
    },
    {
      "index": 2,
      "name": "UQtgkoMT.png",
      "type": "image",
      "url": "https://d.uguu.se/UQtgkoMT.png"
    },
    {
      "index": 3,
      "name": "sample_640x360.mp4",
      "type": "video",
      "url": "https://filesamples.com/samples/video/mp4/sample_640x360.mp4"
    }
  ]
}
```
