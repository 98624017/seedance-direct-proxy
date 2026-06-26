# Seedance 资产库异步任务

## Goal

让下游用户继续通过 NewAPI 兼容的 `/v1/videos` 异步任务机制，提交真人图片 URL 到 Seedance 资产库，最终通过任务查询拿到上游返回的 `AssetId`，供后续 Seedance 视频生成使用。

核心价值：

- 不新增下游自定义端点，尽量复用 NewAPI 已有 `/v1/videos`、任务入库、后台轮询、用户隔离和计费机制。
- 对下游用户隐藏 Seedance 原始资产库接口细节。
- 初版不向下游提供资产删除能力；后续扩展删除时，采用 NewAPI 后台 API 风格入口并保持代理无状态。

## Confirmed Facts

- 当前 `seedance-direct-proxy` 已支持：
  - `POST /v1/videos` 创建 Seedance 视频任务。
  - `GET /v1/videos/{task_id}` 查询 Seedance 视频任务。
  - 从 `Authorization: Bearer ...` 提取上游 token，并兼容 `baseUrl|token` 格式。
  - `model=seedance-asset` 的资产库异步任务分支，以及 `POST /api/task/token/asset/delete` 的资产删除转发能力。
- 当前 `new-api` 已支持：
  - `POST /v1/videos` 进入 `controller.RelayTask`，提交后写入 `model.Task`。
  - `GET /v1/videos/:task_id` 进入 `controller.RelayTaskFetch`，按当前用户读取本地任务记录。
  - 后台 `TaskPollingLoop` 会按任务 `Platform/ChannelId` 调 adaptor 的 `FetchTask`，并用 `ParseTaskResult` 更新任务状态。
  - Sora/OpenAI 视频 adaptor 会把上游返回的 `id/task_id/status/progress/metadata` 作为任务数据保存。
- 上游资产库文档显示：
  - 添加资源：`POST /resources/user/Resources`，请求包含 `Name` 和 `OssPath`，成功响应不包含 `AssetId`。
  - 添加资源请求中的 `Name` 文档语义是资源名称，不是 NewAPI 用户名或上游创建者名称。
  - 查询资源列表：`POST /resources/user/ResourcesList`，响应列表项中可能包含 `AssetId`。
  - `AssetId` 表示资源已加白成功。
- 用户明确要求：
  - 使用异步方式。
  - 端点尽量继续复用 `/v1/videos` 那套机制，即使语义变成资产任务。
  - 初版下游不提供删除资产功能。
  - 后续新增删除资产能力时，优先保持 `seedance-direct-proxy` 无状态，由 NewAPI 负责用户/API Key 权限判断和本地任务标记。
- 2026-06-26 新增上游接入要求：
  - 老渠道机制必须保留，因为老上游后续可能恢复。
  - 需要接入新上游 `https://api.aizhw.cc`，用于老上游长期维护期间承接视频生成。
  - 对下游暴露的 `/v1/videos` 请求体尽量保持不变，由本代理负责改写并转发到新上游。
  - 暂不需要做模型映射；下游愿意更换模型 ID。
  - 新渠道不需要单独创建资产真人库；真人图直接作为公网图片 URL 传入视频任务即可。
  - 新渠道支持单独的参考模式能力，例如首帧、尾帧、首尾帧、全能参考，需要扩展给下游使用。
  - 项目完成后，需要单独写一份适合新渠道的下游 NewAPI 调用文档。
- 新上游文档确认：
  - 创建异步任务为 `POST /v1/videos/tasks`。
  - 查询任务为 `GET /v1/videos/tasks/{task_id}`。
  - JSON 素材字段为 `file_paths` / `filePaths`，公网 URL 数组。
  - 比例字段为 `ratio`，可由现有 `aspect_ratio` 转换。
  - 参考模式字段为 `reference_mode`，支持 `omni`、`first_frame`、`last_frame`、`both_frames`、`text_to_video` 等。
  - 提交成功响应包含 `task_id`、`status=pending`、`created`。
  - 查询成功响应包含 `task_id`、`status`、`progress`、`result.url`，失败时可能包含 `error`。
  - 查询响应可能出现 `result_expired=true`，表示任务曾完成但结果文件已过期。
  - 文档建议单任务轮询间隔 3-5 秒，任务结果默认约 12 小时过期。

