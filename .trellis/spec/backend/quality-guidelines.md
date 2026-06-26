# Quality Guidelines

> Code quality standards for backend development.

---

## Overview

<!--
Document your project's quality standards here.

Questions to answer:
- What patterns are forbidden?
- What linting rules do you enforce?
- What are your testing requirements?
- What code review standards apply?
-->

(To be filled by the team)

---

## Forbidden Patterns

<!-- Patterns that should never be used and why -->

(To be filled by the team)

---

## Required Patterns

### Scenario: Seedance Video `files` Streaming Upload

#### 1. Scope / Trigger
- Trigger: Seedance upstream video create API accepts repeated multipart `files` values as uploaded files, while also accepting `asset://<asset_id>` resource-library addresses.
- Scope: Normal video creation through `POST /v1/videos`; this does not change `seedance-asset` asset creation, which uses JSON `OssPath`.

#### 2. Signatures
- Downstream API: `POST /v1/videos` with JSON fields `model`, `prompt`, optional video settings, and reference fields such as `files`, `images`, `image`, `input_reference`.
- Upstream API: `POST /seedanceapi/common/File/All` with `multipart/form-data` and repeated fields named `files`.

#### 3. Contracts
- `openai.CreateRequest.Files` contains reference values exactly as parsed from downstream compatibility fields.
- HTTP(S) references must be fetched by `mediafetch.Fetcher` and streamed to upstream as multipart file parts with `filename` and `Content-Type`.
- Non-HTTP references such as `asset://...` are valid for video creation passthrough and must remain text `files` fields.
- Preserve input order when writing repeated `files` fields, even when HTTP(S) downloads are prefetched concurrently.
- Enforce configured single-file and total byte limits while copying HTTP(S) response bodies into multipart file parts.
- `files` may mix `asset://...` resource-library addresses and public HTTP(S) URLs in the same video generation request; only HTTP(S) entries are fetched by the proxy.
- Video creation may remain `queued` / upstream `StatusText=待处理` for several minutes; treat it as in progress, not failure.

#### 4. Validation & Error Matrix
- Too many references -> HTTP 400 with `too many reference files`.
- Missing `model` or `prompt` -> HTTP 400 from request parsing.
- Bad, unreachable, or too-large HTTP(S) media URL -> fail creation and return upstream/proxy error.
- Non-HTTP reference such as `asset://asset-123` -> write text field and let Seedance upstream resolve it.
- `seedance-asset` unsafe image URL -> reject locally with HTTP 400 using public URL validation.

#### 5. Good/Base/Bad Cases
- Good: `files: ["https://cdn.example/ref.jpg", "asset://asset-123"]` becomes one file `files` part followed by one text `files` field in the same order.
- Good: video generation with one `asset://...` person asset and one public stage image URL returns a queued task and can later complete with `url` / `video_url`.
- Base: No `files` values still submits only text video parameters.
- Bad: Writing HTTP(S) references as text fields bypasses media fetching, byte limits, and file-part upload.

#### 6. Tests Required
- HTTP API test should inspect upstream multipart parts and assert HTTP(S) references have filenames/content bytes while `asset://...` remains a text field.
- Client test should prove HTTP(S) references are fetched, copied, and sent with content type and filename.
- Existing asset tests should continue proving `seedance-asset` calls `/resources/user/Resources` with JSON `OssPath`.

#### 7. Wrong vs Correct
Wrong:
```go
_ = writer.WriteField("files", "https://cdn.example/ref.jpg")
```

Correct:
```go
part, _ := writer.CreatePart(filePartHeader(result))
_, _ = io.Copy(part, countingReader)
```

Correct for resource-library references:
```go
_ = writer.WriteField("files", "asset://asset-123")
```

### Scenario: Jimeng Video Upstream URL Passthrough

#### 1. Scope / Trigger
- Trigger: The legacy Seedance video upstream may be down for a long maintenance window, so the proxy can switch normal video creation/query to Jimeng while keeping the legacy code path available.
- Scope: Normal video creation and query through `POST /v1/videos` and `GET /v1/videos/{task_id}` when `VIDEO_UPSTREAM_PROVIDER` resolves to `jimeng`.

#### 2. Signatures
- Config keys:
  - `VIDEO_UPSTREAM_PROVIDER`: `jimeng` by default, `legacy` to explicitly use the old Seedance video/asset upstream.
  - `JIMENG_UPSTREAM_BASE_URL`: defaults to `https://api.aizhw.cc`.
