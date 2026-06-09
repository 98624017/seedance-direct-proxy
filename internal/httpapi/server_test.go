package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/98624017/seedance-direct-proxy/internal/config"
	"github.com/98624017/seedance-direct-proxy/internal/seedance"
)

func TestCreateForwardsFileURLsToSeedanceAndPreservesOrder(t *testing.T) {
	var upstreamModel string
	var upstreamFiles []string
	var upstreamToken string

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/seedanceapi/common/File/All" {
			t.Fatalf("unexpected upstream path %s", r.URL.Path)
		}
		upstreamToken = r.Header.Get("token")
		reader, err := r.MultipartReader()
		if err != nil {
			t.Fatalf("MultipartReader() error = %v", err)
		}
		for {
			part, err := reader.NextPart()
			if err != nil {
				break
			}
			switch part.FormName() {
			case "model":
				buf, _ := io.ReadAll(part)
				upstreamModel = string(buf)
			case "files":
				if part.FileName() != "" {
					t.Fatalf("files part filename = %q, want empty text field", part.FileName())
				}
				buf, _ := io.ReadAll(part)
				upstreamFiles = append(upstreamFiles, string(buf))
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"请求成功","data":{"Id":88},"success":true}`))
	}))
	defer upstream.Close()

	api := Server{
		Config: config.Config{
			Port:                     "0",
			UpstreamBaseURL:          upstream.URL,
			MaxReferenceFiles:        12,
			MaxSingleMediaBytes:      50 << 20,
			MaxTotalMediaBytes:       200 << 20,
			MediaPrefetchConcurrency: 6,
			MediaFetchTimeout:        75 * time.Second,
			UpstreamCreateTimeout:    180 * time.Second,
			UpstreamQueryTimeout:     30 * time.Second,
		},
		Client: seedance.Client{
			HTTPClient: http.DefaultClient,
			Config: config.Config{
				UpstreamBaseURL:          upstream.URL,
				MaxReferenceFiles:        12,
				MaxSingleMediaBytes:      50 << 20,
				MaxTotalMediaBytes:       200 << 20,
				MediaPrefetchConcurrency: 6,
				MediaFetchTimeout:        75 * time.Second,
				UpstreamCreateTimeout:    180 * time.Second,
				UpstreamQueryTimeout:     30 * time.Second,
			},
			Now: func() time.Time { return time.Unix(123, 0) },
		},
	}

	body := `{
		"model":"veofast",
		"prompt":"test",
		"duration":"4",
		"aspect_ratio":"16:9",
		"files":["https://asset.test/1.jpg","asset://asset-2","https://asset.test/3.jpg"]
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer http://ignored.example|seedance-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if upstreamToken != "seedance-token" {
		t.Fatalf("upstream token = %q", upstreamToken)
	}
	if upstreamModel != "veofast" {
		t.Fatalf("upstream model = %q", upstreamModel)
	}
	wantFiles := []string{"https://asset.test/1.jpg", "asset://asset-2", "https://asset.test/3.jpg"}
	if len(upstreamFiles) != len(wantFiles) {
		t.Fatalf("files len = %d, want %d: %#v", len(upstreamFiles), len(wantFiles), upstreamFiles)
	}
	for i := range wantFiles {
		if upstreamFiles[i] != wantFiles[i] {
			t.Fatalf("file[%d] = %q, want %q", i, upstreamFiles[i], wantFiles[i])
		}
	}

	var parsed map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("response JSON error = %v", err)
	}
	if parsed["id"] != "88" || parsed["task_id"] != "88" || parsed["model"] != "veofast" {
		t.Fatalf("unexpected response: %#v", parsed)
	}
}