## Requirements

- 资产创建请求必须走 `POST /v1/videos`。
- 资产查询必须走 `GET /v1/videos/{task_id}`。
- 资产任务使用虚拟模型名 `seedance-asset`。
- 请求体需要兼容 NewAPI/OpenAI 风格字段：
  - `model`: 虚拟资产模型名。
  - `prompt`: 资源名称/资产显示名，例如真人形象名。
  - `input_reference`、`image`、`images[0]` 或 `files[0]`: 真人图片 URL。
- 资产图片输入只接受公网可访问 URL，必须是 `http://` 或 `https://`。
- 代理层必须做轻量 URL 安全校验：拒绝空 host、localhost、显式回环 IP、私网 IP、链路本地地址；MVP 不做 DNS 解析和重绑定防护。
- MVP 不支持 multipart 文件、本地文件路径、base64 图片或需要本层托管的图片输入。
- 图片 URL 字段优先级固定为 `input_reference -> image -> images[0] -> files[0]`。
- 资产任务一次只创建一个资源；同一请求传入多张图片时只取第一张，并在 metadata 记录 `ignored_image_count`。
- `seedance-direct-proxy` 识别资产模型后，必须调用上游资产库添加资源接口，而不是视频创建接口。
- 资产任务提交响应必须返回 OpenAI Video 风格异步任务对象，供 NewAPI 原有 task adaptor 正常入库。
- 资产任务响应保持 `object: "video"`，通过顶层 `asset_id` 和 `metadata.seedance.kind="asset"` 标识资产类型。
- 资产任务查询必须通过上游资源列表查找对应资源，并将上游状态归一化成视频任务状态：
  - 未找到或无 `AssetId` 且无明确失败信息：`in_progress`。
  - 找到且 `AssetId` 非空：`completed`。
  - 上游资源列表接口失败，或资源项 `Message`/`StatusText` 明确包含失败语义：`failed`。
- 成功查询结果必须在 `metadata.seedance.asset_id` 或等价 metadata 字段中返回 `AssetId`。
- 成功查询结果必须同时在顶层 `asset_id` 和 `metadata.seedance.asset_id` 返回 `AssetId`，方便下游直接读取核心结果，也保留上游详情。
- 为了保持代理无状态，提交到上游资产库的资源名称 `Name` 带短追踪后缀，例如 `林春芽__ar_abcdef123456`。
- 下游响应必须剥离追踪后缀，展示用户原始资源名称/资产显示名。
- 资产任务 ID 格式为 `asset_req_<unix>_<random>`，其中 `<unix>` 为创建时间戳，用于无状态计算任务年龄，`<random>` 固定为 12 位小写 hex。
- 上游资源名追踪后缀格式固定为 `<资源名称>__ar_<random>`，剥离展示名时按最后一次出现的 `__ar_` 切分。
- 上游资产库 `Name` 最大 50 个 Unicode 字符；由于短追踪后缀固定占 17 个字符，下游资产显示名最大 33 个 Unicode 字符。
- 查询或删除资产资源时，必须从任务 ID 提取 `<random>`，按完整短追踪后缀 `__ar_<random>` 匹配资源名，不能用任意包含匹配。
- NewAPI 侧必须保留用户隔离：用户只能通过自己的 NewAPI key 查询自己提交的资产任务。
- 资产任务按虚拟模型按次计费：提交成功即扣费；后续上游明确失败或任务超时时，沿用 NewAPI 现有任务退款机制。
- 资产模型不叠加视频时长、分辨率、参考视频等视频生成倍率。
- NewAPI `/v1/videos` 对 `seedance-asset` 必须走资产任务分支：只校验 `model`、资源名称/资产显示名和公网图片 URL，不套用视频时长、分辨率或参考视频计费估算。
- `seedance-asset` 不写入 NewAPI 默认模型/价格表；管理员需要在 NewAPI 后台手动配置模型、渠道和按次价格。
- 资源列表查询页数按任务年龄递增，并通过环境变量可调：
  - 0-10 分钟：默认扫描 10 页。
  - 10-60 分钟：默认扫描 20 页。
  - 60 分钟以后：默认扫描 50 页。