- Downstream create API: `POST /v1/videos` with JSON fields `model`, `prompt`, optional `ratio` / `aspect_ratio`, `duration` / `seconds`, `resolution`, `reference_mode`, and reference URL fields.
- Jimeng create API: `POST /v1/videos/tasks` with JSON body using `file_paths`.
- Downstream query API: `GET /v1/videos/{task_id}`.
- Jimeng query API: `GET /v1/videos/tasks/{task_id}` where the task ID is a string.
- Downstream-facing docs and product labels should call this model family `JM series`; the upstream model ID remains unchanged, for example `jimeng-video-seedance-2.0-vip`.

#### 3. Contracts
- Unknown `VIDEO_UPSTREAM_PROVIDER` values must fall back to `jimeng` and log a warning.
- `jimeng` mode must not call the legacy multipart upload path or the legacy asset-library path for normal video tasks.
- `model` is required and passed through unchanged; the proxy does not map or default model IDs.
- `prompt` is passed through unchanged; do not rewrite `@图片1` into `@1`.
- `ratio` wins over `aspect_ratio`; if both are missing, do not send a default ratio.
- `resolution` defaults to `720p` when missing.
- `duration` wins over `seconds`; if both are missing, send JSON number `4`. Strings and numbers are accepted, but the upstream receives a JSON number.
- Reference URLs are collected in this exact order: `files`, `input_reference`, `file_paths`, `filePaths`.
- Preserve duplicate reference URLs and input order. `both_frames` may intentionally use the same URL for first and last frame.
- Public HTTP(S) URLs are passed through as `file_paths`; do not download, re-host, or multipart upload them in `jimeng` mode.
- `asset://...` and `model=seedance-asset` are rejected in `jimeng` mode. They are legacy-only concepts.
- Missing `reference_mode` defaults to `omni` when references exist and `text_to_video` when no references exist.
- Supported `reference_mode` values are English only: `omni`, `first_frame`, `last_frame`, `both_frames`, `text_to_video`.
- Query responses map Jimeng status as `pending -> queued`, `processing -> in_progress`, `completed -> completed`, `failed -> failed`; unknown statuses stay `in_progress`.
- `completed` with `result_expired=true` remains `completed`, omits `url` / `video_url`, and records `metadata.jimeng.result_expired=true` plus an error detail.
- `completed` with no `result.url` and no expiry marker remains `completed`, omits `url` / `video_url`, and records `metadata.jimeng.missing_result_url=true`. Do not map this to `failed`, because that can trigger refund semantics after the upstream already charged.
- When Jimeng returns `cost` or `input_files`, preserve them under `metadata.jimeng`. Real upstream validation showed mixed image/video URL inputs are echoed as `input_files` entries with `type=image` and `type=video`.
- When NewAPI sits in front of this proxy, OpenAI Video query responses must keep the public NewAPI task ID in both `id` and `task_id`; the upstream Jimeng task ID belongs in private task data, not the downstream response.

#### 4. Validation & Error Matrix
- Missing `model` -> HTTP 400.
- Missing `prompt` -> HTTP 400.
- `model=seedance-asset` in `jimeng` mode -> HTTP 400.
- Any `asset://...` reference in `jimeng` mode -> HTTP 400.
- Non-HTTP(S), empty-host, localhost, loopback, private, or link-local URL -> HTTP 400.
- `reference_mode=omni` with 0 or more than 12 URLs -> HTTP 400.
- `reference_mode=text_to_video` with any URL -> HTTP 400.
- `reference_mode=first_frame` or `last_frame` with anything other than exactly 1 URL -> HTTP 400.
- `reference_mode=both_frames` with anything other than exactly 2 URLs -> HTTP 400.
- Jimeng non-2xx create/query responses -> return the upstream HTTP status when it is 4xx/5xx, preserving a short upstream error summary.
- Network errors, timeouts, or invalid JSON from Jimeng -> return a proxy upstream error; never fabricate a successful task.
- Jimeng query `status=failed` -> HTTP 200 task object with `status=failed`, `progress=100`, and a readable `error.message`.
- Jimeng account credit/entitlement exhaustion -> HTTP 200 task object with `status=failed`; preserve the upstream Chinese message such as `即梦账号积分或权益不足，请联系管理员处理。`.

