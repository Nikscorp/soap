package tvmeta

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestNewAllSeasonsCacheEnabledByConfig verifies that a non-zero CacheConfig
// produces an active LRU on allSeasonsCache and that a zero CacheConfig keeps
// the cache in pass-through mode (lru == nil), preserving deterministic
// per-call TMDB calls for tests that haven't opted in.
func TestNewAllSeasonsCacheEnabledByConfig(t *testing.T) {
	t.Run("non-zero config enables LRU", func(t *testing.T) {
		c := New(nil, nil, CacheConfig{
			AllSeasonsSize: 64,
			AllSeasonsTTL:  time.Minute,
		}, nil)
		require.NotNil(t, c.allSeasonsCache)
		require.NotNil(t, c.allSeasonsCache.lru)
		require.Equal(t, "all_seasons", c.allSeasonsCache.name)
	})

	t.Run("zero config yields pass-through", func(t *testing.T) {
		c := New(nil, nil, CacheConfig{}, nil)
		require.NotNil(t, c.allSeasonsCache)
		require.Nil(t, c.allSeasonsCache.lru)
	})

	t.Run("zero size disables cache", func(t *testing.T) {
		c := New(nil, nil, CacheConfig{
			AllSeasonsSize: 0,
			AllSeasonsTTL:  time.Minute,
		}, nil)
		require.Nil(t, c.allSeasonsCache.lru)
	})

	t.Run("zero ttl disables cache", func(t *testing.T) {
		c := New(nil, nil, CacheConfig{
			AllSeasonsSize: 64,
			AllSeasonsTTL:  0,
		}, nil)
		require.Nil(t, c.allSeasonsCache.lru)
	})
}