func TestRootSupportsNewAPIProbe(t *testing.T) {
	api := Server{}

	headReq := httptest.NewRequest(http.MethodHead, "/", nil)
	headRec := httptest.NewRecorder()
	api.Handler().ServeHTTP(headRec, headReq)
	if headRec.Code != http.StatusOK {
		t.Fatalf("HEAD / status = %d body=%s", headRec.Code, headRec.Body.String())
	}
	if headRec.Body.Len() != 0 {
		t.Fatalf("HEAD / body length = %d, want 0", headRec.Body.Len())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/", nil)
	getRec := httptest.NewRecorder()
	api.Handler().ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK || strings.TrimSpace(getRec.Body.String()) != "ok" {
		t.Fatalf("GET / status = %d body=%s", getRec.Code, getRec.Body.String())
	}
}

func TestQueryMapsSeedanceStatusAndVideoURL(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/seedanceapi/user/DataIndex" {
			t.Fatalf("unexpected upstream path %s", r.URL.Path)
		}
		if r.Header.Get("token") != "seedance-token" {
			t.Fatalf("token = %q", r.Header.Get("token"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"code":0,
			"message":"请求成功",
			"data":{
				"Id":88,
				"CreatedAt":"2026-05-20T20:50:58+08:00",
				"UpdatedAt":"2026-05-20T20:55:58+08:00",
				"Status":2,
				"StatusText":"已完成",
				"Message":"",
				"VideoUrl":"https://cdn.test/video.mp4",
				"UseToken":12,
				"DeductToken":1,
				"UseDuration":4
			},
			"success":true
		}`))
	}))
	defer upstream.Close()

	cfg := config.Config{UpstreamBaseURL: upstream.URL, UpstreamQueryTimeout: 30 * time.Second}
	api := Server{Config: cfg, Client: seedance.Client{HTTPClient: http.DefaultClient, Config: cfg}}
	req := httptest.NewRequest(http.MethodGet, "/v1/videos/88", nil)
	req.Header.Set("Authorization", "Bearer seedance-token")
	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var parsed map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("response JSON error = %v", err)
	}
	if parsed["status"] != "completed" || parsed["url"] != "https://cdn.test/video.mp4" || parsed["video_url"] != "https://cdn.test/video.mp4" {
		t.Fatalf("unexpected response: %#v", parsed)
	}
}

func TestUpstreamBusinessFailureReturnsBadGateway(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/seedanceapi/common/File/All":
			_, _ = w.Write([]byte(`{"code":1003,"message":"业务失败","data":{"Id":0},"success":false}`))
		case "/seedanceapi/user/DataIndex":
			_, _ = w.Write([]byte(`{"code":1003,"message":"查询失败","data":{},"success":false}`))
		default:
			t.Fatalf("unexpected upstream path %s", r.URL.Path)
		}
	}))
	defer upstream.Close()

	cfg := config.Config{
		UpstreamBaseURL:       upstream.URL,
		UpstreamCreateTimeout: 30 * time.Second,
		UpstreamQueryTimeout:  30 * time.Second,
	}
	api := Server{Config: cfg, Client: seedance.Client{HTTPClient: http.DefaultClient, Config: cfg}}

	createReq := httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(`{"model":"m","prompt":"p"}`))
	createReq.Header.Set("Authorization", "Bearer token")
	createRec := httptest.NewRecorder()
	api.Handler().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusBadGateway {
		t.Fatalf("create status = %d body=%s", createRec.Code, createRec.Body.String())
	}

	queryReq := httptest.NewRequest(http.MethodGet, "/v1/videos/88", nil)
	queryReq.Header.Set("Authorization", "Bearer token")
	queryRec := httptest.NewRecorder()
	api.Handler().ServeHTTP(queryRec, queryReq)
	if queryRec.Code != http.StatusBadGateway {
		t.Fatalf("query status = %d body=%s", queryRec.Code, queryRec.Body.String())
	}
}

func TestCreateRejectsMissingAuthAndTooManyFiles(t *testing.T) {
	cfg := config.Config{MaxReferenceFiles: 1, UpstreamBaseURL: "http://upstream.test"}
	api := Server{Config: cfg, Client: seedance.Client{HTTPClient: http.DefaultClient, Config: cfg}}

	req := httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(`{"model":"m","prompt":"p"}`))
	rec := httptest.NewRecorder()
	api.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(`{"model":"m","prompt":"p","files":["https://a.test/1.jpg","https://a.test/2.jpg"]}`))
	req.Header.Set("Authorization", "Bearer token")
	rec = httptest.NewRecorder()
	api.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCreateAssetUsesAssetUpstreamResourcesAPI(t *testing.T) {
	var upstreamToken string
	var upstreamPayload map[string]any
	videoUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("asset create should not call video upstream path %s", r.URL.Path)
	}))
	defer videoUpstream.Close()
	assetUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/resources/user/Resources" {
			t.Fatalf("unexpected upstream path %s", r.URL.Path)
		}
		upstreamToken = r.Header.Get("token")
		if err := json.NewDecoder(r.Body).Decode(&upstreamPayload); err != nil {
			t.Fatalf("decode upstream payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"请求成功","data":{},"success":true}`))
	}))
	defer assetUpstream.Close()

	cfg := config.Config{
		UpstreamBaseURL:       videoUpstream.URL,
		AssetUpstreamBaseURL:  assetUpstream.URL,
		UpstreamCreateTimeout: 30 * time.Second,
	}
	api := Server{
		Config: cfg,
		Client: seedance.Client{
			HTTPClient: http.DefaultClient,
			Config:     cfg,
			Now:        func() time.Time { return time.Unix(1780830000, 0) },
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(`{
		"model":"seedance-asset",
		"prompt":"林春芽",
		"input_reference":"https://asset.test/person.jpg",
		"images":["https://asset.test/ignored.jpg"]
	}`))
	req.Header.Set("Authorization", "Bearer seedance-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if upstreamToken != "seedance-token" {
		t.Fatalf("upstream token = %q", upstreamToken)
	}
	if upstreamPayload["OssPath"] != "https://asset.test/person.jpg" {
		t.Fatalf("OssPath = %#v", upstreamPayload["OssPath"])
	}
	name, _ := upstreamPayload["Name"].(string)
	if !strings.HasPrefix(name, "林春芽__ar_") {
		t.Fatalf("Name = %q", name)
	}
	if len([]rune(name)) != len([]rune("林春芽"))+len("__ar_")+12 {
		t.Fatalf("Name length = %d, Name = %q", len([]rune(name)), name)
	}

	var parsed map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("response JSON error = %v", err)
	}
	if parsed["status"] != "queued" || parsed["object"] != "video" || parsed["model"] != "seedance-asset" {
		t.Fatalf("unexpected response: %#v", parsed)
	}
	metadata := parsed["metadata"].(map[string]any)["seedance"].(map[string]any)
	if metadata["ignored_image_count"].(float64) != 1 {
		t.Fatalf("metadata = %#v", metadata)
	}
}