#### 5. Good/Base/Bad Cases
- Good: `files: ["https://cdn.example/first.jpg", "https://cdn.example/first.jpg"]` with `reference_mode=both_frames` sends both URLs in order.
- Good: Request with `aspect_ratio:"4:3"` and no `ratio` sends `ratio:"4:3"`.
- Good: `jimeng-video-seedance-2.0-vip` with one public image URL and one public mp4 URL can submit successfully and later fail with a normal task object if upstream credits are insufficient.
- Good: `jimeng-video-seedance-2.0-vip` with two distinct public image URLs, `reference_mode:"omni"`, and a Chinese prompt can complete successfully; NewAPI must expose the public task ID in both `id` and `task_id`, return `status:"completed"`, and map the upstream mp4 to both `url` and `video_url`.
- Good: Mixed image/image/image/video `files` inputs should preserve order through NewAPI and Jimeng metadata; if Jimeng later fails with a busy-generation message, return an HTTP 200 task object with `status:"failed"` and the upstream message.
- Base: Text-to-video request with no files sends `reference_mode:"text_to_video"` and no `file_paths`.
- Bad: Writing Jimeng HTTP(S) references as legacy multipart `files` parts; Jimeng expects JSON `file_paths`.
- Bad: Mapping a completed task with missing result URL to `failed`, which can refund the downstream user while the upstream has charged.
- Bad: Returning the upstream Jimeng task ID as `task_id` through NewAPI's OpenAI Video query response.

#### 6. Tests Required
- Config tests for default `jimeng`, explicit `legacy`, and invalid-provider fallback.
- Request parser tests for field normalization, reference ordering, duplicate preservation, URL validation, and all `reference_mode` cardinality rules.
- HTTP API tests proving default/Jimeng create calls `/v1/videos/tasks` with `Authorization: Bearer <token>` and JSON `file_paths`.
- HTTP API tests proving `legacy` mode still calls `/seedanceapi/common/File/All` and legacy asset tests still pass.
- Query tests for string task IDs, completed result URL mapping, failed status mapping, unknown status mapping, `result_expired`, and completed-without-URL.
- NewAPI integration checks should use the current source/binary, not stale local binaries, because older binaries may not rewrite upstream `task_id` back to the public task ID in `ConvertToOpenAIVideo`.

#### 7. Wrong vs Correct
Wrong:
```go
// Jimeng mode must not fetch and stream public URLs as multipart file parts.
part, _ := writer.CreateFormFile("files", "ref.jpg")
_, _ = io.Copy(part, downloadedURLBody)
```

Correct:
```go
payload["file_paths"] = req.FilePaths
```

Wrong:
```go
if status == "completed" && resultURL == "" {
    status = "failed"
}
```

Correct:
```go
if status == "completed" && resultURL == "" {
    jimengMeta["missing_result_url"] = true
    out.Error = &openai.ErrorDetail{Code: "missing_result_url", Message: "upstream completed without result url"}
}
```

### Scenario: Seedance Asset Upstream Host

#### 1. Scope / Trigger
- Trigger: Live integration testing showed the asset library API may live on a different host than the video generation API.
- Scope: `seedance-asset` create/query calls.

#### 2. Signatures
- Config key: `ASSET_UPSTREAM_BASE_URL`.
- Default: when unset, use the asset library host `http://119.45.42.208:8620`.
- Asset create path: `POST /resources/user/Resources`.
- Asset query path: `POST /resources/user/ResourcesList`.

#### 3. Contracts
- Video generation uses `UPSTREAM_BASE_URL`.
- Asset create/query uses `ASSET_UPSTREAM_BASE_URL`.
- Do not concatenate asset paths onto the video generation host.

#### 4. Validation & Error Matrix
- Asset API called against the video host may return HTTP 404 `page not found`; this indicates host mismatch, not an invalid asset request body.
- Asset API called against the asset host with valid token and URL should return `code=0`, `success=true` on create.

#### 5. Good/Base/Bad Cases
- Good: `UPSTREAM_BASE_URL=http://119.45.252.34:8618` and `ASSET_UPSTREAM_BASE_URL=http://119.45.42.208:8620`.
- Base: local tests can set both to the same `httptest.Server`.
- Bad: hard-coding `/resources/user/Resources` against `UPSTREAM_BASE_URL`.

