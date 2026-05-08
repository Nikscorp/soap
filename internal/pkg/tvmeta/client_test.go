package tvmeta

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestNewAllSeasonsCacheEnabledByConfig verifies that a non-zero CacheConfig
// produces an active LRU on allSeasonsCache (a second GetOrFetch hits the
// cached value, not the fetch function) and that a zero CacheConfig keeps
// the cache in pass-through mode (every GetOrFetch invokes fetch), preserving
// deterministic per-call TMDB calls for tests that haven't opted in.
//
// The observable signal here is fetch-call-count: lrucache.Cache deliberately
// hides its internal fields, so tests verify behavior rather than peek at
// pointers.
func TestNewAllSeasonsCacheEnabledByConfig(t *testing.T) {
	t.Run("non-zero config enables LRU", func(t *testing.T) {
		c := New(nil, nil, CacheConfig{
			AllSeasonsSize: 64,
			AllSeasonsTTL:  time.Minute,
		}, nil)
		require.NotNil(t, c.allSeasonsCache)
		require.Equal(t, 1, callsAfterTwoGetOrFetch(t, c), "active cache must serve second call from LRU")
		require.Equal(t, 1, c.allSeasonsCache.Len(), "active cache must retain one entry after warm-up")
	})

	t.Run("zero config yields pass-through", func(t *testing.T) {
		c := New(nil, nil, CacheConfig{}, nil)
		require.NotNil(t, c.allSeasonsCache)
		require.Equal(t, 2, callsAfterTwoGetOrFetch(t, c), "pass-through must invoke fetch on every call")
		require.Equal(t, 0, c.allSeasonsCache.Len())
	})

	t.Run("zero size disables cache", func(t *testing.T) {
		c := New(nil, nil, CacheConfig{
			AllSeasonsSize: 0,
			AllSeasonsTTL:  time.Minute,
		}, nil)
		require.Equal(t, 2, callsAfterTwoGetOrFetch(t, c))
	})

	t.Run("zero ttl disables cache", func(t *testing.T) {
		c := New(nil, nil, CacheConfig{
			AllSeasonsSize: 64,
			AllSeasonsTTL:  0,
		}, nil)
		require.Equal(t, 2, callsAfterTwoGetOrFetch(t, c))
	})
}

// callsAfterTwoGetOrFetch issues two GetOrFetch calls against allSeasonsCache
// for the same key and returns how many times the fetch closure ran. 1 ⇒
// active LRU served the second call; 2 ⇒ pass-through.
func callsAfterTwoGetOrFetch(t *testing.T, c *Client) int {
	t.Helper()
	var calls atomic.Int32
	fetch := func(_ context.Context) (*AllSeasonsWithDetails, error) {
		calls.Add(1)
		return &AllSeasonsWithDetails{}, nil
	}
	key := allSeasonsKey{id: 1, lang: "en"}
	_, err := c.allSeasonsCache.GetOrFetch(context.Background(), key, fetch)
	require.NoError(t, err)
	_, err = c.allSeasonsCache.GetOrFetch(context.Background(), key, fetch)
	require.NoError(t, err)
	return int(calls.Load())
}
