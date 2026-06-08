package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port                     string
	UpstreamBaseURL          string
	AssetUpstreamBaseURL     string
	MaxReferenceFiles        int
	MaxSingleMediaBytes      int64
	MaxTotalMediaBytes       int64
	MediaPrefetchConcurrency int
	MediaFetchTimeout        time.Duration
	UpstreamCreateTimeout    time.Duration
	UpstreamQueryTimeout     time.Duration
	AssetListBasePages       int
	AssetListMediumPages     int
	AssetListMaxPages        int
	ShutdownTimeout          time.Duration
}

const (
	defaultUpstreamBaseURL      = "http://119.45.252.34:8618"
	defaultAssetUpstreamBaseURL = "http://119.45.42.208:8620"
)

func Load() Config {
	upstreamBaseURL := trimRightSlash(getString("UPSTREAM_BASE_URL", defaultUpstreamBaseURL))
	assetUpstreamBaseURL := trimRightSlash(getString("ASSET_UPSTREAM_BASE_URL", defaultAssetUpstreamBaseURL))
	return Config{
		Port:                     getString("PORT", "3000"),
		UpstreamBaseURL:          upstreamBaseURL,
		AssetUpstreamBaseURL:     assetUpstreamBaseURL,
		MaxReferenceFiles:        getInt("MAX_REFERENCE_FILES", 12),
		MaxSingleMediaBytes:      getInt64("MAX_SINGLE_MEDIA_BYTES", 52428800),
		MaxTotalMediaBytes:       getInt64("MAX_TOTAL_MEDIA_BYTES", 209715200),
		MediaPrefetchConcurrency: getInt("MEDIA_PREFETCH_CONCURRENCY", 6),
		MediaFetchTimeout:        time.Duration(getInt("MEDIA_FETCH_TIMEOUT_SECONDS", 75)) * time.Second,
		UpstreamCreateTimeout:    time.Duration(getInt("UPSTREAM_CREATE_TIMEOUT_SECONDS", 180)) * time.Second,
		UpstreamQueryTimeout:     time.Duration(getInt("UPSTREAM_QUERY_TIMEOUT_SECONDS", 30)) * time.Second,
		AssetListBasePages:       getInt("ASSET_LIST_BASE_PAGES", 10),
		AssetListMediumPages:     getInt("ASSET_LIST_MEDIUM_PAGES", 20),
		AssetListMaxPages:        getInt("ASSET_LIST_MAX_PAGES", 50),
		ShutdownTimeout:          time.Duration(getInt("SHUTDOWN_TIMEOUT_SECONDS", 10)) * time.Second,
	}
}

func getString(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func getInt64(key string, fallback int64) int64 {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func trimRightSlash(value string) string {
	for len(value) > 1 && value[len(value)-1] == '/' {
		value = value[:len(value)-1]
	}
	return value
}
