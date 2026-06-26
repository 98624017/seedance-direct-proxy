package openai

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

const AssetModel = "seedance-asset"

var ReferenceFieldOrder = []string{
	"images",
	"image",
	"input_reference",
	"files",
	"input_video",
	"video_url",
	"reference_video",
	"audio",
	"audios",
}

type CreateRequest struct {
	Model         string
	Prompt        string
	Duration      string
	AspectRatio   string
	Files         []string
	GenerateAudio string
	Watermark     string
	Resolution    string
	Raw           map[string]any
}

type VideoResponse struct {
	ID        string         `json:"id"`
	TaskID    string         `json:"task_id"`
	Object    string         `json:"object"`
	Model     string         `json:"model,omitempty"`
	Status    string         `json:"status"`
	Progress  int            `json:"progress"`
	AssetID   string         `json:"asset_id,omitempty"`
	AssetURI  string         `json:"asset_uri,omitempty"`
	CreatedAt int64          `json:"created_at,omitempty"`
	UpdatedAt int64          `json:"updated_at,omitempty"`
	URL       string         `json:"url,omitempty"`
	VideoURL  string         `json:"video_url,omitempty"`
	Error     *ErrorDetail   `json:"error,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type ErrorDetail struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

type AssetRequest struct {
	Model             string
	Name              string
	ImageURL          string
	IgnoredImageCount int
	Raw               map[string]any
}

type JimengRequest struct {
	Model         string
	Prompt        string
	Ratio         string
	Resolution    string
	Duration      float64
	ReferenceMode string
	FilePaths     []string
	Raw           map[string]any
}

func ParseCreateRequest(body []byte) (CreateRequest, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return CreateRequest{}, fmt.Errorf("invalid JSON body")
	}

	model := stringValue(raw["model"])
	prompt := stringValue(raw["prompt"])
	if model == "" {
		return CreateRequest{}, fmt.Errorf("model is required")
	}
	if prompt == "" {
		return CreateRequest{}, fmt.Errorf("prompt is required")
	}

	duration := stringValue(raw["duration"])
	if duration == "" {
		duration = stringValue(raw["seconds"])
	}
	if duration == "" {
		duration = "4"
	}

	aspectRatio := stringValue(raw["aspect_ratio"])
	if aspectRatio == "" {
		aspectRatio = aspectRatioFromSize(stringValue(raw["size"]))
	}
	if aspectRatio == "" {
		aspectRatio = "16:9"
	}

	model, resolution := normalizeModelAndResolution(model, defaultString(raw["resolution"], "480p"))
	files := collectReferenceFiles(raw)

	return CreateRequest{
		Model:         model,
		Prompt:        prompt,
		Duration:      duration,
		AspectRatio:   aspectRatio,
		Files:         files,
		GenerateAudio: defaultString(raw["generate_audio"], "true"),
		Watermark:     defaultString(raw["watermark"], "false"),
		Resolution:    resolution,
		Raw:           raw,
	}, nil
}

func ModelFromBody(body []byte) string {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return ""
	}
	return stringValue(raw["model"])
}

func IsAssetModel(model string) bool {
	return strings.TrimSpace(model) == AssetModel
}

func ParseAssetRequest(body []byte) (AssetRequest, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return AssetRequest{}, fmt.Errorf("invalid JSON body")
	}

	model := stringValue(raw["model"])
	if model == "" {
		return AssetRequest{}, fmt.Errorf("model is required")
	}
	if !IsAssetModel(model) {
		return AssetRequest{}, fmt.Errorf("model is not an asset model")
	}

	name := stringValue(raw["prompt"])
	if name == "" {
		return AssetRequest{}, fmt.Errorf("prompt is required")
	}

	imageURL, ignored := firstAssetImageURL(raw)
	if imageURL == "" {
		return AssetRequest{}, fmt.Errorf("image url is required")
	}
	if err := ValidatePublicHTTPURL(imageURL); err != nil {
		return AssetRequest{}, err
	}

	return AssetRequest{
		Model:             model,
		Name:              name,
		ImageURL:          imageURL,
		IgnoredImageCount: ignored,
		Raw:               raw,
	}, nil
}

func ParseJimengRequest(body []byte) (JimengRequest, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return JimengRequest{}, fmt.Errorf("invalid JSON body")
	}

	model := stringValue(raw["model"])
	if model == "" {
		return JimengRequest{}, fmt.Errorf("model is required")
	}
	if IsAssetModel(model) {
		return JimengRequest{}, fmt.Errorf("seedance-asset is not supported by jimeng upstream; use a video model and pass public image URLs in files")
	}

	prompt := stringValue(raw["prompt"])
	if prompt == "" {
		return JimengRequest{}, fmt.Errorf("prompt is required")
	}

	files, err := collectJimengFilePaths(raw)
	if err != nil {
		return JimengRequest{}, err
	}
	referenceMode := stringValue(raw["reference_mode"])
	if referenceMode == "" {
		if len(files) > 0 {
			referenceMode = "omni"
		} else {
			referenceMode = "text_to_video"
		}
	}
	if err := validateJimengReferenceMode(referenceMode, len(files)); err != nil {
		return JimengRequest{}, err
	}

	duration, err := jimengDuration(raw)
	if err != nil {
		return JimengRequest{}, err
	}

	ratio := stringValue(raw["ratio"])
	if ratio == "" {
		ratio = stringValue(raw["aspect_ratio"])
	}
	resolution := stringValue(raw["resolution"])
	if resolution == "" {
		resolution = "720p"
	}

	return JimengRequest{
		Model:         model,
		Prompt:        prompt,
		Ratio:         ratio,
		Resolution:    resolution,
		Duration:      duration,
		ReferenceMode: referenceMode,
		FilePaths:     files,
		Raw:           raw,
	}, nil
}

func ParseAssetTaskUnix(taskID string) int64 {
	parts := strings.Split(taskID, "_")
	if len(parts) < 4 || parts[0] != "asset" || parts[1] != "req" {
		return 0
	}
	var ts int64
	_, _ = fmt.Sscanf(parts[2], "%d", &ts)
	return ts
}

func ValidateAssetTaskID(taskID string) error {
	parts := strings.Split(strings.TrimSpace(taskID), "_")
	if len(parts) != 4 || parts[0] != "asset" || parts[1] != "req" {
		return fmt.Errorf("asset task id must use asset_req_<unix>_<12 lowercase hex chars>")
	}
	if parts[2] == "" || len(parts[3]) != 12 {
		return fmt.Errorf("asset task id must use asset_req_<unix>_<12 lowercase hex chars>")
	}
	for _, r := range parts[2] {
		if r < '0' || r > '9' {
			return fmt.Errorf("asset task timestamp must be numeric")
		}
	}
	for _, r := range parts[3] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return fmt.Errorf("asset task random suffix must be lowercase hex")
		}
	}
	if ParseAssetTaskUnix(taskID) <= 0 {
		return fmt.Errorf("asset task timestamp must be positive")
	}
	return nil
}

func firstAssetImageURL(raw map[string]any) (string, int) {
	candidates := make([]string, 0)
	for _, key := range []string{"input_reference", "image", "images", "files"} {
		appendAssetImageValues(&candidates, raw[key])
	}
	for i, candidate := range candidates {
		if candidate != "" {
			return candidate, len(candidates) - i - 1
		}
	}
	return "", 0
}

func appendAssetImageValues(out *[]string, value any) {
	switch v := value.(type) {
	case string:
		if s := strings.TrimSpace(v); s != "" {
			*out = append(*out, s)
		}
	case []any:
		for _, item := range v {
			appendAssetImageValues(out, item)
		}
	case map[string]any:
		if s := stringValue(v["url"]); s != "" {
			*out = append(*out, s)
			return
		}
		if nested, ok := v["image_url"]; ok {
			appendAssetImageValues(out, nested)
		}
	}
}

func ValidatePublicHTTPURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("image url must be a public http or https URL")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("image url must be a public http or https URL")
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return fmt.Errorf("image url host is required")
	}
	lowerHost := strings.ToLower(strings.TrimSuffix(host, "."))
	if lowerHost == "localhost" || strings.HasSuffix(lowerHost, ".localhost") {
		return fmt.Errorf("image url host is not allowed")
	}
	if strings.Contains(lowerHost, "%") {
		return fmt.Errorf("image url host is not allowed")
	}
	if ip := net.ParseIP(lowerHost); ip != nil && isBlockedIP(ip) {
		return fmt.Errorf("image url host is not allowed")
	}
	return nil
}

func collectJimengFilePaths(raw map[string]any) ([]string, error) {
	files := make([]string, 0)
	for _, key := range []string{"files", "input_reference", "file_paths", "filePaths"} {
		appendReferenceFileValues(&files, raw[key])
	}
	for _, file := range files {
		if strings.HasPrefix(strings.TrimSpace(file), "asset://") {
			return nil, fmt.Errorf("asset:// references are not supported by jimeng upstream; use public image URLs")
		}
		if err := ValidatePublicHTTPURL(file); err != nil {
			return nil, err
		}
	}
	return files, nil
}

func validateJimengReferenceMode(mode string, fileCount int) error {
	switch mode {
	case "omni":
		if fileCount < 1 || fileCount > 12 {
			return fmt.Errorf("reference_mode=omni requires 1-12 URLs")
		}
	case "text_to_video":
		if fileCount != 0 {
			return fmt.Errorf("reference_mode=text_to_video does not accept URLs")
		}
	case "first_frame", "last_frame":
		if fileCount != 1 {
			return fmt.Errorf("reference_mode=%s requires exactly 1 URL", mode)
		}
	case "both_frames":
		if fileCount != 2 {
			return fmt.Errorf("reference_mode=both_frames requires exactly 2 URLs")
		}
	default:
		return fmt.Errorf("reference_mode must be one of omni, first_frame, last_frame, both_frames, text_to_video")
	}
	return nil
}

func jimengDuration(raw map[string]any) (float64, error) {
	value, ok := raw["duration"]
	if !ok || stringValue(value) == "" {
		value = raw["seconds"]
	}
	if stringValue(value) == "" {
		return 4, nil
	}
	switch v := value.(type) {
	case float64:
		return v, nil
	case json.Number:
		n, err := strconv.ParseFloat(v.String(), 64)
		if err != nil {
			return 0, fmt.Errorf("duration must be numeric")
		}
		return n, nil
	case string:
		n, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return 0, fmt.Errorf("duration must be numeric")
		}
		return n, nil
	default:
		text := stringValue(value)
		if text == "" {
			return 4, nil
		}
		n, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return 0, fmt.Errorf("duration must be numeric")
		}
		return n, nil
	}
}

func isBlockedIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified()
}

func normalizeModelAndResolution(model, resolution string) (string, string) {
	switch model {
	case "doubao-seedance-2-0-fast-260128-480p":
		return "doubao-seedance-2-0-fast-260128", "480p"
	case "doubao-seedance-2-0-260128-480p":
		return "doubao-seedance-2-0-260128", "480p"
	case "doubao-seedance-2-0-fast-260128-720p":
		return "doubao-seedance-2-0-fast-260128", "720p"
	case "doubao-seedance-2-0-260128-720p":
		return "doubao-seedance-2-0-260128", "720p"
	default:
		return model, resolution
	}
}

func stringValue(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	case bool:
		if v {
			return "true"
		}
		return "false"
	case json.Number:
		return strings.TrimSpace(v.String())
	default:
		return ""
	}
}

func defaultString(value any, fallback string) string {
	if s := stringValue(value); s != "" {
		return s
	}
	return fallback
}

func collectReferenceFiles(raw map[string]any) []string {
	files := make([]string, 0)
	for _, key := range ReferenceFieldOrder {
		value, ok := raw[key]
		if !ok {
			continue
		}
		appendReferenceFileValues(&files, value)
	}
	return files
}

func appendReferenceFileValues(out *[]string, value any) {
	switch v := value.(type) {
	case string:
		if s := strings.TrimSpace(v); s != "" {
			*out = append(*out, s)
		}
	case []any:
		for _, item := range v {
			appendReferenceFileValues(out, item)
		}
	case map[string]any:
		if s := stringValue(v["url"]); s != "" {
			*out = append(*out, s)
			return
		}
		for _, key := range []string{"image_url", "video_url", "audio_url", "file_url"} {
			if nested, ok := v[key]; ok {
				appendReferenceFileValues(out, nested)
			}
		}
	}
}

func aspectRatioFromSize(size string) string {
	switch strings.ToLower(strings.TrimSpace(size)) {
	case "720x1280", "1024x1792":
		return "9:16"
	case "1280x720", "1792x1024":
		return "16:9"
	default:
		return ""
	}
}
