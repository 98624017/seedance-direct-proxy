package seedance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/98624017/seedance-direct-proxy/internal/config"
	"github.com/98624017/seedance-direct-proxy/internal/mediafetch"
	"github.com/98624017/seedance-direct-proxy/internal/openai"
)

type Client struct {
	HTTPClient *http.Client
	Config     config.Config
	Now        func() time.Time
}

type UpstreamError struct {
	StatusCode int
	Message    string
	Body       string
}

func (e UpstreamError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Body != "" {
		return e.Body
	}
	return fmt.Sprintf("upstream returned status %d", e.StatusCode)
}

func (c Client) Create(ctx context.Context, req openai.CreateRequest, token string) (openai.VideoResponse, error) {
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	cfg := c.Config
	ctx, cancel := contextWithOptionalTimeout(ctx, cfg.UpstreamCreateTimeout)
	defer cancel()

	body, contentType, pipeErrs := c.buildMultipartStream(ctx, req)
	defer cleanupMultipartStream(body, pipeErrs, cancel)

	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.UpstreamBaseURL+"/seedanceapi/common/File/All", body)
	if err != nil {
		return openai.VideoResponse{}, err
	}
	upstreamReq.Header.Set("Content-Type", contentType)
	upstreamReq.Header.Set("token", token)

	resp, err := client.Do(upstreamReq)
	if err != nil {
		cancel()
		_ = body.Close()
		if pipeErr := waitPipeError(pipeErrs); pipeErr != nil && !isExpectedPipeClose(pipeErr) {
			return openai.VideoResponse{}, pipeErr
		}
		return openai.VideoResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return openai.VideoResponse{}, readUpstreamError(resp)
	}

	var out CreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return openai.VideoResponse{}, fmt.Errorf("decode seedance create response: %w", err)
	}
	if out.Code != 0 || !out.Success || out.Data.ID == 0 {
		return openai.VideoResponse{}, UpstreamError{StatusCode: resp.StatusCode, Message: out.Message}
	}

	now := time.Now()
	if c.Now != nil {
		now = c.Now()
	}
	return NormalizeCreate(out, req.Model, now), nil
}

func (c Client) Query(ctx context.Context, taskID int64, token string) (openai.VideoResponse, error) {
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	cfg := c.Config
	ctx, cancel := contextWithOptionalTimeout(ctx, cfg.UpstreamQueryTimeout)
	defer cancel()

	payload := map[string]int64{"Id": taskID}
	body, _ := json.Marshal(payload)
	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.UpstreamBaseURL+"/seedanceapi/user/DataIndex", bytes.NewReader(body))
	if err != nil {
		return openai.VideoResponse{}, err
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("token", token)

	resp, err := client.Do(upstreamReq)
	if err != nil {
		return openai.VideoResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return openai.VideoResponse{}, readUpstreamError(resp)
	}

	var out QueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return openai.VideoResponse{}, fmt.Errorf("decode seedance query response: %w", err)
	}
	if out.Code != 0 || !out.Success {
		return openai.VideoResponse{}, UpstreamError{StatusCode: resp.StatusCode, Message: out.Message}
	}
	return NormalizeQuery(out, ""), nil
}

func (c Client) buildMultipartStream(ctx context.Context, req openai.CreateRequest) (io.ReadCloser, string, <-chan error) {
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)
	errs := make(chan error, 1)

	go func() {
		err := c.writeMultipart(ctx, writer, req)
		closeErr := writer.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			_ = pw.CloseWithError(err)
			errs <- err
			close(errs)
			return
		}
		_ = pw.Close()
		close(errs)
	}()

	return pr, writer.FormDataContentType(), errs
}

func cleanupMultipartStream(body io.Closer, pipeErrs <-chan error, cancel context.CancelFunc) {
	cancel()
	_ = body.Close()
	_ = waitPipeError(pipeErrs)
}

func contextWithOptionalTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout > 0 {
		return context.WithTimeout(ctx, timeout)
	}
	return context.WithCancel(ctx)
}

func (c Client) writeMultipart(ctx context.Context, writer *multipart.Writer, req openai.CreateRequest) error {
	textFields := [][2]string{
		{"model", req.Model},
		{"prompt", req.Prompt},
		{"duration", req.Duration},
		{"aspect_ratio", req.AspectRatio},
		{"generate_audio", req.GenerateAudio},
		{"watermark", req.Watermark},
		{"resolution", req.Resolution},
	}
	for _, field := range textFields {
		if err := writer.WriteField(field[0], field[1]); err != nil {
			return err
		}
	}
	if len(req.Files) == 0 {
		return nil
	}

	fetcher := mediafetch.Fetcher{
		Client:              c.httpClient(),
		Timeout:             c.Config.MediaFetchTimeout,
		MaxSingleMediaBytes: c.Config.MaxSingleMediaBytes,
		MaxTotalMediaBytes:  c.Config.MaxTotalMediaBytes,
		PrefetchConcurrency: c.Config.MediaPrefetchConcurrency,
	}
	results, cancel, err := fetcher.Start(ctx, req.Files)
	if err != nil {
		return err
	}
	defer cancel()

	var total int64
	var totalMu sync.Mutex
	for _, ch := range results {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case result, ok := <-ch:
			if !ok {
				return fmt.Errorf("media fetch ended unexpectedly")
			}
			if result.Err != nil {
				return result.Err
			}
			if result.Body == nil {
				return fmt.Errorf("media response body is empty")
			}
			if err := c.writeFilePart(writer, result, &total, &totalMu); err != nil {
				_ = result.Body.Close()
				return err
			}
		}
	}
	return nil
}

func (c Client) writeFilePart(writer *multipart.Writer, result mediafetch.Result, total *int64, totalMu *sync.Mutex) error {
	defer result.Body.Close()

	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="files"; filename="%s"`, escapeQuotes(result.Filename)))
	header.Set("Content-Type", result.ContentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return err
	}

	counting := &mediafetch.CountingReader{
		Reader:    result.Body,
		MaxSingle: c.Config.MaxSingleMediaBytes,
		MaxTotal:  c.Config.MaxTotalMediaBytes,
		Total:     total,
		TotalMu:   totalMu,
	}
	_, err = io.Copy(part, counting)
	return err
}

func (c Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func readUpstreamError(resp *http.Response) error {
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	message := strings.TrimSpace(string(body))
	var parsed map[string]any
	if json.Unmarshal(body, &parsed) == nil {
		if m, ok := parsed["message"].(string); ok && strings.TrimSpace(m) != "" {
			message = strings.TrimSpace(m)
		}
	}
	return UpstreamError{StatusCode: resp.StatusCode, Message: message, Body: string(body)}
}

func waitPipeError(errs <-chan error) error {
	for err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func isExpectedPipeClose(err error) bool {
	return errors.Is(err, io.ErrClosedPipe) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded)
}

func escapeQuotes(value string) string {
	return strings.ReplaceAll(value, `"`, `\"`)
}

func ParseTaskID(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("task id is required")
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("task id must be numeric")
		}
	}
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("task id must be numeric")
	}
	return id, nil
}
