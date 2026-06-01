package mediafetch

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"
)

type Fetcher struct {
	Client              *http.Client
	Timeout             time.Duration
	MaxSingleMediaBytes int64
	MaxTotalMediaBytes  int64
	PrefetchConcurrency int
}

type Result struct {
	URL           string
	Filename      string
	ContentType   string
	ContentLength int64
	Body          io.ReadCloser
	Err           error
}

func (f Fetcher) Start(ctx context.Context, rawURLs []string) ([]<-chan Result, context.CancelFunc, error) {
	if f.Client == nil {
		return nil, nil, fmt.Errorf("missing HTTP client")
	}
	if f.PrefetchConcurrency <= 0 {
		f.PrefetchConcurrency = 6
	}
	if f.PrefetchConcurrency > len(rawURLs) && len(rawURLs) > 0 {
		f.PrefetchConcurrency = len(rawURLs)
	}

	ctx, cancel := context.WithCancel(ctx)
	jobs := make(chan fetchJob)
	resultChans := make([]chan Result, len(rawURLs))
	results := make([]<-chan Result, len(rawURLs))

	for i, rawURL := range rawURLs {
		out := make(chan Result)
		resultChans[i] = out
		results[i] = out
		_ = rawURL
	}

	for range f.PrefetchConcurrency {
		go f.worker(ctx, jobs)
	}

	go func() {
		defer close(jobs)
		for i, rawURL := range rawURLs {
			job := fetchJob{URL: rawURL, Out: resultChans[i]}
			select {
			case jobs <- job:
			case <-ctx.Done():
				for _, out := range resultChans[i:] {
					close(out)
				}
				return
			}
		}
	}()

	return results, cancel, nil
}

type fetchJob struct {
	URL string
	Out chan Result
}

func (f Fetcher) worker(ctx context.Context, jobs <-chan fetchJob) {
	for job := range jobs {
		result := f.fetchOne(ctx, job.URL)
		select {
		case job.Out <- result:
		case <-ctx.Done():
			if result.Body != nil {
				_ = result.Body.Close()
			}
		}
		close(job.Out)
	}
}

func (f Fetcher) fetchOne(parent context.Context, rawURL string) Result {
	value := strings.TrimSpace(rawURL)
	parsed, err := url.Parse(value)
	if err != nil || parsed == nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return Result{URL: value, Err: fmt.Errorf("invalid media URL: only http/https is supported")}
	}

	ctx := parent
	cancel := func() {}
	if f.Timeout > 0 {
		ctx, cancel = context.WithTimeout(parent, f.Timeout)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, value, nil)
	if err != nil {
		cancel()
		return Result{URL: value, Err: err}
	}
	req.Header.Set("Accept", "image/*, video/*, audio/*, application/octet-stream")

	resp, err := f.Client.Do(req)
	if err != nil {
		cancel()
		return Result{URL: value, Err: fmt.Errorf("fetch media failed: %w", err)}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		cancel()
		return Result{URL: value, Err: fmt.Errorf("fetch media failed: status %d", resp.StatusCode)}
	}

	contentLength := resp.ContentLength
	if contentLength > 0 && f.MaxSingleMediaBytes > 0 && contentLength > f.MaxSingleMediaBytes {
		resp.Body.Close()
		cancel()
		return Result{URL: value, Err: fmt.Errorf("media too large: content length %d exceeds %d", contentLength, f.MaxSingleMediaBytes)}
	}

	contentType := normalizeContentType(resp.Header.Get("Content-Type"))
	filename := filenameFromURL(parsed, contentType)

	return Result{
		URL:           value,
		Filename:      filename,
		ContentType:   contentType,
		ContentLength: contentLength,
		Body:          cancelOnClose{ReadCloser: resp.Body, cancel: cancel},
	}
}

type cancelOnClose struct {
	io.ReadCloser
	cancel func()
}

func (c cancelOnClose) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}

type CountingReader struct {
	Reader     io.Reader
	MaxSingle  int64
	MaxTotal   int64
	SingleRead int64
	Total      *int64
	TotalMu    *sync.Mutex
}

func (r *CountingReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if n > 0 {
		r.SingleRead += int64(n)
		if r.MaxSingle > 0 && r.SingleRead > r.MaxSingle {
			return n, fmt.Errorf("media too large: exceeds single file limit %d", r.MaxSingle)
		}
		if r.Total != nil {
			if r.TotalMu != nil {
				r.TotalMu.Lock()
				*r.Total += int64(n)
				total := *r.Total
				r.TotalMu.Unlock()
				if r.MaxTotal > 0 && total > r.MaxTotal {
					return n, fmt.Errorf("media too large: exceeds total limit %d", r.MaxTotal)
				}
			} else {
				*r.Total += int64(n)
				if r.MaxTotal > 0 && *r.Total > r.MaxTotal {
					return n, fmt.Errorf("media too large: exceeds total limit %d", r.MaxTotal)
				}
			}
		}
	}
	return n, err
}

func normalizeContentType(value string) string {
	if value == "" {
		return "application/octet-stream"
	}
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil || strings.TrimSpace(mediaType) == "" {
		return "application/octet-stream"
	}
	return strings.ToLower(strings.TrimSpace(mediaType))
}

func filenameFromURL(u *url.URL, contentType string) string {
	name := path.Base(u.Path)
	if name == "." || name == "/" || strings.TrimSpace(name) == "" {
		name = "file"
	}
	name = sanitizeFilename(name)
	if path.Ext(name) == "" {
		if ext := extensionFromContentType(contentType); ext != "" {
			name += ext
		}
	}
	return name
}

func sanitizeFilename(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.ReplaceAll(value, "\n", "")
	value = strings.ReplaceAll(value, `"`, "_")
	value = strings.ReplaceAll(value, `\`, "_")
	value = strings.ReplaceAll(value, `/`, "_")
	if value == "" {
		return "file"
	}
	return value
}

func extensionFromContentType(contentType string) string {
	switch strings.ToLower(contentType) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "video/mp4":
		return ".mp4"
	case "audio/mpeg":
		return ".mp3"
	case "audio/wav", "audio/x-wav":
		return ".wav"
	default:
		return ""
	}
}
