package tmdb

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTMDB_OK(t *testing.T) {
	cfg := Config{
		APIKey:          "test-api-key",
		EnableAutoRetry: true,
		RequestTimeout:  5 * time.Second,
		MaxIdleConns:    10,
		IdleConnTimeout: 30 * time.Second,
	}

	c, err := NewTMDB(cfg)
	require.NoError(t, err)
	assert.NotNil(t, c)
}

func TestNewTMDB_AutoRetryDisabled(t *testing.T) {
	cfg := Config{
		APIKey:          "another-key",
		EnableAutoRetry: false,
		RequestTimeout:  time.Second,
		MaxIdleConns:    1,
		IdleConnTimeout: time.Second,
	}

	c, err := NewTMDB(cfg)
	require.NoError(t, err)
	assert.NotNil(t, c)
}

func TestNewTMDB_EmptyAPIKey(t *testing.T) {
	c, err := NewTMDB(Config{})
	require.Error(t, err)
	assert.Nil(t, c)
}
