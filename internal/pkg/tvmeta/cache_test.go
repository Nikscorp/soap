package tvmeta

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

// TestNewCacheMetricsTVMetaFamilyNames pins the public Prometheus metric
// shape exposed by tvmeta. Dashboards scrape against the literal family
// names tvmeta_cache_{hits,misses,errors}_total with a `method` label whose
// values are exactly {details, all_seasons, search}; a silent rename of the
// names or label values would only show up as a blank dashboard panel
// post-deploy. Generic cache mechanics live in internal/pkg/lrucache and are
// covered there — this test exists *only* to guard the byte-identical names.
func TestNewCacheMetricsTVMetaFamilyNames(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := newCacheMetrics(reg)
	require.NotNil(t, metrics)

	// Construct one cache per method so the registry sees children for
	// every label value the caller will emit in production. Touch each so
	// the children appear in the gather output.
	c := New(nil, nil, CacheConfig{
		DetailsSize:    16,
		DetailsTTL:     time.Hour,
		AllSeasonsSize: 16,
		AllSeasonsTTL:  time.Hour,
		SearchSize:     16,
		SearchTTL:      time.Hour,
	}, reg)

	// CounterVecs with no observed children are omitted from Gather output;
	// touch hits, misses, and errors so all three families surface in the
	// gather, otherwise the "family must be registered" assertions below
	// would catch a silent rename only by accident.
	detailsFetch := func(_ context.Context) (*TvShowDetails, error) { return &TvShowDetails{}, nil }
	allSeasonsFetch := func(_ context.Context) (*AllSeasonsWithDetails, error) { return &AllSeasonsWithDetails{}, nil }
	searchFetch := func(_ context.Context) (*TVShows, error) { return &TVShows{}, nil }

	// Miss (cold) and hit (second call, same key).
	for range 2 {
		_, _ = c.detailsCache.GetOrFetch(context.Background(), detailsKey{id: 1, lang: "en"}, detailsFetch)
		_, _ = c.allSeasonsCache.GetOrFetch(context.Background(), allSeasonsKey{id: 1, lang: "en"}, allSeasonsFetch)
		_, _ = c.searchCache.GetOrFetch(context.Background(), searchKey{query: "q", lang: "en"}, searchFetch)
	}
	// Error increments the errors family for each cache.
	failDetails := func(_ context.Context) (*TvShowDetails, error) { return nil, context.DeadlineExceeded }
	failAllSeasons := func(_ context.Context) (*AllSeasonsWithDetails, error) { return nil, context.DeadlineExceeded }
	failSearch := func(_ context.Context) (*TVShows, error) { return nil, context.DeadlineExceeded }
	_, _ = c.detailsCache.GetOrFetch(context.Background(), detailsKey{id: 999, lang: "en"}, failDetails)
	_, _ = c.allSeasonsCache.GetOrFetch(context.Background(), allSeasonsKey{id: 999, lang: "en"}, failAllSeasons)
	_, _ = c.searchCache.GetOrFetch(context.Background(), searchKey{query: "fail", lang: "en"}, failSearch)

	families, err := reg.Gather()
	require.NoError(t, err)

	wantFamilies := map[string]bool{
		"tvmeta_cache_hits_total":   false,
		"tvmeta_cache_misses_total": false,
		"tvmeta_cache_errors_total": false,
	}
	wantMethods := map[string]bool{
		"details":     false,
		"all_seasons": false,
		"search":      false,
	}

	for _, f := range families {
		if _, ok := wantFamilies[f.GetName()]; !ok {
			continue
		}
		wantFamilies[f.GetName()] = true
		for _, m := range f.GetMetric() {
			labels := m.GetLabel()
			require.Len(t, labels, 1, "metric %s should have exactly one label", f.GetName())
			require.Equal(t, "method", labels[0].GetName(),
				"metric %s must use the `method` label", f.GetName())
			if v := labels[0].GetValue(); wantMethods[v] != true {
				wantMethods[v] = true
			}
		}
	}

	for name, found := range wantFamilies {
		require.True(t, found, "Prometheus family %q must be registered", name)
	}
	require.True(t, wantMethods["details"], "method=details must be observed")
	require.True(t, wantMethods["all_seasons"], "method=all_seasons must be observed")
	require.True(t, wantMethods["search"], "method=search must be observed")
}
