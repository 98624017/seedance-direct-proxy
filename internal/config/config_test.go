package config

import "testing"

func TestLoadUsesSeparateDefaultAssetUpstreamBaseURL(t *testing.T) {
	t.Setenv("UPSTREAM_BASE_URL", "")
	t.Setenv("JIMENG_UPSTREAM_BASE_URL", "")
	t.Setenv("ASSET_UPSTREAM_BASE_URL", "")
	t.Setenv("VIDEO_UPSTREAM_PROVIDER", "")

	cfg := Load()

	if cfg.VideoUpstreamProvider != "jimeng" {
		t.Fatalf("unexpected provider: %q", cfg.VideoUpstreamProvider)
	}
	if cfg.JimengUpstreamBaseURL != defaultJimengUpstreamBaseURL {
		t.Fatalf("unexpected jimeng upstream base URL: %q", cfg.JimengUpstreamBaseURL)
	}
	if cfg.UpstreamBaseURL != defaultUpstreamBaseURL {
		t.Fatalf("unexpected upstream base URL: %q", cfg.UpstreamBaseURL)
	}
	if cfg.AssetUpstreamBaseURL != defaultAssetUpstreamBaseURL {
		t.Fatalf("unexpected asset upstream base URL: %q", cfg.AssetUpstreamBaseURL)
	}
}

func TestLoadAllowsOverridingAssetUpstreamBaseURL(t *testing.T) {
	t.Setenv("UPSTREAM_BASE_URL", "http://video.example.test/")
	t.Setenv("JIMENG_UPSTREAM_BASE_URL", "http://jimeng.example.test/")
	t.Setenv("ASSET_UPSTREAM_BASE_URL", "http://asset.example.test/")
	t.Setenv("VIDEO_UPSTREAM_PROVIDER", "legacy")

	cfg := Load()

	if cfg.VideoUpstreamProvider != "legacy" {
		t.Fatalf("unexpected provider: %q", cfg.VideoUpstreamProvider)
	}
	if cfg.UpstreamBaseURL != "http://video.example.test" {
		t.Fatalf("unexpected upstream base URL: %q", cfg.UpstreamBaseURL)
	}
	if cfg.JimengUpstreamBaseURL != "http://jimeng.example.test" {
		t.Fatalf("unexpected jimeng upstream base URL: %q", cfg.JimengUpstreamBaseURL)
	}
	if cfg.AssetUpstreamBaseURL != "http://asset.example.test" {
		t.Fatalf("unexpected asset upstream base URL: %q", cfg.AssetUpstreamBaseURL)
	}
}

func TestLoadFallsBackToJimengForInvalidProvider(t *testing.T) {
	t.Setenv("VIDEO_UPSTREAM_PROVIDER", "typo")

	cfg := Load()

	if cfg.VideoUpstreamProvider != "jimeng" {
		t.Fatalf("unexpected provider: %q", cfg.VideoUpstreamProvider)
	}
	if !cfg.VideoUpstreamInvalid {
		t.Fatalf("expected invalid provider flag")
	}
}

func TestLoadParsesAssetUpstreamTokens(t *testing.T) {
	t.Setenv("ASSET_UPSTREAM_TOKENS", " token-a , token-b ,, token-c ")

	cfg := Load()

	if len(cfg.AssetUpstreamTokens) != 3 {
		t.Fatalf("unexpected token count: %#v", cfg.AssetUpstreamTokens)
	}
	if cfg.AssetUpstreamTokens[0] != "token-a" || cfg.AssetUpstreamTokens[1] != "token-b" || cfg.AssetUpstreamTokens[2] != "token-c" {
		t.Fatalf("unexpected tokens: %#v", cfg.AssetUpstreamTokens)
	}
}
