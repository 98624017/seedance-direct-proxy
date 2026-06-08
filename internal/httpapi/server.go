package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/98624017/seedance-direct-proxy/internal/config"
	"github.com/98624017/seedance-direct-proxy/internal/openai"
	"github.com/98624017/seedance-direct-proxy/internal/seedance"
)

type Server struct {
	Config config.Config
	Client seedance.Client
	Logger *slog.Logger
}

type deleteAssetRequest struct {
	TaskID string `json:"task_id"`
}

func (s Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRoot)
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/api/task/token/asset/delete", s.handleTokenAssetDelete)
	mux.HandleFunc("/v1/videos", s.handleVideos)
	mux.HandleFunc("/v1/videos/", s.handleVideoTask)
	return logMiddleware(s.logger(), mux)
}

func (s Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		writeError(w, http.StatusNotFound, "not found", nil)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = w.Write([]byte("ok"))
}

func (s Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok"))
}

func (s Server) handleVideos(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/v1/videos" {
		writeError(w, http.StatusNotFound, "not found", nil)
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}

	token, err := extractBearerToken(r.Header.Get("Authorization"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error(), nil)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body", nil)
		return
	}
	if openai.IsAssetModel(openai.ModelFromBody(body)) {
		assetReq, err := openai.ParseAssetRequest(body)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error(), nil)
			return
		}
		resp, err := s.Client.CreateAsset(r.Context(), assetReq, token)
		if err != nil {
			s.writeUpstreamError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}

	createReq, err := openai.ParseCreateRequest(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if len(createReq.Files) > s.Config.MaxReferenceFiles {
		writeError(w, http.StatusBadRequest, "too many reference files", map[string]any{"max": s.Config.MaxReferenceFiles})
		return
	}

	resp, err := s.Client.Create(r.Context(), createReq, token)
	if err != nil {
		s.writeUpstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s Server) handleVideoTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	token, err := extractBearerToken(r.Header.Get("Authorization"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error(), nil)
		return
	}

	taskIDRaw := strings.TrimPrefix(r.URL.Path, "/v1/videos/")
	if taskIDRaw == "" || strings.Contains(taskIDRaw, "/") {
		writeError(w, http.StatusNotFound, "not found", nil)
		return
	}
	if strings.HasPrefix(taskIDRaw, "asset_req_") {
		if err := openai.ValidateAssetTaskID(taskIDRaw); err != nil {
			writeError(w, http.StatusBadRequest, err.Error(), nil)
			return
		}
		resp, err := s.Client.QueryAsset(r.Context(), taskIDRaw, token)
		if err != nil {
			s.writeUpstreamError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}

	taskID, err := seedance.ParseTaskID(taskIDRaw)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}

	resp, err := s.Client.Query(r.Context(), taskID, token)
	if err != nil {
		s.writeUpstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s Server) handleTokenAssetDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	token, err := extractBearerToken(r.Header.Get("Authorization"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error(), nil)
		return
	}

	var req deleteAssetRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	req.TaskID = strings.TrimSpace(req.TaskID)
	if err := openai.ValidateAssetTaskID(req.TaskID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}

	deleted, err := s.Client.DeleteAssetByTaskID(r.Context(), req.TaskID, token)
	if err != nil {
		s.writeUpstreamError(w, err)
		return
	}
	deletedAt := time.Now().Unix()
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "",
		"data": map[string]any{
			"task_id":     deleted.TaskID,
			"deleted":     true,
			"deleted_at":  deletedAt,
			"resource_id": deleted.ResourceID,
			"asset_id":    deleted.AssetID,
			"asset_uri":   deleted.AssetURI,
		},
	})
}

func (s Server) writeUpstreamError(w http.ResponseWriter, err error) {
	var validation openai.ValidationError
	if errors.As(err, &validation) {
		writeError(w, http.StatusBadRequest, validation.Error(), nil)
		return
	}
	var upstream seedance.UpstreamError
	if errors.As(err, &upstream) {
		status := upstream.StatusCode
		if status < http.StatusBadRequest {
			status = http.StatusBadGateway
		}
		writeError(w, status, upstream.Error(), map[string]any{"upstream_body": upstream.Body})
		return
	}
	writeError(w, http.StatusBadGateway, err.Error(), nil)
}

func extractBearerToken(header string) (string, error) {
	header = strings.TrimSpace(header)
	if header == "" {
		return "", errors.New("missing Authorization header")
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(strings.ToLower(header), strings.ToLower(prefix)) {
		return "", errors.New("Authorization header must use Bearer")
	}
	token := strings.TrimSpace(header[len(prefix):])
	if token == "" {
		return "", errors.New("empty bearer token")
	}
	if idx := strings.LastIndex(token, "|"); idx >= 0 {
		token = strings.TrimSpace(token[idx+1:])
	}
	if token == "" {
		return "", errors.New("empty bearer token")
	}
	return token, nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string, detail map[string]any) {
	payload := map[string]any{
		"error": map[string]any{
			"message": message,
		},
	}
	if len(detail) > 0 {
		payload["error"].(map[string]any)["detail"] = detail
	}
	writeJSON(w, status, payload)
}

func (s Server) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func logMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		logger.Info("request", "method", r.Method, "path", r.URL.Path)
	})
}
