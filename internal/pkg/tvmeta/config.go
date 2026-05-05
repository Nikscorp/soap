package tvmeta

// Config groups the tvmeta package's runtime configuration. Today it only
// carries CacheConfig; new tvmeta-level knobs (e.g. concurrency caps, default
// page sizes) would slot in alongside Cache rather than ballooning the
// top-level config tree.
type Config struct {
	Cache CacheConfig `yaml:"cache"`
}
