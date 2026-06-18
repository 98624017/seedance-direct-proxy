package seedance

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
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

func (c Client) CreateAsset(ctx context.Context, req openai.AssetRequest, token string) (openai.VideoResponse, error) {
	client := c.httpClient()
	cfg := c.Config
	ctx, cancel := contextWithOptionalTimeout(ctx, cfg.UpstreamCreateTimeout)
	defer cancel()

	now := time.Now()
	if c.Now != nil {
		now = c.Now()
	}
	taskID, err := assetTaskID(now)
	if err != nil {
		return openai.VideoResponse{}, err
	}
	upstreamName := assetUpstreamName(req.Name, taskID)
	if assetUpstreamNameLength(upstreamName) > maxAssetUpstreamNameChars {
		return openai.VideoResponse{}, openai.ValidationError{
			Message: fmt.Sprintf("asset name is too long, max %d characters", maxAssetDisplayNameChars),
		}
	}
	payload := map[string]any{
		"Name":    upstreamName,
		"OssPath": req.ImageURL,
	}
	body, _ := json.Marshal(payload)
	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.assetUpstreamBaseURL()+"/resources/user/Resources", bytes.NewReader(body))
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

	var out CreateAssetResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return openai.VideoResponse{}, fmt.Errorf("decode seedance asset create response: %w", err)
	}
	if out.Code != 0 || !out.Success {
		return openai.VideoResponse{}, UpstreamError{StatusCode: resp.StatusCode, Message: out.Message}
	}

	return openai.VideoResponse{
		ID:        taskID,
		TaskID:    taskID,
		Object:    "video",
		Model:     openai.AssetModel,
		Status:    "queued",
		Progress:  0,
		CreatedAt: now.Unix(),
		Metadata: assetMetadata(map[string]any{
			"kind":                "asset",
			"name":                req.Name,
			"upstream_name":       upstreamName,
			"oss_path":            req.ImageURL,
			"ignored_image_count": req.IgnoredImageCount,
			"code":                out.Code,
			"message":             out.Message,
			"success":             out.Success,
		}),
	}, nil
}

func (c Client) QueryAsset(ctx context.Context, taskID string, token string) (openai.VideoResponse, error) {
	ctx, cancel := contextWithOptionalTimeout(ctx, c.Config.UpstreamQueryTimeout)
	defer cancel()

	item, scannedPages, lastResponse, ok, err := c.findAssetResource(ctx, taskID, token)
	if err != nil {
		return openai.VideoResponse{}, err
	}
	if ok {
		return NormalizeAssetQuery(taskID, item, scannedPages), nil
	}
	return NormalizeAssetNotFound(taskID, scannedPages, lastResponse), nil
}

func (c Client) DeleteAssetByTaskID(ctx context.Context, taskID string, token string) (DeletedAsset, error) {
	ctx, cancel := contextWithOptionalTimeout(ctx, c.Config.UpstreamQueryTimeout)
	defer cancel()

	item, _, _, ok, err := c.findAssetResource(ctx, taskID, token)
	if err != nil {
		return DeletedAsset{}, err
	}
	if !ok {
		return DeletedAsset{}, UpstreamError{StatusCode: http.StatusNotFound, Message: "asset resource not found"}
	}
	if item.ID <= 0 {
		return DeletedAsset{}, UpstreamError{StatusCode: http.StatusBadGateway, Message: "asset resource id is missing"}
	}
	if err := c.deleteAssetResource(ctx, item.ID, token); err != nil {
		return DeletedAsset{}, err
	}
	assetID := strings.TrimSpace(item.AssetID)
	return DeletedAsset{
		TaskID:     taskID,
		ResourceID: item.ID,
		AssetID:    assetID,
		AssetURI:   assetURI(assetID),
	}, nil
}

func (c Client) deleteAssetResource(ctx context.Context, resourceID int64, token string) error {
	client := c.httpClient()
	payload := map[string]int64{"Id": resourceID}
	body, _ := json.Marshal(payload)
	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.assetUpstreamBaseURL()+"/resources/user/Resources", bytes.NewReader(body))
	if err != nil {
		return err
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("token", token)

	resp, err := client.Do(upstreamReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return readUpstreamError(resp)
	}

	var out DeleteAssetResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return fmt.Errorf("decode seedance asset delete response: %w", err)
	}
	if out.Code != 0 || !out.Success {
		return UpstreamError{StatusCode: resp.StatusCode, Message: out.Message}
	}
	return nil
}

func (c Client) findAssetResource(ctx context.Context, taskID string, token string) (AssetResource, int, AssetListResponse, bool, error) {
	client := c.httpClient()
	cfg := c.Config

	now := time.Now()
	if c.Now != nil {
		now = c.Now()
	}
	maxPages := assetListPagesForTask(taskID, now, cfg.AssetListBasePages, cfg.AssetListMediumPages, cfg.AssetListMaxPages)
	var lastResponse AssetListResponse
	for page := 1; page <= maxPages; page++ {
		payload := map[string]int{"Page": page}
		body, _ := json.Marshal(payload)
		upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.assetUpstreamBaseURL()+"/resources/user/ResourcesList", bytes.NewReader(body))
		if err != nil {
			return AssetResource{}, maxPages, lastResponse, false, err
		}
		upstreamReq.Header.Set("Content-Type", "application/json")
		upstreamReq.Header.Set("token", token)

		resp, err := client.Do(upstreamReq)
		if err != nil {
			return AssetResource{}, maxPages, lastResponse, false, err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			err := readUpstreamError(resp)
			return AssetResource{}, maxPages, lastResponse, false, err
		}
		var out AssetListResponse
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			_ = resp.Body.Close()
			return AssetResource{}, maxPages, lastResponse, false, fmt.Errorf("decode seedance asset list response: %w", err)
		}
		_ = resp.Body.Close()
		if out.Code != 0 || !out.Success {
			return AssetResource{}, maxPages, lastResponse, false, UpstreamError{StatusCode: resp.StatusCode, Message: out.Message}
		}
		lastResponse = out
		for _, item := range out.Data.Data {
			if assetResourceNameMatchesTask(item.Name, taskID) {
				return item, maxPages, lastResponse, true, nil
			}
		}
	}
	return AssetResource{}, maxPages, lastResponse, false, nil
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

	return c.writeFileParts(ctx, writer, req.Files)
}