#### 6. Tests Required
- Asset create/query tests should fail if the video upstream server receives asset paths.

#### 7. Wrong vs Correct
Wrong:
```go
cfg.UpstreamBaseURL + "/resources/user/Resources"
```

Correct:
```go
c.assetUpstreamBaseURL() + "/resources/user/Resources"
```

### Scenario: Seedance Asset Result URI

#### 1. Scope / Trigger
- Trigger: Asset creation returns an upstream `AssetId` only after asynchronous resource approval, while video generation expects resource-library references in `asset://<asset_id>` form.
- Scope: `GET /v1/videos/{asset_req_id}` responses for `seedance-asset` tasks.

#### 2. Signatures
- Response type: `openai.VideoResponse`.
- Top-level fields: `asset_id` for the raw upstream ID, `asset_uri` for direct video-generation input.
- Metadata fields: mirror both values under `metadata.seedance.asset_id` and `metadata.seedance.asset_uri`.

#### 3. Contracts
- Preserve `asset_id` exactly as the trimmed upstream `AssetId`, for example `asset-123`.
- Derive `asset_uri` as `asset://` + `asset_id`, for example `asset://asset-123`.
- Only emit `asset_uri` when `asset_id` is non-empty.
- Downstream video generation examples should tell callers to pass `asset_uri` in `files`.
- Before querying asset status, validate task IDs strictly as `asset_req_<unix>_<random>` with a positive numeric timestamp and a non-empty lowercase-hex random suffix.
- Never query the asset upstream for short prefixes such as `asset_req_` or `asset_req_1`; short prefixes can match unrelated upstream resource names when using substring lookup.

#### 4. Validation & Error Matrix
- Found resource with non-empty `AssetId` -> `status=completed`, `asset_id`, and `asset_uri` present.
- Found resource without `AssetId` and no explicit failure -> `status=in_progress`, no `asset_uri`.
- Explicit resource failure -> `status=failed`, no `asset_uri` unless upstream also returned a non-empty `AssetId`.
- Invalid `asset_req` task ID format -> HTTP 400 before any asset upstream request.

#### 5. Good/Base/Bad Cases
- Good: response includes both `"asset_id":"asset-123"` and `"asset_uri":"asset://asset-123"`.
- Base: queued create response has no `asset_id` or `asset_uri`.
- Bad: replacing `asset_id` with `asset://asset-123` breaks callers that need the raw upstream ID.

#### 6. Tests Required
- Asset query success test must assert top-level and metadata `asset_uri`.
- Documentation must show using `asset_uri` in video generation `files`.
- HTTP asset query test must assert invalid short `asset_req` task IDs return HTTP 400 and do not call the asset upstream.

#### 7. Wrong vs Correct
Wrong:
```json
{"asset_id":"asset://asset-123"}
```

Correct:
```json
{"asset_id":"asset-123","asset_uri":"asset://asset-123"}
```

### Scenario: Seedance Asset Upstream Name Limit

#### 1. Scope / Trigger
- Trigger: Live upstream testing showed `POST /resources/user/Resources` accepts `Name` up to 50 Unicode characters and returns `code=1`, `message="操作失败！"` at 51 characters.
- Scope: `seedance-asset` create/query/delete resource-name tracing.

#### 2. Signatures
- Public task ID stays `asset_req_<unix>_<12 lowercase hex chars>` so task age can be computed without storage.
- Upstream asset `Name` uses short trace suffix `__ar_<12 lowercase hex chars>`.
- Display name is the request `prompt`.

#### 3. Contracts
- Full upstream `Name` must be at most 50 Unicode characters.
- Short suffix length is 17 characters, so display name must be at most 33 Unicode characters.
- Create must fail locally with HTTP 400 for overlong display names; do not let upstream return opaque `操作失败！`.
- Query/delete must extract the random suffix from `asset_req_<unix>_<random>` and match resources by exact suffix `__ar_<random>`, not substring.
- Strip display names by the last `__ar_` marker.

#### 4. Good/Base/Bad Cases
- Good: `真人测试__ar_abcdef123456` matches task `asset_req_1780830000_abcdef123456`.
- Good: 33-character display name plus suffix is exactly 50 characters.
- Bad: using `__asset_req_<unix>_<random>` leaves only 15 display-name characters and can hit the upstream limit.
- Bad: matching any resource name that merely contains `asset_req_...` or `ar_<random>` is unsafe.