- 新增资产删除能力时，删除入口必须由 NewAPI 暴露给下游，`seedance-direct-proxy` 只作为无状态上游删除转发。
- 资产删除入口固定为 `POST /api/task/token/asset/delete`，使用 `Authorization: Bearer <NewAPI API Key>`。
- 资产删除请求体 MVP 只支持 `{"task_id":"asset_req_xxx"}`，不支持直接按 `asset_id` 或 `resource_id` 删除。
- 下游删除资产必须按当前 API Key 隔离，只允许删除当前 key 创建的 `seedance-asset` 成功任务。
- 删除请求不得让下游直接指定上游 `resource_id` 作为唯一凭据；NewAPI 必须先从任务表中读取并校验归属。
- NewAPI 删除成功后必须保留任务历史，并在任务 `data` / `metadata.seedance` 中标记 `deleted=true`、`deleted_at`。
- 资产删除成功响应必须使用 NewAPI 后台 API 风格最小结构，包含 `success=true` 和 `data.task_id/deleted/deleted_at/asset_id/asset_uri`。
- `/api/task/token/self` 仍可返回已删除资产任务；下游筛选可用资产时必须排除 `deleted=true`。
- MVP 不新增“我的资产列表”专用 API；用户查询已通过资产时，先通过现有任务记录/任务日志按 `seedance-asset` 和成功状态过滤，或用 `GET /v1/videos/{task_id}` 查询单个任务。
- 新上游接入必须与老渠道共存，不能删除或破坏：
  - 老视频上游 `/seedanceapi/common/File/All`、`/seedanceapi/user/DataIndex`。
  - 老资产库上游 `/resources/user/Resources`、`/resources/user/ResourcesList`、资产删除转发。
  - 现有 `seedance-asset` 资产任务合同。
- 新上游视频创建仍对下游暴露 `POST /v1/videos`，代理转发到新上游 `POST /v1/videos/tasks`。
- 新上游视频查询仍对下游暴露 `GET /v1/videos/{task_id}`，代理转发到新上游 `GET /v1/videos/tasks/{task_id}`。
- 新上游任务 ID 按字符串处理；`jimeng` 模式下查询跳过老渠道数字 ID 校验，但仍拒绝空值和包含 `/` 的路径。
- 新上游模式下，下游可以直接把真人图片 URL 放到现有参考素材字段中，不需要先创建 `seedance-asset`。
- 新上游 MVP 只支持 JSON 公网 URL 素材，不支持 multipart 文件上传、本地文件路径、base64 或 data URL。
- 新上游模式下，代理应兼容现有下游字段并转换：
  - `ratio` 优先透传；未传 `ratio` 时 `aspect_ratio` -> `ratio`；两者都缺失时不补默认值，交给新上游默认。
  - `resolution` 透传；缺失时默认 `720p`。
  - `files`、`input_reference`、`file_paths`、`filePaths` 四类参考素材字段 -> `file_paths`。
  - 四类参考素材字段按 `files -> input_reference -> file_paths -> filePaths` 顺序收集，保留重复 URL 和原始顺序。
  - 下游新渠道文档只表达 `files`，其余字段仅作为代理兼容输入，不作为推荐对外合同。
  - `duration` / `seconds` 映射为新上游 `duration`，并统一以 JSON number 提交给新上游；两者都缺失时默认 `4`。
  - 代理不校验 `duration` 范围、模型时长上限或是否整数；相关错误交给新上游返回。
  - 如果下游未传 `reference_mode`，有参考 URL 时自动补 `omni`，无参考 URL 时自动补 `text_to_video`。
  - `reference_mode` 作为正式下游能力透传到新上游，至少支持 `omni`、`first_frame`、`last_frame`、`both_frames`、`text_to_video`。
  - `reference_mode` 只支持英文枚举，不兼容中文别名。
  - `reference_mode=omni` 时 URL 数量必须为 1-12 个；0 个或超过 12 个返回 400。
  - `reference_mode=text_to_video` 时不能传 URL；如果请求包含参考 URL，返回 400。
  - `reference_mode=first_frame` 或 `last_frame` 时，URL 数量必须正好 1 个，否则返回 400。
  - `reference_mode=both_frames` 时，`file_paths[0]` 固定为首帧，`file_paths[1]` 固定为尾帧；URL 数量必须正好 2 个，否则返回 400。
  - `prompt` 原样透传，代理不自动改写 `@图片1`、`@图片2` 等占位符；新渠道下游文档只推荐 `@1`、`@2`。
  - `model` 必填，直接透传，不做本层模型映射，也不补默认模型。
