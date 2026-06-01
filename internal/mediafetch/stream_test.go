package mediafetch

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestFetcherDoesNotCancelBodyBeforeConsumerReads(t *testing.T) {
	body := []byte("seedance-media-body")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer server.Close()

	fetcher := Fetcher{
		Client:              server.Client(),
		Timeout:             time.Second,
		MaxSingleMediaBytes: 1024,
		MaxTotalMediaBytes:  1024,
		PrefetchConcurrency: 1,
	}
	results, cancel, err := fetcher.Start(t.Context(), []string{server.URL + "/image.jpg"})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer cancel()

	result := <-results[0]
	if result.Err != nil {
		t.Fatalf("fetch result error = %v", result.Err)
	}
	defer result.Body.Close()

	got, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(got) != string(body) {
		t.Fatalf("body = %q, want %q", got, body)
	}
}

func TestFetcherClosesUnconsumedBodyOnCancel(t *testing.T) {
	var closed atomic.Bool
	responseStarted := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		close(responseStarted)
		<-r.Context().Done()
		closed.Store(true)
	}))
	defer server.Close()

	fetcher := Fetcher{
		Client:              server.Client(),
		Timeout:             time.Second,
		MaxSingleMediaBytes: 1024,
		MaxTotalMediaBytes:  1024,
		PrefetchConcurrency: 1,
	}
	results, cancel, err := fetcher.Start(t.Context(), []string{server.URL + "/unconsumed.jpg"})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	select {
	case <-responseStarted:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for media response to start")
	}

	cancel()

	deadline := time.After(time.Second)
	for !closed.Load() {
		select {
		case <-deadline:
			t.Fatalf("unconsumed response body was not closed after cancel")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	if _, ok := <-results[0]; ok {
		t.Fatalf("result channel should be closed after cancel")
	}
}

func TestFetcherOrderedResultChannelsDoNotBlockLaterJobs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(r.URL.Path))
	}))
	defer server.Close()

	fetcher := Fetcher{
		Client:              server.Client(),
		Timeout:             time.Second,
		MaxSingleMediaBytes: 1024,
		MaxTotalMediaBytes:  1024,
		PrefetchConcurrency: 2,
	}
	results, cancel, err := fetcher.Start(t.Context(), []string{
		server.URL + "/1.jpg",
		server.URL + "/2.jpg",
		server.URL + "/3.jpg",
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer cancel()

	for i, ch := range results {
		select {
		case result, ok := <-ch:
			if !ok {
				t.Fatalf("result channel %d closed unexpectedly", i)
			}
			if result.Err != nil {
				t.Fatalf("result %d error = %v", i, result.Err)
			}
			_ = result.Body.Close()
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for result %d", i)
		}
	}
}
