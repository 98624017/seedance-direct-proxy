package openai

import (
	"encoding/json"
	"fmt"
	"strings"
)

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
		switch v := value.(type) {
		case string:
			if s := strings.TrimSpace(v); s != "" {
				files = append(files, s)
			}
		case []any:
			for _, item := range v {
				if s := stringValue(item); s != "" {
					files = append(files, s)
				}
			}
		}
	}
	return files
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