- 新上游模式下，公网 URL 原样转发为新上游 `file_paths`，代理不下载、不转存、不 multipart 上传。
- 新上游模式下 URL 校验沿用现有轻量公网 URL 安全校验：只接受 `http://` / `https://`，拒绝空 host、`localhost`、显式回环 IP、私网 IP、链路本地地址；不做 DNS 解析和重绑定防护。
- 新上游不支持或无需承接 `asset://...` 资产引用；若请求里包含 `asset://...`，需要按决策选择拒绝、忽略或仍走老渠道。
- 新上游模式下，如果参考素材中包含 `asset://...`，代理必须拒绝请求并返回 400，提示新上游不支持旧资产引用、需要改传公网图片 URL。
- 新上游模式下，如果下游调用 `model=seedance-asset`，代理必须拒绝请求并返回 400，提示新上游不需要创建资产，需改用视频模型并把真人图片 URL 放进 `files` / `input_reference` 等参考素材字段。
- 因默认 provider 为 `jimeng`，默认配置下 `model=seedance-asset` 会被拒绝；只有显式设置 `VIDEO_UPSTREAM_PROVIDER=legacy` 后才继续使用老资产库分支。
- 新上游返回 `result_expired=true` 时，代理保持 `status=completed`，不填 `url` / `video_url`，在 `metadata.jimeng.result_expired=true` 和 `error.message` 中提示结果已过期、需要重新生成。
- 新上游返回 `status=completed` 但没有 `result.url` 且没有 `result_expired=true` 时，代理仍保持 `status=completed`，不填 `url` / `video_url`，在 `metadata.jimeng.missing_result_url=true` 和 `error.message` 中提示上游完成但未返回结果 URL；不能映射成 `failed`，避免触发下游退款语义。
- 新上游错误处理必须统一改写成下游可理解的 OpenAI 风格错误：
  - 创建/查询上游返回 400、401、429、503 等 HTTP 错误时，代理保留合适 HTTP 状态并返回 `error.message`，同时在 detail/metadata 中保留上游错误摘要。
  - 创建/查询上游返回非 2xx、网络错误、超时、响应 JSON 解析失败时，代理返回 502/504 类错误，不伪造成任务成功。
  - 查询任务返回 `status=failed` 时，代理返回 HTTP 200 的任务对象，`status=failed`、`progress=100`，并把上游 `error` 写入 `error.message` 和 `metadata.jimeng.error`。
  - 查询任务返回未知状态时，代理保守映射为 `in_progress`，并在 `metadata.jimeng.raw_status` 保留原始状态，避免提前失败。
  - 上游 429 不在代理层自动重试，直接把限流信息返回给 NewAPI/下游，由轮询侧按间隔控制。
- 新老上游分流采用整实例配置切换：
  - 默认走新上游 `jimeng`，因为当前老渠道不可用。
  - 通过环境变量选择新上游，例如 `VIDEO_UPSTREAM_PROVIDER=legacy|jimeng`。
  - `legacy` 模式继续使用老视频上游和资产库能力。
  - `jimeng` 模式下普通视频创建/查询转发到新上游；老资产库分支仍保留在代码中，供切回老渠道时使用。
  - `VIDEO_UPSTREAM_PROVIDER` 配置为未知值时，回退到 `jimeng` 并记录 warning 日志。

