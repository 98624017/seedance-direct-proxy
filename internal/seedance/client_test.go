package seedance

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/98624017/seedance-direct-proxy/internal/config"
	"github.com/98624017/seedance-direct-proxy/internal/openai"
)

func TestCreateReturnsWhenUpstreamConnectionFails(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	upstreamURL := "http://" + listener.Addr().String()
	_ = listener.Close()

	client := Client{
		HTTPClient: &http.Client{Timeout: 2 * time.Second},
		Config: config.Config{
			UpstreamBaseURL:       upstreamURL,
			UpstreamCreateTimeout: 2 * time.Second,
			MediaFetchTimeout:     time.Second,
		},
	}
	req := openai.CreateRequest{
		Model:         "doubao-seedance-2-0-260128-2",
		Prompt:        "test",
		Duration:      "4",
		AspectRatio:   "16:9",
		GenerateAudio: "true",
		Watermark:     "false",
		Resolution:    "480p",
	}

	done := make(chan error, 1)
	go func() {
		_, err := client.Create(context.Background(), req, "token")
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("Create() expected connection error")
		}
	case <-time.After(time.Second):
		t.Fatalf("Create() did not return after upstream connection failure")
	}
}

func TestCreateStreamsFileURLsAsMultipartFileParts(t *testing.T) {
	mediaBody := "seedance-media-body"
	media := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "image/*, video/*, audio/*, application/octet-stream" {
			t.Fatalf("media Accept = %q", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Content-Length", strconv.Itoa(len(mediaBody)))
		_, _ = w.Write([]byte(mediaBody))
	}))
	defer media.Close()

	var upstreamFilename string
	var upstreamContentType string
	var upstreamFileBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reader, err := r.MultipartReader()
		if err != nil {
			t.Fatalf("MultipartReader() error = %v", err)
		}
		for {
			part, err := reader.NextPart()
			if err != nil {
				break
			}
			if part.FormName() != "files" {
				continue
			}
			upstreamFilename = part.FileName()
			upstreamContentType = part.Header.Get("Content-Type")
			buf, _ := io.ReadAll(part)
			upstreamFileBody = string(buf)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"请求成功","data":{"Id":88},"success":true}`))
	}))
	defer upstream.Close()

	client := Client{
		HTTPClient: media.Client(),
		Config: config.Config{
			UpstreamBaseURL:          upstream.URL,
			UpstreamCreateTimeout:    2 * time.Second,
			MediaFetchTimeout:        time.Second,
			MediaPrefetchConcurrency: 1,
			MaxSingleMediaBytes:      1024,
			MaxTotalMediaBytes:       1024,
		},
	}
	req := openai.CreateRequest{
		Model:         "doubao-seedance-2-0-260128-2",
		Prompt:        "test",
		Duration:      "4",
		AspectRatio:   "16:9",
		Files:         []string{media.URL + "/ref.jpg"},
		GenerateAudio: "true",
		Watermark:     "false",
		Resolution:    "480p",
	}

	if _, err := client.Create(context.Background(), req, "token"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if upstreamFilename != "ref.jpg" {
		t.Fatalf("upstream filename = %q", upstreamFilename)
	}
	if upstreamContentType != "image/jpeg" {
		t.Fatalf("upstream content type = %q", upstreamContentType)
	}
	if upstreamFileBody != mediaBody {
		t.Fatalf("upstream file body = %q", upstreamFileBody)
	}
}

func TestQueryAssetFallsBackToConfiguredTokens(t *testing.T) {
	var calls []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Header.Get("token"))
		if len(calls) == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"bad token"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"请求成功","data":{"data":[{"Id":11,"Name":"林春芽__ar_abcdef123456","AssetId":"asset-11","Status":1,"StatusText":"处理成功"}],"total":1},"success":true}`))
	}))
	defer upstream.Close()

	client := Client{
		HTTPClient: upstream.Client(),
		Config: config.Config{
			AssetUpstreamBaseURL: upstream.URL,
			AssetListBasePages:   1,
			AssetUpstreamTokens:  []string{"token-b", "token-c"},
			UpstreamQueryTimeout: time.Second,
		},
	}

	resp, err := client.QueryAsset(context.Background(), "asset_req_1700000000_abcdef123456", "token-a")
	if err != nil {
		t.Fatalf("QueryAsset error = %v", err)
	}
	if resp.AssetID != "asset-11" {
		t.Fatalf("AssetID = %q", resp.AssetID)
	}
	if len(calls) != 2 || calls[0] != "token-a" || calls[1] != "token-b" {
		t.Fatalf("unexpected token calls: %#v", calls)
	}
}

func TestQueryAssetReturnsLastFallbackErrorWhenAllTokensFail(t *testing.T) {
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"bad token"}`))
	}))
	defer upstream.Close()

	client := Client{
		HTTPClient: upstream.Client(),
		Config: config.Config{
			AssetUpstreamBaseURL: upstream.URL,
			AssetListBasePages:   1,
			AssetUpstreamTokens:  []string{"token-b"},
			UpstreamQueryTimeout: time.Second,
		},
	}

	_, err := client.QueryAsset(context.Background(), "asset_req_1700000000_abcdef123456", "token-a")
	if err == nil {
		t.Fatalf("expected error")
	}
	var upstreamErr UpstreamError
	if !errors.As(err, &upstreamErr) || upstreamErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("unexpected calls: %d", calls)
	}
}

func TestDeleteAssetFallsBackToConfiguredTokens(t *testing.T) {
	var calls []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.Header.Get("token"))
		switch len(calls) {
		case 1:
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"forbidden"}`))
		case 2:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"message":"请求成功","data":{"data":[{"Id":11,"Name":"林春芽__ar_abcdef123456","AssetId":"asset-11","Status":1,"StatusText":"处理成功"}],"total":1},"success":true}`))
		case 3:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"message":"请求成功","data":{},"success":true}`))
		default:
			t.Fatalf("unexpected call %d: %s %s", len(calls), r.Method, r.Header.Get("token"))
		}
	}))
	defer upstream.Close()

	client := Client{
		HTTPClient: upstream.Client(),
		Config: config.Config{
			AssetUpstreamBaseURL: upstream.URL,
			AssetListBasePages:   1,
			AssetUpstreamTokens:  []string{"token-b"},
			UpstreamQueryTimeout: time.Second,
		},
	}

	deleted, err := client.DeleteAssetByTaskID(context.Background(), "asset_req_1700000000_abcdef123456", "token-a")
	if err != nil {
		t.Fatalf("DeleteAssetByTaskID error = %v", err)
	}
	if deleted.ResourceID != 11 {
		t.Fatalf("ResourceID = %d", deleted.ResourceID)
	}
	if len(calls) != 3 || calls[0] != "POST token-a" || calls[1] != "POST token-b" || calls[2] != "DELETE token-b" {
		t.Fatalf("unexpected call sequence: %#v", calls)
	}
}