func TestCreateAssetRejectsNameThatWouldExceedUpstreamLimit(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("too long asset name should not call upstream path %s", r.URL.Path)
	}))
	defer upstream.Close()

	cfg := config.Config{
		UpstreamBaseURL:       upstream.URL,
		AssetUpstreamBaseURL:  upstream.URL,
		UpstreamCreateTimeout: 30 * time.Second,
	}
	api := Server{
		Config: cfg,
		Client: seedance.Client{
			HTTPClient: http.DefaultClient,
			Config:     cfg,
			Now:        func() time.Time { return time.Unix(1780830000, 0) },
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(`{
		"model":"seedance-asset",
		"prompt":"一一一一一一一一一一一一一一一一一一一一一一一一一一一一一一一一一一",
		"input_reference":"https://asset.test/person.jpg"
	}`))
	req.Header.Set("Authorization", "Bearer seedance-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "asset name is too long, max 33 characters") {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestCreateAssetAllowsMaximumDisplayNameLength(t *testing.T) {
	var upstreamPayload map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&upstreamPayload); err != nil {
			t.Fatalf("decode upstream payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"请求成功","data":{},"success":true}`))
	}))
	defer upstream.Close()

	cfg := config.Config{
		UpstreamBaseURL:       upstream.URL,
		AssetUpstreamBaseURL:  upstream.URL,
		UpstreamCreateTimeout: 30 * time.Second,
	}
	api := Server{
		Config: cfg,
		Client: seedance.Client{
			HTTPClient: http.DefaultClient,
			Config:     cfg,
			Now:        func() time.Time { return time.Unix(1780830000, 0) },
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(`{
		"model":"seedance-asset",
		"prompt":"一一一一一一一一一一一一一一一一一一一一一一一一一一一一一一一一一",
		"input_reference":"https://asset.test/person.jpg"
	}`))
	req.Header.Set("Authorization", "Bearer seedance-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	name, _ := upstreamPayload["Name"].(string)
	if got := len([]rune(name)); got != 50 {
		t.Fatalf("upstream name length = %d, name=%q", got, name)
	}
}

func TestQueryAssetReturnsAssetID(t *testing.T) {
	var pages []float64
	videoUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("asset query should not call video upstream path %s", r.URL.Path)
	}))
	defer videoUpstream.Close()
	assetUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/resources/user/ResourcesList" {
			t.Fatalf("unexpected upstream path %s", r.URL.Path)
		}
		var payload map[string]float64
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		pages = append(pages, payload["Page"])
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"code":0,
			"message":"请求成功",
			"data":{
				"data":[{
					"Id":11,
					"CreatedAt":"2026-06-03T21:55:05+08:00",
					"UpdatedAt":"2026-06-03T21:56:05+08:00",
					"Name":"林春芽__ar_abcdef123456",
					"OssPath":"https://asset.test/person.jpg",
					"Status":1,
					"StatusText":"处理成功",
					"Message":"",
					"AssetId":"asset-123"
				}],
				"total":1
			},
			"success":true
		}`))
	}))
	defer assetUpstream.Close()

	cfg := config.Config{
		UpstreamBaseURL:      videoUpstream.URL,
		AssetUpstreamBaseURL: assetUpstream.URL,
		UpstreamQueryTimeout: 30 * time.Second,
		AssetListBasePages:   10,
		AssetListMediumPages: 20,
		AssetListMaxPages:    50,
	}
	api := Server{Config: cfg, Client: seedance.Client{HTTPClient: http.DefaultClient, Config: cfg}}
	req := httptest.NewRequest(http.MethodGet, "/v1/videos/asset_req_1780830000_abcdef123456", nil)
	req.Header.Set("Authorization", "Bearer seedance-token")
	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if len(pages) != 1 || pages[0] != 1 {
		t.Fatalf("pages = %#v", pages)
	}
	var parsed map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("response JSON error = %v", err)
	}
	if parsed["status"] != "completed" || parsed["asset_id"] != "asset-123" || parsed["asset_uri"] != "asset://asset-123" {
		t.Fatalf("unexpected response: %#v", parsed)
	}
	metadata := parsed["metadata"].(map[string]any)["seedance"].(map[string]any)
	if metadata["asset_id"] != "asset-123" || metadata["asset_uri"] != "asset://asset-123" || metadata["name"] != "林春芽" {
		t.Fatalf("metadata = %#v", metadata)
	}
}

func TestQueryAssetMatchesOnlyTraceSuffix(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"code":0,
			"message":"请求成功",
			"data":{
				"data":[{
					"Id":99,
					"Name":"错误命中 ar_abcdef123456 但不是追踪后缀",
					"AssetId":"asset-wrong"
				},{
					"Id":123,
					"Name":"林春芽__ar_abcdef123456",
					"AssetId":"asset-123"
				}],
				"total":2
			},
			"success":true
		}`))
	}))
	defer upstream.Close()

	cfg := config.Config{
		UpstreamBaseURL:      upstream.URL,
		AssetUpstreamBaseURL: upstream.URL,
		UpstreamQueryTimeout: 30 * time.Second,
		AssetListBasePages:   10,
	}
	api := Server{Config: cfg, Client: seedance.Client{HTTPClient: http.DefaultClient, Config: cfg}}
	req := httptest.NewRequest(http.MethodGet, "/v1/videos/asset_req_1780830000_abcdef123456", nil)
	req.Header.Set("Authorization", "Bearer seedance-token")
	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var parsed map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("response JSON error = %v", err)
	}
	if parsed["asset_id"] != "asset-123" {
		t.Fatalf("unexpected response: %#v", parsed)
	}
	metadata := parsed["metadata"].(map[string]any)["seedance"].(map[string]any)
	if metadata["resource_id"] != float64(123) {
		t.Fatalf("metadata = %#v", metadata)
	}
}