#### 5. Tests Required
- Asset create test should assert the upstream `Name` uses `__ar_`.
- Asset create test should assert 33-character display names succeed and 34-character names return HTTP 400.
- Query/delete tests should include a misleading non-suffix resource and assert it is ignored.

### Scenario: Seedance Asset Delete Boundary

#### 1. Scope / Trigger
- Trigger: Exposing asset deletion to downstream NewAPI API-key callers while keeping `seedance-direct-proxy` stateless.
- Scope: NewAPI owns user/API-key authorization and task-history mutation; `seedance-direct-proxy` only forwards an already-authorized resource deletion to the Seedance asset upstream.

#### 2. Signatures
- Downstream NewAPI API: `POST /api/task/token/asset/delete` with `Authorization: Bearer <NewAPI API Key>`.
- Request body MVP: `{"task_id":"asset_req_xxx"}`.
- Proxy upstream-compatible forwarding API: `POST /api/task/token/asset/delete` with `Authorization: Bearer <Seedance upstream token>`.
- Seedance upstream asset API: `DELETE /resources/user/Resources` with JSON body `{"Id": <resource_id>}`.

#### 3. Contracts
- NewAPI must query by `user_id + token_id + task_id`; same-user different-token tasks are not deletable by the current API key.
- Downstream callers must not provide `resource_id` as the deletion authority. NewAPI forwards the task's upstream task ID after ownership checks; the proxy resolves the real resource ID by querying the asset list.
- Only successful, undeleted `seedance-asset` tasks may be deleted.
- Deletion does not remove task history, does not refund quota, and does not change task status to failure.
- On success, NewAPI must set both top-level `data.deleted/deleted_at` and `data.metadata.seedance.deleted/deleted_at`.
- `/api/task/token/self` remains a task-history endpoint; available-asset filtering must exclude deleted tasks.

#### 4. Validation & Error Matrix
- Missing or unknown `task_id` -> API error.
- Task belongs to another user or token -> API error as not found/no permission.
- Task status is not `SUCCESS` -> API error.
- Task is not identified as `seedance-asset` by data model, origin model, or metadata kind -> API error.
- Task data already has `deleted=true` -> API error.
- Proxy cannot find the asset resource by upstream task ID -> API error.
- Proxy delete upstream non-2xx or business failure -> API error and do not mark local task deleted.

#### 5. Good/Base/Bad Cases
- Good: Current API key deletes its own successful asset task; proxy receives `POST /api/task/token/asset/delete`, resolves the resource ID, deletes the Seedance resource, and local task data is marked deleted.
- Base: Deleted task still appears in `/api/task/token/self` for audit/history.
- Bad: Passing `asset_id`, `asset_uri`, or raw `resource_id` directly from downstream to delete without task ownership validation.

#### 6. Tests Required
- Proxy HTTP test should assert method/path/body/token for list-and-delete behavior and asset upstream host usage.
- NewAPI controller test should cover successful deletion, other-token rejection, non-asset rejection, non-success rejection, and already-deleted rejection.
- NewAPI test must verify local task data receives both top-level and metadata deleted flags.

#### 7. Wrong vs Correct
Wrong:
```http
DELETE /v1/videos/asset_req_xxx
```

Correct:
```http
POST /api/task/token/asset/delete
```

Wrong:
```json
{"resource_id":123}
```

Correct:
```json
{"task_id":"asset_req_xxx"}
```

### Public URL Host Validation

When accepting user-provided public image/media URLs, validate the parsed host before use:

- Require `http` or `https`.
- Reject empty hosts, `localhost`, `*.localhost`, loopback IPs, private IPs, link-local IPs, multicast link-local IPs, and unspecified IPs.
- Reject hosts containing `%` before `net.ParseIP`; this blocks IPv6 zone IDs such as `http://[fe80::1%25eth0]/a.jpg`, which otherwise parse as a non-IP host and can bypass link-local checks.
- Do not rely on DNS resolution for MVP lightweight validation unless the task explicitly requires SSRF/rebinding protection.

(To be filled by the team)

---

## Testing Requirements

For URL safety validation, include table-driven bad cases for every blocked host class, including an IPv6 link-local address with a zone ID (`http://[fe80::1%25eth0]/...`).

(To be filled by the team)

---

## Code Review Checklist

<!-- What reviewers should check -->

(To be filled by the team)
