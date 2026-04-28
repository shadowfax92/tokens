package ccusage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const cacheVersion = 1

type cacheFile struct {
	Version   int        `json:"version"`
	FetchedAt time.Time  `json:"fetched_at"`
	Data      *UsageData `json:"data"`
}

type FetchOptions struct {
	NoCache  bool
	CacheTTL time.Duration
}

type FetchResult struct {
	Data       *UsageData
	FromCache  bool
	FetchedAt  time.Time
	CacheAge   time.Duration
	FetchTook  time.Duration
}

func cachePath() string {
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "tokens", "cache.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "tokens", "cache.json")
}

func CachePath() string {
	return cachePath()
}

func readCache() (*cacheFile, error) {
	data, err := os.ReadFile(cachePath())
	if err != nil {
		return nil, err
	}
	var cf cacheFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("parse cache: %w", err)
	}
	if cf.Version != cacheVersion {
		return nil, errors.New("cache version mismatch")
	}
	return &cf, nil
}

func writeCache(data *UsageData) error {
	p := cachePath()
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	cf := cacheFile{
		Version:   cacheVersion,
		FetchedAt: time.Now(),
		Data:      data,
	}
	b, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func ClearCache() error {
	err := os.Remove(cachePath())
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func Fetch(opts FetchOptions) *FetchResult {
	if !opts.NoCache {
		if cf, err := readCache(); err == nil {
			age := time.Since(cf.FetchedAt)
			if age < opts.CacheTTL {
				return &FetchResult{
					Data:      cf.Data,
					FromCache: true,
					FetchedAt: cf.FetchedAt,
					CacheAge:  age,
				}
			}
		}
	}

	start := time.Now()
	data := fetchAll()
	took := time.Since(start)

	if len(data.Errors) == 0 {
		_ = writeCache(data)
	}

	return &FetchResult{
		Data:      data,
		FromCache: false,
		FetchedAt: time.Now(),
		FetchTook: took,
	}
}
