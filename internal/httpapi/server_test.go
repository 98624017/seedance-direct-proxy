package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/98624017/seedance-direct-proxy/internal/config"
	"github.com/98624017/seedance-direct-proxy/internal/seedance"
)

func TestCreateStreamsMediaToSeedanceAndPreservesOrder(t *testing.T) {
	var upstreamModel string
	var upstreamFiles []string
	var upstreamToken string
	var maxInFlight int
	var inFlight int
	var mu sync.Mutex

	assetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		inFlight++
		if inFlight > maxInFlight {
			maxInFlight = inFlight
		}
		if maxInFlight == 3 {
			mu.Unlock()
		} else {
			mu.Unlock()
			time.Sleep(20 * time.Millisecond)
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("jpeg-" + strings.TrimPrefix(r.URL.Path, "/")))
		mu.Lock()
		inFlight--
		mu.Unlock()
	}))
	defer assetServer.Close()

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
				upstreamFiles = append(upstreamFiles, part.FileName())
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
		"files":["` + assetServer.URL + `/1.jpg","` + assetServer.URL + `/2.jpg","` + assetServer.URL + `/3.jpg"]
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
	wantFiles := []string{"1.jpg", "2.jpg", "3.jpg"}
	for i := range wantFiles {
		if upstreamFiles[i] != wantFiles[i] {
			t.Fatalf("file[%d] = %q, want %q", i, upstreamFiles[i], wantFiles[i])
		}
	}
	if maxInFlight < 2 {
		t.Fatalf("expected concurrent media fetches, maxInFlight=%d", maxInFlight)
	}

	var parsed map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("response JSON error = %v", err)
	}
	if parsed["id"] != "88" || parsed["task_id"] != "88" || parsed["model"] != "veofast" {
		t.Fatalf("unexpected response: %#v", parsed)
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
