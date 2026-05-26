// Package config is a thin generic wrapper around kelseyhightower/envconfig.
// Services declare their config as a struct tagged with `envconfig:"..."`
// and required:"true" / default:"..." annotations, then call MustLoad at
// startup. Failing fast on missing required env vars is a deliberate choice:
// a misconfigured process should never receive traffic.
package config

import (
	"fmt"

	"github.com/kelseyhightower/envconfig"
)

func Load[T any](prefix string) (T, error) {
	var cfg T
	if err := envconfig.Process(prefix, &cfg); err != nil {
		return cfg, fmt.Errorf("config: load prefix %q: %w", prefix, err)
	}
	return cfg, nil
}

func MustLoad[T any](prefix string) T {
	cfg, err := Load[T](prefix)
	if err != nil {
		panic(err)
	}
	return cfg
}