## Acceptance Criteria

- [ ] 下游调用 `POST /v1/videos`，传入资产模型、名称和图片 URL 后，NewAPI 返回一个公开 `task_id`，并在本地任务表中记录任务。
- [ ] 多图片输入只创建一个资产，按固定优先级取第一张，并在响应 metadata 中记录被忽略图片数量。
- [ ] 资产图片 URL 为 localhost、回环 IP、私网 IP、链路本地地址或空 host 时，请求被拒绝。
- [ ] `seedance-direct-proxy` 对资产模型调用上游 `/resources/user/Resources`，请求头使用上游 `token`，请求体包含可追踪的 `Name` 和图片 `OssPath`。
- [ ] NewAPI 后台轮询该任务时，能够通过代理的 `GET /v1/videos/{asset_task_id}` 查询到资产处理状态。
- [ ] 资产处理成功后，`GET /v1/videos/{task_id}` 返回 `status=completed`，并包含 `AssetId`。
- [ ] 代理查询资产状态时根据任务年龄递增扫描上游资源列表页数，未找到则返回 `in_progress` 并等待后续轮询或任务超时。
- [ ] 资产任务响应保持 OpenAI Video 兼容结构，`object` 为 `video`，不破坏普通视频客户端处理流程。
- [ ] 资产任务失败时，任务状态更新为失败，并向用户返回可读错误信息。
- [ ] 未找到资源项、资源项无 `AssetId`、或上游状态枚举未知时，不提前判失败，而是返回 `in_progress` 等待后续轮询或超时。
- [ ] 资产任务提交成功即按 `seedance-asset` 模型价格计费；失败/超时按现有任务退款机制处理。
- [ ] NewAPI 侧 `seedance-asset` 不会因缺少视频专用参数失败，也不会叠加视频倍率。
- [ ] 普通 Seedance 视频模型的 `/v1/videos` 创建和查询行为不回归。
- [ ] 如果启用资产删除，下游只能删除当前 API Key 创建的已成功资产任务，不能删除同用户其他 key 或其他用户的资产。
- [ ] 如果启用资产删除，下游通过 `POST /api/task/token/asset/delete` 和 `task_id` 删除资产，不需要也不能传上游 `resource_id`。
- [ ] 如果启用资产删除，删除成功后上游资产库资源被删除，NewAPI 任务历史仍保留但标记为 deleted。
- [ ] 如果启用资产删除，删除成功响应为最小结构：`success=true`，`data.deleted=true`，并包含 `task_id`、`deleted_at`、`asset_id`、`asset_uri`。
- [ ] 如果启用资产删除，已删除资产不会出现在“可用真人资产”筛选结果中。
- [ ] 两个仓库的相关单元测试通过。
- [ ] 老渠道配置不变时，普通视频、资产创建、资产查询和资产删除行为不回归。
- [ ] 启用新上游后，下游仍调用 `POST /v1/videos` 创建视频任务，代理实际请求新上游 `/v1/videos/tasks`。
- [ ] 新上游模式下缺少 `model` 返回 400，不补默认模型。
- [ ] 启用新上游后，下游仍调用 `GET /v1/videos/{task_id}` 查询任务，代理实际请求新上游 `/v1/videos/tasks/{task_id}`。
- [ ] `jimeng` 模式下支持非数字字符串任务 ID 查询，同时拒绝空任务 ID 和包含 `/` 的路径。
- [ ] 启用新上游后，`files` / `input_reference` / `file_paths` / `filePaths` 中的公网 URL 能转换成新上游 `file_paths`。
- [ ] 多个兼容素材字段混用时，按 `files -> input_reference -> file_paths -> filePaths` 收集，保留重复 URL 和原始顺序。
- [ ] 启用新上游后，`aspect_ratio` 能转换成新上游 `ratio`。
- [ ] `ratio` / `aspect_ratio` 都缺失时，代理不补默认比例。
- [ ] `resolution` 缺失时，新上游请求默认提交 `resolution="720p"`。
- [ ] 启用新上游后，字符串或数字形式的 `duration` / `seconds` 都能转换成 JSON number 类型的 `duration`。
- [ ] `duration` / `seconds` 都缺失时，新上游请求默认提交 `duration=4`。
- [ ] 代理不因 `duration` 超范围或模型时长上限主动拒绝；上游错误按错误归一化返回。
- [ ] 新上游返回 completed 时，下游响应保持 OpenAI Video 风格，并在 `url` / `video_url` 返回 `result.url`。
- [ ] 新上游失败响应能映射成 `status=failed` 和可读 `error.message`。
- [ ] 默认配置下走新上游 `jimeng`；显式设置 `VIDEO_UPSTREAM_PROVIDER=legacy` 后才走老上游。
- [ ] `VIDEO_UPSTREAM_PROVIDER` 配置为未知值时回退到 `jimeng`，并记录可排查的 warning 日志。
- [ ] 新上游模式下传入 `asset://...` 时返回 400，不自动回退老渠道。
- [ ] 新上游模式下只接受 JSON URL 素材；multipart、本地文件、base64、data URL 不在 MVP 范围内。
- [ ] 新上游模式下调用 `model=seedance-asset` 返回 400，不继续调用旧资产库分支。
- [ ] 默认配置下调用 `model=seedance-asset` 返回 400；显式 `VIDEO_UPSTREAM_PROVIDER=legacy` 时老资产库行为不回归。
- [ ] 新上游返回 `result_expired=true` 时，下游响应仍为 `status=completed`，并通过 metadata/error 表达结果不可下载。
- [ ] 新上游返回 `completed` 但缺少 `result.url` 且未标记过期时，下游响应仍为 `status=completed`，metadata/error 表达缺少结果 URL，不映射为 failed。
- [ ] 新上游模式下公网 URL 原样进入 `file_paths`，代理不预下载素材。
- [ ] 新上游模式下 URL 为 localhost、回环 IP、私网 IP、链路本地地址、空 host、非 HTTP(S) scheme 时返回 400。
- [ ] 新上游模式下缺少 `reference_mode` 时，有参考 URL 自动补 `omni`，无参考 URL 自动补 `text_to_video`。
- [ ] 新上游模式下下游可显式传 `reference_mode=first_frame`、`last_frame`、`both_frames`、`omni` 或 `text_to_video`，代理正确透传。
- [ ] 新上游模式下中文 `reference_mode` 不做兼容转换；下游文档只给英文枚举。
- [ ] `reference_mode=omni` 时 URL 数量必须为 1-12 个；0 个或超过 12 个都返回 400。
- [ ] `reference_mode=text_to_video` 时不能传 URL；带 URL 返回 400。
- [ ] `reference_mode=first_frame` 或 `last_frame` 时必须正好 1 个 URL；少于或多于 1 个都返回 400。
- [ ] `reference_mode=both_frames` 时必须正好 2 个 URL，顺序为首帧、尾帧；少于或多于 2 个都返回 400。
- [ ] 代理不改写 prompt 中的素材引用占位符；新渠道下游文档只推荐 `@1`、`@2`。
- [ ] 新上游创建接口返回非 2xx、限流、网络错误、超时或非法 JSON 时，代理返回明确错误，不创建伪任务。
- [ ] 新上游查询接口返回 `status=failed` 时，代理返回 OpenAI Video 风格失败任务对象，并包含可读错误信息。
- [ ] 新上游查询接口返回未知状态时，代理不提前判失败，返回 `in_progress` 并保留原始状态。
- [ ] 新上游 429 被明确透传为限流错误，代理不做无界重试。
- [ ] 项目完成后新增一份面向下游 NewAPI 的新渠道调用文档，覆盖模型、`files` URL 素材、`reference_mode`、首帧/尾帧/首尾帧示例、状态查询和错误处理；文档不对外推荐 `input_reference`、`file_paths`、`filePaths`。

