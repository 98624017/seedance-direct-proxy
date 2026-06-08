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

type CreateAssetResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
	Success bool   `json:"success"`
}

type DeleteAssetResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
	Success bool   `json:"success"`
}

type AssetListResponse struct {
	Code    int           `json:"code"`
	Message string        `json:"message"`
	Data    AssetListData `json:"data"`
	Success bool          `json:"success"`
}

type AssetListData struct {
	Data  []AssetResource `json:"data"`
	Total int             `json:"total"`
}

type AssetResource struct {
	ID                   int64  `json:"Id"`
	CreatedAt            string `json:"CreatedAt"`
	UpdatedAt            string `json:"UpdatedAt"`
	CreatedID            int64  `json:"CreatedId"`
	CreatedName          string `json:"CreatedName"`
	UserUserID           int64  `json:"UserUserId"`
	UserResourcesGroupID int64  `json:"UserResourcesGroupId"`
	Name                 string `json:"Name"`
	OssPath              string `json:"OssPath"`
	Desc                 string `json:"Desc"`
	Prompt               string `json:"Prompt"`
	UserResourcesTypeID  int64  `json:"UserResourcesTypeId"`
	Status               int    `json:"Status"`
	StatusText           string `json:"StatusText"`
	Message              string `json:"Message"`
	AssetID              string `json:"AssetId"`
}

type DeletedAsset struct {
	TaskID     string
	ResourceID int64
	AssetID    string
	AssetURI   string
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

func NormalizeAssetQuery(taskID string, item AssetResource, scannedPages int) openai.VideoResponse {
	status := "in_progress"
	progress := 50
	var errDetail *openai.ErrorDetail
	message := strings.TrimSpace(item.Message)
	statusText := strings.TrimSpace(item.StatusText)
	assetID := strings.TrimSpace(item.AssetID)
	assetURI := assetURI(assetID)
	if assetID != "" {
		status = "completed"
		progress = 100
	} else if assetResourceFailed(message, statusText) {
		status = "failed"
		progress = 100
		if message == "" {
			message = statusText
		}
		if message == "" {
			message = "Seedance asset task failed"
		}
		errDetail = &openai.ErrorDetail{Code: fmt.Sprintf("%d", item.Status), Message: message}
	}

	name := stripAssetTraceSuffix(item.Name)
	out := openai.VideoResponse{
		ID:        taskID,
		TaskID:    taskID,
		Object:    "video",
		Model:     openai.AssetModel,
		Status:    status,
		Progress:  progress,
		AssetID:   assetID,
		AssetURI:  assetURI,
		CreatedAt: parseUnix(item.CreatedAt),
		UpdatedAt: parseUnix(item.UpdatedAt),
		Error:     errDetail,
		Metadata: map[string]any{
			"seedance": map[string]any{
				"kind":                   "asset",
				"asset_id":               assetID,
				"asset_uri":              assetURI,
				"resource_id":            item.ID,
				"name":                   name,
				"upstream_name":          item.Name,
				"oss_path":               item.OssPath,
				"status":                 item.Status,
				"status_text":            item.StatusText,
				"message":                item.Message,
				"scanned_pages":          scannedPages,
				"user_resources_type_id": item.UserResourcesTypeID,
			},
		},
	}
	return out
}

func assetURI(assetID string) string {
	assetID = strings.TrimSpace(assetID)
	if assetID == "" {
		return ""
	}
	return "asset://" + assetID
}

func NormalizeAssetNotFound(taskID string, scannedPages int, resp AssetListResponse) openai.VideoResponse {
	return openai.VideoResponse{
		ID:       taskID,
		TaskID:   taskID,
		Object:   "video",
		Model:    openai.AssetModel,
		Status:   "in_progress",
		Progress: 50,
		Metadata: map[string]any{
			"seedance": map[string]any{
				"kind":          "asset",
				"scanned_pages": scannedPages,
				"message":       "asset resource not found yet",
				"total":         resp.Data.Total,
			},
		},
	}
}

func assetResourceFailed(message, statusText string) bool {
	text := strings.ToLower(strings.TrimSpace(message + " " + statusText))
	if text == "" {
		return false
	}
	for _, marker := range []string{"失败", "错误", "异常", "未通过", "fail", "failed", "error"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func stripAssetTraceSuffix(name string) string {
	const marker = "__asset_req_"
	idx := strings.LastIndex(name, marker)
	if idx < 0 {
		return strings.TrimSpace(name)
	}
	return strings.TrimSpace(name[:idx])
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
