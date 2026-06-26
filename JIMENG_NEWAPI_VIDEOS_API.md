# JM 系列 NewAPI 视频调用文档

本文档面向下游客户端，说明通过 NewAPI `/v1/videos` 调用 JM 系列视频任务。本文档只描述新渠道；旧 Seedance 资产库和 `seedance-asset` 不在本文档范围内。

## 基础信息

```http
Authorization: Bearer <NEWAPI_API_KEY>
Content-Type: application/json
```

创建任务：

```http
POST /v1/videos
```

查询任务：

```http
GET /v1/videos/{task_id}
```

## 请求字段

| 字段 | 必填 | 说明 |
|---|---:|---|
| `model` | 是 | NewAPI 中配置的 JM 系列模型名，例如 `jimeng-video-seedance-2.0-vip` |
| `prompt` | 是 | 视频提示词；引用素材时使用 `@1`、`@2` |
| `files` | 按模式 | 公网 URL 字符串或数组 |
| `reference_mode` | 否 | 英文枚举；缺省时有 `files` 默认为 `omni`，无 `files` 默认为 `text_to_video` |
| `duration` | 否 | 秒数；缺省为 `4` |
| `ratio` | 否 | 画面比例；缺省交给上游 |
| `aspect_ratio` | 否 | 兼容字段，未传 `ratio` 时会转为 `ratio` |
| `resolution` | 否 | 清晰度；缺省为 `720p` |

素材只推荐使用 `files`。公网 URL 会原样提交给上游，不会由代理下载或转存。

URL 必须是公网 `http://` 或 `https://` 地址；`localhost`、回环 IP、私网 IP、链路本地地址、本地路径、base64、data URL 都会被拒绝。

## 模型命名

下游文档和产品展示建议把该 Seedance/Jimeng 视频模型系列命名为 **JM 系列**。

当前已验证的模型：

```text
jimeng-video-seedance-2.0-vip
```

## 参考模式

`reference_mode` 只支持英文枚举：

| 模式 | URL 数量 | 说明 |
|---|---:|---|
| `text_to_video` | 0 | 文生视频，不能传 `files` |
| `omni` | 1-12 | 全能参考，图片/视频/音频 URL 总数最多 12 个 |
| `first_frame` | 1 | 首帧参考 |
| `last_frame` | 1 | 尾帧参考 |
| `both_frames` | 2 | 首尾帧参考，`files[0]` 是首帧，`files[1]` 是尾帧 |

`both_frames` 允许首帧和尾帧使用同一个 URL。

## 示例

文生视频：

```bash
curl -X POST 'https://your-newapi.example/v1/videos' \
  -H 'Authorization: Bearer <NEWAPI_API_KEY>' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "jimeng-video-seedance-2.0-vip",
    "prompt": "城市夜景中一辆跑车缓慢驶过，电影感镜头",
    "reference_mode": "text_to_video",
    "duration": 4,
    "resolution": "720p"
  }'
```

全能参考：

```bash
curl -X POST 'https://your-newapi.example/v1/videos' \
  -H 'Authorization: Bearer <NEWAPI_API_KEY>' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "jimeng-video-seedance-2.0-vip",
    "prompt": "@1 图片中的人物在舞台上挥手",
    "reference_mode": "omni",
    "ratio": "4:3",
    "duration": 4,
    "resolution": "720p",
    "files": [
      "https://cdn.example.com/person.jpg"
    ]
  }'
```

首帧参考：

```json
{
  "model": "jimeng-video-seedance-2.0-vip",
  "prompt": "从首帧画面开始，镜头缓慢推进",
  "reference_mode": "first_frame",
  "duration": 4,
  "resolution": "720p",
  "files": ["https://cdn.example.com/first.jpg"]
}
```

尾帧参考：

```json
{
  "model": "jimeng-video-seedance-2.0-vip",
  "prompt": "最终停在参考尾帧的构图",
  "reference_mode": "last_frame",
  "duration": 4,
  "resolution": "720p",
  "files": ["https://cdn.example.com/last.jpg"]
}
```

首尾帧参考：

```json
{
  "model": "jimeng-video-seedance-2.0-vip",
  "prompt": "从 @1 平滑过渡到 @2",
  "reference_mode": "both_frames",
  "duration": 4,
  "resolution": "720p",
  "files": [
    "https://cdn.example.com/first.jpg",
    "https://cdn.example.com/last.jpg"
  ]
}
```

