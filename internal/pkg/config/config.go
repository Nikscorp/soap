package config

import (
	"github.com/Nikscorp/soap/internal/app/lazysoap"
	"github.com/Nikscorp/soap/internal/pkg/clients/tmdb"
	"github.com/Nikscorp/soap/internal/pkg/trace"
	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	LazySoapConfig lazysoap.Config `yaml:"lazysoap"`
	TMDBConfig     tmdb.Config     `yaml:"tmdb"`
	Trace          trace.Config    `yaml:"trace"`
}

func ParseConfig(path string) (*Config, error) {
	var cfg Config
	err := cleanenv.ReadConfig(path, &cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}