func TestQueryAssetScansMorePagesForOlderTask(t *testing.T) {
	var pages []float64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]float64
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		pages = append(pages, payload["Page"])
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"请求成功","data":{"data":[],"total":0},"success":true}`))
	}))
	defer upstream.Close()

	cfg := config.Config{
		UpstreamBaseURL:      upstream.URL,
		UpstreamQueryTimeout: 30 * time.Second,
		AssetListBasePages:   10,
		AssetListMediumPages: 20,
		AssetListMaxPages:    50,
	}
	api := Server{
		Config: cfg,
		Client: seedance.Client{
			HTTPClient: http.DefaultClient,
			Config:     cfg,
			Now:        func() time.Time { return time.Unix(1780830000, 0).Add(30 * time.Minute) },
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/videos/asset_req_1780830000_abcdef123456", nil)
	req.Header.Set("Authorization", "Bearer seedance-token")
	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if len(pages) != 20 {
		t.Fatalf("pages scanned = %d, want 20", len(pages))
	}
	var parsed map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("response JSON error = %v", err)
	}
	if parsed["status"] != "in_progress" {
		t.Fatalf("unexpected response: %#v", parsed)
	}
}

func TestQueryAssetRejectsInvalidAssetTaskIDBeforeUpstream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("invalid asset task id should not call upstream path %s", r.URL.Path)
	}))
	defer upstream.Close()

	cfg := config.Config{
		UpstreamBaseURL:      upstream.URL,
		AssetUpstreamBaseURL: upstream.URL,
		UpstreamQueryTimeout: 30 * time.Second,
	}
	api := Server{Config: cfg, Client: seedance.Client{HTTPClient: http.DefaultClient, Config: cfg}}

	for _, taskID := range []string{"asset_req_", "asset_req_1", "asset_req_1780830000_", "asset_req_1780830000_bad-suffix"} {
		req := httptest.NewRequest(http.MethodGet, "/v1/videos/"+taskID, nil)
		req.Header.Set("Authorization", "Bearer seedance-token")
		rec := httptest.NewRecorder()

		api.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d body=%s", taskID, rec.Code, rec.Body.String())
		}
	}
}

func TestTokenAssetDeleteFindsResourceAndDeletesByTaskID(t *testing.T) {
	var upstreamToken []string
	var deletePayload map[string]float64
	var listCalled bool
	videoUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("asset delete should not call video upstream path %s", r.URL.Path)
	}))
	defer videoUpstream.Close()
	assetUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamToken = append(upstreamToken, r.Header.Get("token"))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/resources/user/ResourcesList":
			if r.Method != http.MethodPost {
				t.Fatalf("list method = %s, want POST", r.Method)
			}
			listCalled = true
			_, _ = w.Write([]byte(`{
				"code":0,
				"message":"请求成功",
				"data":{
					"data":[{
						"Id":99,
						"Name":"错误命中 ar_abcdef123456 但不是追踪后缀",
						"AssetId":"asset-wrong"
					},{
						"Id":123,
						"Name":"林春芽__ar_abcdef123456",
						"AssetId":"asset-123"
					}],
					"total":1
				},
				"success":true
			}`))
		case "/resources/user/Resources":
			if r.Method != http.MethodDelete {
				t.Fatalf("delete method = %s, want DELETE", r.Method)
			}
			if err := json.NewDecoder(r.Body).Decode(&deletePayload); err != nil {
				t.Fatalf("decode delete payload: %v", err)
			}
			_, _ = w.Write([]byte(`{"code":0,"message":"请求成功","data":{},"success":true}`))
		default:
			t.Fatalf("unexpected upstream path %s", r.URL.Path)
		}
	}))
	defer assetUpstream.Close()

	cfg := config.Config{
		UpstreamBaseURL:      videoUpstream.URL,
		AssetUpstreamBaseURL: assetUpstream.URL,
		UpstreamQueryTimeout: 30 * time.Second,
		AssetListBasePages:   10,
	}
	api := Server{
		Config: cfg,
		Client: seedance.Client{HTTPClient: http.DefaultClient, Config: cfg},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/task/token/asset/delete", strings.NewReader(`{"task_id":"asset_req_1780830000_abcdef123456"}`))
	req.Header.Set("Authorization", "Bearer http://video.test|seedance-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !listCalled {
		t.Fatalf("expected ResourcesList to be called")
	}
	if len(upstreamToken) != 2 || upstreamToken[0] != "seedance-token" || upstreamToken[1] != "seedance-token" {
		t.Fatalf("upstream tokens = %#v", upstreamToken)
	}
	if deletePayload["Id"] != 123 {
		t.Fatalf("delete payload = %#v", deletePayload)
	}

	var parsed map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("response JSON error = %v", err)
	}
	if parsed["success"] != true {
		t.Fatalf("unexpected response: %#v", parsed)
	}
	data := parsed["data"].(map[string]any)
	if data["task_id"] != "asset_req_1780830000_abcdef123456" || data["deleted"] != true || data["resource_id"] != float64(123) {
		t.Fatalf("data = %#v", data)
	}
	if data["deleted_at"] == nil || data["deleted_at"].(float64) <= 0 {
		t.Fatalf("deleted_at missing in data = %#v", data)
	}
	if data["asset_id"] != "asset-123" || data["asset_uri"] != "asset://asset-123" {
		t.Fatalf("data = %#v", data)
	}
}

func TestTokenAssetDeleteRejectsInvalidTaskIDBeforeUpstream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("invalid asset task id should not call upstream path %s", r.URL.Path)
	}))
	defer upstream.Close()

	cfg := config.Config{
		UpstreamBaseURL:      upstream.URL,
		AssetUpstreamBaseURL: upstream.URL,
	}
	api := Server{Config: cfg, Client: seedance.Client{HTTPClient: http.DefaultClient, Config: cfg}}

	req := httptest.NewRequest(http.MethodPost, "/api/task/token/asset/delete", strings.NewReader(`{"task_id":"asset_req_1"}`))
	req.Header.Set("Authorization", "Bearer seedance-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	api.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}
