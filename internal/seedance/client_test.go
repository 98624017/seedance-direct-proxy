package seedance

import (
	"context"
	"net"
	"net/http"
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

func TestCreateReturnsMediaFetchError(t *testing.T) {
	client := Client{
		HTTPClient: &http.Client{Timeout: 2 * time.Second},
		Config: config.Config{
			UpstreamBaseURL:       "http://127.0.0.1:1",
			UpstreamCreateTimeout: 2 * time.Second,
			MediaFetchTimeout:     time.Second,
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

	_, err := client.Create(context.Background(), req, "token")
	if err == nil {
		t.Fatalf("Create() expected media URL error")
	}
}