## Out of Scope

- 直接在 `seedance-direct-proxy` 面向下游开放无权限判断的删除接口。
- 直接按用户传入的 `asset_id` 或上游 `resource_id` 删除且不校验任务归属。
- NewAPI 后台管理页面删除资产。
- 独立资产表和独立资产列表 API。
- 专用“我的资产列表”下游 API。
- 为避免上游资产名带追踪信息而新增持久化映射表。
- 同步等待 `AssetId` 返回。
- 对上游资产分组进行创建、删除或复杂管理。
- multipart/base64 图片上传与临时对象存储托管。
- 单请求批量创建多个资产。
- 完整 DNS 解析、DNS 重绑定防护或远程图片内容预检。
- 修改 NewAPI 默认模型/价格配置。
- 移除老视频渠道、老资产库渠道或资产删除转发。
- 在本层做新上游模型映射。
- 为新上游单独创建真人资产库。
- 新增下游公开端点替代 `/v1/videos`。
- 为新上游实现复杂素材预检或自动判断图片/视频/音频内容类型。

## Open Questions

- 资产 token 池是否放在 `seedance-direct-proxy` 的环境变量里，仅用于资产查询/删除 fallback，不落库、不暴露给 `new-api`？
- 如果采用代理层 token 池，是否接受创建资产仍只使用请求里传入的 token，而查询/删除才允许在池内逐个尝试？

