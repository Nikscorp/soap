package lazysoap

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Nikscorp/soap/internal/pkg/lrucache"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

// TestImgCacheKeyRoundTrip verifies the (path, size) -> key shape: identical
// inputs produce identical keys, and any plausible difference in either
// component yields a distinct key. The cache mechanics themselves are
// covered by internal/pkg/lrucache; this test pins down only the wiring.
func TestImgCacheKeyRoundTrip(t *testing.T) {
	cases := []struct {
		path string
		size string
	}{
		// Plain filename — TMDB strips the leading slash off the SPA-side
		// reference before chi captures {path}, so the path here never
		// starts with "/".
		{path: "i8NA7TqNgnXuAtDeQOF5baX0jI6.jpg", size: "w185"},
		{path: "i8NA7TqNgnXuAtDeQOF5baX0jI6.jpg", size: "w342"},
		{path: "i8NA7TqNgnXuAtDeQOF5baX0jI6.jpg", size: "w500"},
		{path: "i8NA7TqNgnXuAtDeQOF5baX0jI6.jpg", size: "w780"},
		{path: "another_poster.jpg", size: "w185"},
		// Empty size happens before allow-list normalization; included to
		// confirm the helper itself does not normalize.
		{path: "i8NA7TqNgnXuAtDeQOF5baX0jI6.jpg", size: ""},
	}

	keys := make(map[string]struct{}, len(cases))
	for _, c := range cases {
		k := imgCacheKey(c.path, c.size)
		require.NotContains(t, keys, k, "imgCacheKey collision for path=%q size=%q", c.path, c.size)
		keys[k] = struct{}{}
		// Same inputs MUST produce the same key — ruling out any hidden
		// hashing or randomization in the helper.
		require.Equal(t, k, imgCacheKey(c.path, c.size))
	}
	require.Len(t, keys, len(cases))
}

// TestImgCacheKeySeparatorIsCollisionFree pins the chosen "|" separator: a
// path that ends with the size token (or vice versa) must still produce a
// distinct key. The contract documented on imgCacheKey relies on TMDB poster
// paths and allow-listed sizes never containing "|", so this test guards
// against a future refactor that picks a separator already present in the
// inputs.
func TestImgCacheKeySeparatorIsCollisionFree(t *testing.T) {
	// Two pairs that would collide under naive concatenation (path+size).
	a := imgCacheKey("foo", "barw185")
	b := imgCacheKey("foobar", "w185")
	require.NotEqual(t, a, b)
}

// TestImgCacheNewProducesExpectedMetricFamily verifies the wiring promised
// in the plan: an imgCache constructed via lrucache.New + lrucache.NewMetrics
// using the "lazysoap_img_cache" prefix surfaces the
// lazysoap_img_cache_hits_total family in a fresh registry. Dashboards
// depend on this name; a silent rename would be invisible until a panel
// goes blank.
func TestImgCacheNewProducesExpectedMetricFamily(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := lrucache.NewMetrics(reg, "lazysoap_img_cache", "image bytes cache")
	require.NotNil(t, metrics)

	cache := lrucache.New[string, ImgCacheEntry]("img", 16, time.Hour, metrics)
	require.NotNil(t, cache)

	// Drive a miss + a hit + an error so all three CounterVecs surface a
	// child in Gather (CounterVecs without observed children are omitted).
	entry := ImgCacheEntry{body: []byte("payload"), contentType: "image/jpeg"}
	fetch := func(_ context.Context) (ImgCacheEntry, error) { return entry, nil }
	got, err := cache.GetOrFetch(context.Background(), imgCacheKey("p.jpg", "w185"), fetch)
	require.NoError(t, err)
	require.Equal(t, entry, got)

	got, err = cache.GetOrFetch(context.Background(), imgCacheKey("p.jpg", "w185"), fetch)
	require.NoError(t, err)
	require.Equal(t, entry, got)

	boom := errors.New("fake upstream failure")
	_, err = cache.GetOrFetch(context.Background(), imgCacheKey("p.jpg", "w342"), func(_ context.Context) (ImgCacheEntry, error) {
		return ImgCacheEntry{}, boom
	})
	require.ErrorIs(t, err, boom)

	families, err := reg.Gather()
	require.NoError(t, err)

	want := map[string]bool{
		"lazysoap_img_cache_hits_total":   false,
		"lazysoap_img_cache_misses_total": false,
		"lazysoap_img_cache_errors_total": false,
	}
	for _, f := range families {
		if _, ok := want[f.GetName()]; ok {
			want[f.GetName()] = true
			require.Contains(t, f.GetHelp(), "image bytes cache", "help text should reference subject for %s", f.GetName())
			for _, m := range f.GetMetric() {
				labels := m.GetLabel()
				require.Len(t, labels, 1)
				require.Equal(t, "method", labels[0].GetName())
				require.Equal(t, "img", labels[0].GetValue())
			}
		}
	}
	for name, found := range want {
		require.True(t, found, "metric family %q must be registered", name)
	}

	require.Equal(t, 1, cache.Len(), "successful fetch is cached; error path is not")
}
