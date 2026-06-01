package seedance

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/98624017/seedance-direct-proxy/internal/openai"
)

type CreateResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		ID int64 `json:"Id"`
	} `json:"data"`
	Success bool `json:"success"`
}

type QueryResponse struct {
	Code    int       `json:"code"`
	Message string    `json:"message"`
	Data    QueryData `json:"data"`
	Success bool      `json:"success"`
}

type QueryData struct {
	ID          int64  `json:"Id"`
	CreatedAt   string `json:"CreatedAt"`
	UpdatedAt   string `json:"UpdatedAt"`
	Status      int    `json:"Status"`
	StatusText  string `json:"StatusText"`
	Message     string `json:"Message"`
	VideoURL    string `json:"VideoUrl"`
	UseToken    int64  `json:"UseToken"`
	DeductToken int64  `json:"DeductToken"`
	UseDuration int64  `json:"UseDuration"`
}

func NormalizeCreate(resp CreateResponse, model string, now time.Time) openai.VideoResponse {
	id := strconv.FormatInt(resp.Data.ID, 10)
	return openai.VideoResponse{
		ID:        id,
		TaskID:    id,
		Object:    "video",
		Model:     model,
		Status:    "queued",
		Progress:  0,
		CreatedAt: now.Unix(),
		Metadata: map[string]any{
			"seedance": map[string]any{
				"code":    resp.Code,
				"message": resp.Message,
				"success": resp.Success,
				"Id":      resp.Data.ID,
			},
		},
	}
}

func NormalizeQuery(resp QueryResponse, model string) openai.VideoResponse {
	id := strconv.FormatInt(resp.Data.ID, 10)
	status := StatusName(resp.Data.Status)
	out := openai.VideoResponse{
		ID:        id,
		TaskID:    id,
		Object:    "video",
		Model:     model,
		Status:    status,
		Progress:  Progress(status),
		CreatedAt: parseUnix(resp.Data.CreatedAt),
		UpdatedAt: parseUnix(resp.Data.UpdatedAt),
		Metadata: map[string]any{
			"seedance": map[string]any{
				"code":        resp.Code,
				"message":     resp.Message,
				"success":     resp.Success,
				"Id":          resp.Data.ID,
				"Status":      resp.Data.Status,
				"StatusText":  resp.Data.StatusText,
				"Message":     resp.Data.Message,
				"UseToken":    resp.Data.UseToken,
				"DeductToken": resp.Data.DeductToken,
				"UseDuration": resp.Data.UseDuration,
			},
		},
	}

	if status == "completed" && strings.TrimSpace(resp.Data.VideoURL) != "" {
		out.URL = strings.TrimSpace(resp.Data.VideoURL)
		out.VideoURL = out.URL
	}
	if status == "failed" {
		message := strings.TrimSpace(resp.Data.Message)
		if message == "" {
			message = strings.TrimSpace(resp.Data.StatusText)
		}
		if message == "" {
			message = "Seedance task failed"
		}
		out.Error = &openai.ErrorDetail{Code: fmt.Sprintf("%d", resp.Data.Status), Message: message}
	}

	return out
}

func StatusName(status int) string {
	switch status {
	case 0:
		return "queued"
	case 1:
		return "in_progress"
	case 2:
		return "completed"
	case 3:
		return "failed"
	default:
		return "queued"
	}
}

func Progress(status string) int {
	switch status {
	case "queued":
		return 0
	case "in_progress":
		return 50
	case "completed", "failed":
		return 100
	default:
		return 0
	}
}

func parseUnix(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t.Unix()
	}
	return 0
}