## 创建响应

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

## 查询响应

生成中：

```json
{
  "id": "8e8c4f3a2d6b4c9f",
  "task_id": "8e8c4f3a2d6b4c9f",
  "object": "video",
  "status": "in_progress",
  "progress": 50
}
```

成功：

```json
{
  "id": "8e8c4f3a2d6b4c9f",
  "task_id": "8e8c4f3a2d6b4c9f",
  "object": "video",
  "status": "completed",
  "progress": 100,
  "url": "https://cdn.example.com/video.mp4",
  "video_url": "https://cdn.example.com/video.mp4"
}
```

失败：

```json
{
  "id": "8e8c4f3a2d6b4c9f",
  "task_id": "8e8c4f3a2d6b4c9f",
  "object": "video",
  "status": "failed",
  "progress": 100,
  "error": {
    "message": "生成失败：安全审核未通过"
  }
}
```

上游余额或权益不足时，也会以失败任务对象返回：

```json
{
  "id": "task_xxx",
  "task_id": "task_xxx",
  "object": "video",
  "model": "jimeng-video-seedance-2.0-vip",
  "status": "failed",
  "progress": 100,
  "error": {
    "message": "即梦账号积分或权益不足，请联系管理员处理。"
  },
  "metadata": {
    "jimeng": {
      "cost": 200,
      "status": "failed",
      "raw_status": "failed",
      "progress": "失败",
      "input_files": [
        {
          "index": 0,
          "type": "image",
          "url": "https://example.com/person.jpg"
        },
        {
          "index": 1,
          "type": "video",
          "url": "https://example.com/reference.mp4"
        }
      ]
    }
  }
}
```

说明：

- `metadata.jimeng.cost` 是上游返回的任务消耗信息，不由代理计算。
- `metadata.jimeng.input_files` 是上游识别后的素材列表；图片 URL 会标记为 `image`，视频 URL 会标记为 `video`。
- 通过 NewAPI 查询时，`id` 和 `task_id` 都应保持 NewAPI 公开任务 ID，不暴露上游真实任务 ID。
- 真实链路验证中，只传 1 个图片 URL 时，上游也能创建任务并返回 `input_files[0].type=image`；若上游内部依赖异常，可能失败为 `getaddrinfo EAI_AGAIN commerce-api-sg.capcut.com`。
- 真实链路验证中，传 2 个不同图片 URL 和中文提示词 `两只猫一起在公园里玩耍` 时，任务可从 `queued` / `in_progress` 正常完成为 `completed`，并返回可访问的 mp4 `url` / `video_url`。
- 真实链路验证中，传 2 个图片 URL、1 个 PNG 角色图 URL、1 个 mp4 URL 时，上游可按顺序识别为 `image,image,image,video`；若生成服务繁忙，任务会以 HTTP 200 任务对象返回 `status=failed` 和错误 `生成服务暂时繁忙，请稍后重试。`。

结果过期或缺少结果 URL 时，任务仍保持 `completed`，避免把上游已扣费任务误判为失败：

```json
{
  "id": "8e8c4f3a2d6b4c9f",
  "task_id": "8e8c4f3a2d6b4c9f",
  "object": "video",
  "status": "completed",
  "progress": 100,
  "error": {
    "code": "result_expired",
    "message": "result expired, please regenerate"
  },
  "metadata": {
    "jimeng": {
      "result_expired": true
    }
  }
}
```

## 错误处理

- 提交参数错误返回 HTTP 400。
- 上游限流返回 HTTP 429。
- 上游不可用、网络错误或响应异常返回 502/504 类错误。
- 查询到任务 `failed` 时，接口返回 HTTP 200 的任务对象，并在 `error.message` 中给出原因。
- 账号积分或权益不足属于上游任务失败，查询响应为 HTTP 200，`status=failed`，`error.message` 会包含上游提示。
- 未知上游状态会按处理中返回，避免提前判失败。

## 不支持

- 不支持 `seedance-asset`。
- 不支持 `asset://...`。
- 不支持 multipart 文件上传。
- 不支持 base64、data URL、本地路径或内网 URL。
- 不支持中文 `reference_mode`；请使用英文枚举。
