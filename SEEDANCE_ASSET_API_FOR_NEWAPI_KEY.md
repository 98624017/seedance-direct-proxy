# Seedance 真人形象资产 API 使用说明

本文档已合并到唯一 NewAPI 调用文档：

```text
SEEDANCE_NEWAPI_OPENAI_VIDEOS_API.md
```

后续请只维护和阅读 `SEEDANCE_NEWAPI_OPENAI_VIDEOS_API.md`，其中已经包含：

- `model="seedance-asset"` 创建真人形象资产。
- `GET /v1/videos/{task_id}` 查询资产处理结果。
- `asset_uri` / `asset://asset-xxxx` 在视频生成 `files` 中的用法。
- `/api/task/token/self` 查询当前 API Key 的所有任务并筛选可用真人资产。
- `POST /api/task/token/asset/delete` 删除当前 API Key 创建的成功资产。