func (c Client) writeFileParts(ctx context.Context, writer *multipart.Writer, files []string) error {
	httpURLs := make([]string, 0, len(files))
	resultIndexes := make(map[int]int, len(files))
	for i, file := range files {
		if isHTTPReference(file) {
			resultIndexes[i] = len(httpURLs)
			httpURLs = append(httpURLs, strings.TrimSpace(file))
		}
	}

	var results []<-chan mediafetch.Result
	cancel := func() {}
	if len(httpURLs) > 0 {
		fetcher := mediafetch.Fetcher{
			Client:              c.httpClient(),
			Timeout:             c.Config.MediaFetchTimeout,
			MaxSingleMediaBytes: c.Config.MaxSingleMediaBytes,
			MaxTotalMediaBytes:  c.Config.MaxTotalMediaBytes,
			PrefetchConcurrency: c.Config.MediaPrefetchConcurrency,
		}
		var err error
		results, cancel, err = fetcher.Start(ctx, httpURLs)
		if err != nil {
			return err
		}
	}
	defer cancel()

	var total int64
	var totalMu sync.Mutex
	for i, file := range files {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		resultIndex, ok := resultIndexes[i]
		if !ok {
			if err := writer.WriteField("files", file); err != nil {
				return err
			}
			continue
		}

		ch := results[resultIndex]
		result, ok := <-ch
		if !ok {
			return ctx.Err()
		}
		if result.Err != nil {
			return fmt.Errorf("fetch reference file %d: %w", i+1, result.Err)
		}
		if result.Body == nil {
			return fmt.Errorf("fetch reference file %d: empty response body", i+1)
		}
		if err := c.writeSingleFilePart(writer, result, &total, &totalMu); err != nil {
			_ = result.Body.Close()
			return err
		}
	}
	return nil
}

func isHTTPReference(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == nil {
		return false
	}
	return (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func (c Client) writeSingleFilePart(writer *multipart.Writer, result mediafetch.Result, total *int64, totalMu *sync.Mutex) error {
	defer result.Body.Close()

	part, err := writer.CreatePart(filePartHeader(result))
	if err != nil {
		return err
	}
	reader := &mediafetch.CountingReader{
		Reader:     result.Body,
		MaxSingle:  c.Config.MaxSingleMediaBytes,
		MaxTotal:   c.Config.MaxTotalMediaBytes,
		Total:      total,
		TotalMu:    totalMu,
		SingleRead: 0,
	}
	if _, err := io.Copy(part, reader); err != nil {
		return err
	}
	return nil
}

func filePartHeader(result mediafetch.Result) textproto.MIMEHeader {
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{
		"name":     "files",
		"filename": result.Filename,
	}))
	if result.ContentType != "" {
		header.Set("Content-Type", result.ContentType)
	} else {
		header.Set("Content-Type", "application/octet-stream")
	}
	return header
}

func (c Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c Client) assetUpstreamBaseURL() string {
	if strings.TrimSpace(c.Config.AssetUpstreamBaseURL) != "" {
		return strings.TrimRight(c.Config.AssetUpstreamBaseURL, "/")
	}
	return strings.TrimRight(c.Config.UpstreamBaseURL, "/")
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

func assetTaskID(now time.Time) (string, error) {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return fmt.Sprintf("asset_req_%d_%x", now.Unix(), buf), nil
}

const (
	assetTracePrefix          = "__ar_"
	maxAssetUpstreamNameChars = 50
	maxAssetDisplayNameChars  = maxAssetUpstreamNameChars - len(assetTracePrefix) - 12
)

func assetUpstreamName(name, taskID string) string {
	return strings.TrimSpace(name) + assetTraceSuffix(taskID)
}

func assetResourceNameMatchesTask(name, taskID string) bool {
	return strings.HasSuffix(strings.TrimSpace(name), assetTraceSuffix(taskID))
}

func assetTraceSuffix(taskID string) string {
	return assetTracePrefix + assetTaskRandomSuffix(taskID)
}

func assetTaskRandomSuffix(taskID string) string {
	parts := strings.Split(strings.TrimSpace(taskID), "_")
	if len(parts) != 4 {
		return ""
	}
	return parts[3]
}

func assetUpstreamNameLength(name string) int {
	return len([]rune(strings.TrimSpace(name)))
}

func assetMetadata(seedance map[string]any) map[string]any {
	return map[string]any{"seedance": seedance}
}

func assetListPagesForTask(taskID string, now time.Time, basePages, mediumPages, maxPages int) int {
	if basePages <= 0 {
		basePages = 10
	}
	if mediumPages <= 0 {
		mediumPages = 20
	}
	if maxPages <= 0 {
		maxPages = 50
	}
	createdAt := openai.ParseAssetTaskUnix(taskID)
	if createdAt <= 0 {
		return basePages
	}
	age := now.Sub(time.Unix(createdAt, 0))
	switch {
	case age >= time.Hour:
		return maxPages
	case age >= 10*time.Minute:
		return mediumPages
	default:
		return basePages
	}
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
