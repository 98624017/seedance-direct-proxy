package config

import "testing"

func TestLoadUsesSeparateDefaultAssetUpstreamBaseURL(t *testing.T) {
	t.Setenv("UPSTREAM_BASE_URL", "")
	t.Setenv("ASSET_UPSTREAM_BASE_URL", "")

	cfg := Load()

	if cfg.UpstreamBaseURL != defaultUpstreamBaseURL {
		t.Fatalf("unexpected upstream base URL: %q", cfg.UpstreamBaseURL)
	}
	if cfg.AssetUpstreamBaseURL != defaultAssetUpstreamBaseURL {
		t.Fatalf("unexpected asset upstream base URL: %q", cfg.AssetUpstreamBaseURL)
	}
}

func TestLoadAllowsOverridingAssetUpstreamBaseURL(t *testing.T) {
	t.Setenv("UPSTREAM_BASE_URL", "http://video.example.test/")
	t.Setenv("ASSET_UPSTREAM_BASE_URL", "http://asset.example.test/")

	cfg := Load()

	if cfg.UpstreamBaseURL != "http://video.example.test" {
		t.Fatalf("unexpected upstream base URL: %q", cfg.UpstreamBaseURL)
	}
	if cfg.AssetUpstreamBaseURL != "http://asset.example.test" {
		t.Fatalf("unexpected asset upstream base URL: %q", cfg.AssetUpstreamBaseURL)
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
