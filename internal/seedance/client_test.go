package seedance

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
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

func TestCreateForwardsFilesWithoutFetching(t *testing.T) {
	var upstreamFileValue string
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
			if part.FileName() != "" {
				t.Fatalf("files part filename = %q, want text field", part.FileName())
			}
			buf, _ := io.ReadAll(part)
			upstreamFileValue = string(buf)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"请求成功","data":{"Id":88},"success":true}`))
	}))
	defer upstream.Close()

	client := Client{
		HTTPClient: &http.Client{Timeout: 2 * time.Second},
		Config: config.Config{
			UpstreamBaseURL:       upstream.URL,
			UpstreamCreateTimeout: 2 * time.Second,
		},
	}
	req := openai.CreateRequest{
		Model:         "doubao-seedance-2-0-260128-2",
		Prompt:        "test",
		Duration:      "4",
		AspectRatio:   "16:9",
		Files:         []string{"file:///not-supported.jpg"},
		GenerateAudio: "true",
		Watermark:     "false",
		Resolution:    "480p",
	}

	if _, err := client.Create(context.Background(), req, "token"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if upstreamFileValue != "file:///not-supported.jpg" {
		t.Fatalf("upstream file value = %q", upstreamFileValue)
	}
}
