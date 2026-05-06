package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseConfig_OK(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := []byte(`
lazysoap:
  listen_addr: 127.0.0.1:9999
  read_timeout: 7s
tmdb:
  api_key: "secret-key"
  request_timeout: 3s
`)
	require.NoError(t, os.WriteFile(path, body, 0o600))

	cfg, err := ParseConfig(path)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "127.0.0.1:9999", cfg.LazySoapConfig.Address)
	assert.Equal(t, 7*time.Second, cfg.LazySoapConfig.ReadTimeout)
	assert.Equal(t, "secret-key", cfg.TMDBConfig.APIKey)
	assert.Equal(t, 3*time.Second, cfg.TMDBConfig.RequestTimeout)

	// tvmeta.cache defaults come from env-default tags, since the YAML body
	// above has no `tvmeta:` section. Documents the safe-by-default cache
	// behavior: zero-config builds get the documented sizes/TTLs.
	assert.Equal(t, 1024, cfg.TVMeta.Cache.DetailsSize)
	assert.Equal(t, 6*time.Hour, cfg.TVMeta.Cache.DetailsTTL)
	assert.Equal(t, 1024, cfg.TVMeta.Cache.AllSeasonsSize)
	assert.Equal(t, 6*time.Hour, cfg.TVMeta.Cache.AllSeasonsTTL)
	assert.Equal(t, 256, cfg.TVMeta.Cache.SearchSize)
	assert.Equal(t, 30*time.Minute, cfg.TVMeta.Cache.SearchTTL)
}

func TestParseConfig_TVMetaCacheOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := []byte(`
tmdb:
  api_key: "k"
tvmeta:
  cache:
    details_size: 2
    details_ttl: 1m
    all_seasons_size: 4
    all_seasons_ttl: 2m
    search_size: 8
    search_ttl: 3m
`)
	require.NoError(t, os.WriteFile(path, body, 0o600))

	cfg, err := ParseConfig(path)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, 2, cfg.TVMeta.Cache.DetailsSize)
	assert.Equal(t, time.Minute, cfg.TVMeta.Cache.DetailsTTL)
	assert.Equal(t, 4, cfg.TVMeta.Cache.AllSeasonsSize)
	assert.Equal(t, 2*time.Minute, cfg.TVMeta.Cache.AllSeasonsTTL)
	assert.Equal(t, 8, cfg.TVMeta.Cache.SearchSize)
	assert.Equal(t, 3*time.Minute, cfg.TVMeta.Cache.SearchTTL)
}

func TestParseConfig_MissingFile(t *testing.T) {
	cfg, err := ParseConfig(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	require.Error(t, err)
	assert.Nil(t, cfg)
}

func TestParseConfig_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.yaml")
	require.NoError(t, os.WriteFile(path, []byte("::not yaml::"), 0o600))

	cfg, err := ParseConfig(path)
	require.Error(t, err)
	assert.Nil(t, cfg)
}
