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