## Current Direction

- 资产 token 池放在 `seedance-direct-proxy` 的环境变量里，仅用于资产查询/删除 fallback，不落库、不暴露给 `new-api`。
- `new-api` 只保留任务归属、上游 task id、渠道 id 等业务字段。
- 资产创建仍只使用请求 token；资产查询/删除在代理层按任务名和 token 池兜底。
- 新老上游采用整实例配置切换，推荐环境变量名为 `VIDEO_UPSTREAM_PROVIDER`，默认 `jimeng`，老上游值为 `legacy`。
- 新上游模式下 `model` 必填，不补默认模型。
- `VIDEO_UPSTREAM_PROVIDER` 配错时回退到 `jimeng`，因为当前老渠道不可用。
- `jimeng` 模式下查询任务 ID 使用字符串，不走老渠道数字 ID 解析。
- 新上游 MVP 只支持 JSON 公网 URL 素材，不支持 multipart 文件上传透传。
- 新上游模式下 `model=seedance-asset` 直接拒绝，请下游改用视频模型 + 公网 URL。
- 默认配置下接受 `seedance-asset` 被拒绝；老资产库只在显式 `legacy` 模式启用。
- 新上游 `result_expired=true` 映射为已完成但结果过期，不映射成生成失败。
- 新上游 completed 但缺少结果 URL 时仍保持 completed，不映射成 failed，避免下游退款语义和上游扣费不一致。
- 新上游模式下公网 URL 原样转发为 `file_paths`，代理不下载、不转存。
- 新上游模式下沿用现有轻量公网 URL 安全校验。
- 新上游模式下 `duration` 统一以 JSON number 提交给上游。
- `duration` 缺失时默认 `4`。
- `ratio` / `aspect_ratio` 缺失时不补默认值，交给上游。
- `resolution` 缺失时默认 `720p`。
- `duration` 范围和模型上限交给上游校验。
- 新上游模式下缺少 `reference_mode` 时由代理自动补默认值：有参考 URL 为 `omni`，无参考 URL 为 `text_to_video`。
- 新上游的 `reference_mode` 作为下游可显式使用的能力暴露，项目完成时需要写独立下游调用文档。
- `reference_mode` 只支持英文枚举，不兼容中文别名。
- `reference_mode=omni` 强约束为 1-12 个 URL。
- `reference_mode=text_to_video` 强约束为 0 个 URL。
- `reference_mode=first_frame` / `last_frame` 强约束为正好 1 个 URL。
- `reference_mode=both_frames` 强约束为正好 2 个 URL，顺序为首帧、尾帧。
- prompt 原样透传，不自动改写素材引用占位符。

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
