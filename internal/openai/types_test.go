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

func TestParseCreateRequestCollectsObjectReferences(t *testing.T) {
	raw := []byte(`{
		"model":"veofast",
		"prompt":"test",
		"images":[
			{"url":"https://asset.test/image-1.jpg"},
			{"image_url":{"url":"https://asset.test/image-2.jpg"}}
		],
		"input_video":{"video_url":"https://asset.test/ref.mp4"},
		"audio":{"audio_url":{"url":"https://asset.test/ref.mp3"}}
	}`)
	req, err := ParseCreateRequest(raw)
	if err != nil {
		t.Fatalf("ParseCreateRequest() error = %v", err)
	}

	wantFiles := []string{
		"https://asset.test/image-1.jpg",
		"https://asset.test/image-2.jpg",
		"https://asset.test/ref.mp4",
		"https://asset.test/ref.mp3",
	}
	if len(req.Files) != len(wantFiles) {
		t.Fatalf("files len = %d, want %d: %#v", len(req.Files), len(wantFiles), req.Files)
	}
	for i := range wantFiles {
		if req.Files[i] != wantFiles[i] {
			t.Fatalf("files[%d] = %q, want %q", i, req.Files[i], wantFiles[i])
		}
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

func TestParseJimengRequestNormalizesFieldsAndPreservesDuplicates(t *testing.T) {
	raw := []byte(`{
		"model":"jimeng-video-seedance-2.0-vip",
		"prompt":"use @1 and @2",
		"aspect_ratio":"4:3",
		"duration":"6",
		"files":["https://asset.test/a.jpg","https://asset.test/a.jpg"],
		"input_reference":"https://asset.test/b.jpg",
		"file_paths":["https://asset.test/c.jpg"],
		"filePaths":["https://asset.test/d.jpg"]
	}`)
	req, err := ParseJimengRequest(raw)
	if err != nil {
		t.Fatalf("ParseJimengRequest() error = %v", err)
	}
	if req.Model != "jimeng-video-seedance-2.0-vip" || req.Prompt != "use @1 and @2" {
		t.Fatalf("unexpected request: %#v", req)
	}
	if req.Ratio != "4:3" || req.Resolution != "720p" || req.Duration != 6 || req.ReferenceMode != "omni" {
		t.Fatalf("unexpected normalized fields: %#v", req)
	}
	want := []string{
		"https://asset.test/a.jpg",
		"https://asset.test/a.jpg",
		"https://asset.test/b.jpg",
		"https://asset.test/c.jpg",
		"https://asset.test/d.jpg",
	}
	if len(req.FilePaths) != len(want) {
		t.Fatalf("file paths = %#v", req.FilePaths)
	}
	for i := range want {
		if req.FilePaths[i] != want[i] {
			t.Fatalf("file_paths[%d] = %q, want %q", i, req.FilePaths[i], want[i])
		}
	}
}

func TestParseJimengRequestReferenceModeRules(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{
			name: "text to video default",
			raw:  `{"model":"m","prompt":"p"}`,
		},
		{
			name: "both frames same url allowed",
			raw:  `{"model":"m","prompt":"p","reference_mode":"both_frames","files":["https://asset.test/a.jpg","https://asset.test/a.jpg"]}`,
		},
		{
			name:    "both frames requires two",
			raw:     `{"model":"m","prompt":"p","reference_mode":"both_frames","files":["https://asset.test/a.jpg"]}`,
			wantErr: true,
		},
		{
			name:    "first frame requires one",
			raw:     `{"model":"m","prompt":"p","reference_mode":"first_frame","files":["https://asset.test/a.jpg","https://asset.test/b.jpg"]}`,
			wantErr: true,
		},
		{
			name:    "text to video rejects urls",
			raw:     `{"model":"m","prompt":"p","reference_mode":"text_to_video","files":["https://asset.test/a.jpg"]}`,
			wantErr: true,
		},
		{
			name:    "chinese reference mode rejected",
			raw:     `{"model":"m","prompt":"p","reference_mode":"首帧参考","files":["https://asset.test/a.jpg"]}`,
			wantErr: true,
		},
		{
			name:    "asset uri rejected",
			raw:     `{"model":"m","prompt":"p","files":["asset://asset-1"]}`,
			wantErr: true,
		},
		{
			name:    "unsafe url rejected",
			raw:     `{"model":"m","prompt":"p","files":["http://127.0.0.1/a.jpg"]}`,
			wantErr: true,
		},
		{
			name:    "asset model rejected",
			raw:     `{"model":"seedance-asset","prompt":"p","files":["https://asset.test/a.jpg"]}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseJimengRequest([]byte(tt.raw))
			if tt.wantErr && err == nil {
				t.Fatalf("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestParseAssetRequestTakesFirstImageAndCountsIgnored(t *testing.T) {
	raw := []byte(`{
		"model":"seedance-asset",
		"prompt":"林春芽",
		"input_reference":"https://asset.test/primary.jpg",
		"image":"https://asset.test/ignored-1.jpg",
		"images":["https://asset.test/ignored-2.jpg"],
		"files":["https://asset.test/ignored-3.jpg"]
	}`)
	req, err := ParseAssetRequest(raw)
	if err != nil {
		t.Fatalf("ParseAssetRequest() error = %v", err)
	}
	if req.Name != "林春芽" || req.ImageURL != "https://asset.test/primary.jpg" {
		t.Fatalf("unexpected request: %#v", req)
	}
	if req.IgnoredImageCount != 3 {
		t.Fatalf("ignored count = %d, want 3", req.IgnoredImageCount)
	}
}

func TestParseAssetRequestRejectsUnsafeURLs(t *testing.T) {
	for _, imageURL := range []string{
		"file:///tmp/a.jpg",
		"data:image/png;base64,aaaa",
		"http://localhost/a.jpg",
		"http://127.0.0.1/a.jpg",
		"http://10.0.0.1/a.jpg",
		"http://172.16.0.1/a.jpg",
		"http://192.168.1.1/a.jpg",
		"http://169.254.1.1/a.jpg",
		"http://[fe80::1%25eth0]/a.jpg",
	} {
		raw, _ := json.Marshal(map[string]any{
			"model":           "seedance-asset",
			"prompt":          "asset",
			"input_reference": imageURL,
		})
		if _, err := ParseAssetRequest(raw); err == nil {
			t.Fatalf("ParseAssetRequest(%q) expected error", imageURL)
		}
	}
}

func TestValidateAssetTaskIDRejectsShortPrefixes(t *testing.T) {
	valid := []string{
		"asset_req_1780830000_abcdef123456",
		"asset_req_1780830000_0123abcdef45",
	}
	for _, taskID := range valid {
		if err := ValidateAssetTaskID(taskID); err != nil {
			t.Fatalf("ValidateAssetTaskID(%q) error = %v", taskID, err)
		}
	}

	invalid := []string{
		"",
		"asset_req_",
		"asset_req_1",
		"asset_req_1_",
		"asset_req_0_abcdef",
		"asset_req_x_abcdef",
		"asset_req_1780830000_",
		"asset_req_1780830000_abcdef",
		"asset_req_1780830000_ABCDEF",
		"asset_req_1780830000_abcdef123456_extra",
		"video_req_1780830000_abcdef",
	}
	for _, taskID := range invalid {
		if err := ValidateAssetTaskID(taskID); err == nil {
			t.Fatalf("ValidateAssetTaskID(%q) expected error", taskID)
		}
	}
}
