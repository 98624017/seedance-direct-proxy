package openai

import (
	"encoding/json"
	"testing"
)

func TestParseCreateRequestDefaultsAndReferenceOrder(t *testing.T) {
	body := map[string]any{
		"model":           "doubao-seedance-2-0-260128-2",
		"prompt":          "test prompt",
		"seconds":         4,
		"size":            "720x1280",
		"images":          []any{"https://asset.test/image-1.jpg"},
		"files":           []any{"https://asset.test/file-1.jpg", "https://asset.test/file-2.jpg"},
		"audio":           "https://asset.test/audio.mp3",
		"generate_audio":  true,
		"watermark":       false,
		"unknown_ignored": "x",
	}
	raw, _ := json.Marshal(body)

	req, err := ParseCreateRequest(raw)
	if err != nil {
		t.Fatalf("ParseCreateRequest() error = %v", err)
	}

	if req.Model != "doubao-seedance-2-0-260128-2" {
		t.Fatalf("model = %q", req.Model)
	}
	if req.Duration != "4" {
		t.Fatalf("duration = %q", req.Duration)
	}
	if req.AspectRatio != "9:16" {
		t.Fatalf("aspect_ratio = %q", req.AspectRatio)
	}
	if req.GenerateAudio != "true" || req.Watermark != "false" || req.Resolution != "480p" {
		t.Fatalf("defaults = %q %q %q", req.GenerateAudio, req.Watermark, req.Resolution)
	}
	wantFiles := []string{
		"https://asset.test/image-1.jpg",
		"https://asset.test/file-1.jpg",
		"https://asset.test/file-2.jpg",
		"https://asset.test/audio.mp3",
	}
	if len(req.Files) != len(wantFiles) {
		t.Fatalf("files len = %d, want %d", len(req.Files), len(wantFiles))
	}
	for i := range wantFiles {
		if req.Files[i] != wantFiles[i] {
			t.Fatalf("files[%d] = %q, want %q", i, req.Files[i], wantFiles[i])
		}
	}
}

func TestParseCreateRequestPreservesUnmappedModel(t *testing.T) {
	raw := []byte(`{"model":"veofast","prompt":"test"}`)
	req, err := ParseCreateRequest(raw)
	if err != nil {
		t.Fatalf("ParseCreateRequest() error = %v", err)
	}
	if req.Model != "veofast" {
		t.Fatalf("model = %q", req.Model)
	}
}

func TestParseCreateRequestMapsResolutionModelSuffix(t *testing.T) {
	tests := []struct {
		name           string
		model          string
		resolution     string
		wantModel      string
		wantResolution string
	}{
		{
			name:           "fast 480p",
			model:          "doubao-seedance-2-0-fast-260128-480p",
			resolution:     "720p",
			wantModel:      "doubao-seedance-2-0-fast-260128",
			wantResolution: "480p",
		},
		{
			name:           "standard 480p",
			model:          "doubao-seedance-2-0-260128-480p",
			resolution:     "720p",
			wantModel:      "doubao-seedance-2-0-260128",
			wantResolution: "480p",
		},
		{
			name:           "fast 720p",
			model:          "doubao-seedance-2-0-fast-260128-720p",
			resolution:     "480p",
			wantModel:      "doubao-seedance-2-0-fast-260128",
			wantResolution: "720p",
		},
		{
			name:           "standard 720p",
			model:          "doubao-seedance-2-0-260128-720p",
			resolution:     "480p",
			wantModel:      "doubao-seedance-2-0-260128",
			wantResolution: "720p",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, _ := json.Marshal(map[string]any{
				"model":      tt.model,
				"prompt":     "test",
				"resolution": tt.resolution,
			})
			req, err := ParseCreateRequest(raw)
			if err != nil {
				t.Fatalf("ParseCreateRequest() error = %v", err)
			}
			if req.Model != tt.wantModel {
				t.Fatalf("model = %q, want %q", req.Model, tt.wantModel)
			}
			if req.Resolution != tt.wantResolution {
				t.Fatalf("resolution = %q, want %q", req.Resolution, tt.wantResolution)
			}
		})
	}
}

func TestParseCreateRequestRequiresModelAndPrompt(t *testing.T) {
	for _, raw := range []string{
		`{"prompt":"x"}`,
		`{"model":"doubao-seedance-2-0-260128-2"}`,
		`{"model":"","prompt":"x"}`,
		`{"model":"doubao-seedance-2-0-260128-2","prompt":""}`,
	} {
		if _, err := ParseCreateRequest([]byte(raw)); err == nil {
			t.Fatalf("ParseCreateRequest(%s) expected error", raw)
		}
	}
}
