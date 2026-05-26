package config_test

import (
	"testing"
	"time"

	"github.com/online-shop/pkg/config"
)

type sampleConfig struct {
	Port     int           `envconfig:"PORT" default:"8080"`
	DBURL    string        `envconfig:"DATABASE_URL" required:"true"`
	Timeout  time.Duration `envconfig:"TIMEOUT" default:"5s"`
	Features []string      `envconfig:"FEATURES"`
}

func TestLoad_ParsesValues(t *testing.T) {
	t.Setenv("SVC_PORT", "9090")
	t.Setenv("SVC_DATABASE_URL", "postgres://localhost/db")
	t.Setenv("SVC_TIMEOUT", "30s")
	t.Setenv("SVC_FEATURES", "a,b,c")

	got, err := config.Load[sampleConfig]("SVC")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Port != 9090 {
		t.Errorf("Port = %d, want 9090", got.Port)
	}
	if got.DBURL != "postgres://localhost/db" {
		t.Errorf("DBURL = %q", got.DBURL)
	}
	if got.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v", got.Timeout)
	}
	if len(got.Features) != 3 || got.Features[0] != "a" {
		t.Errorf("Features = %v", got.Features)
	}
}

func TestLoad_AppliesDefaults(t *testing.T) {
	t.Setenv("SVC_DATABASE_URL", "postgres://localhost/db")

	got, err := config.Load[sampleConfig]("SVC")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Port != 8080 {
		t.Errorf("Port default = %d, want 8080", got.Port)
	}
	if got.Timeout != 5*time.Second {
		t.Errorf("Timeout default = %v, want 5s", got.Timeout)
	}
}

func TestLoad_ErrorOnMissingRequired(t *testing.T) {
	_, err := config.Load[sampleConfig]("SVC_NOPE_")
	if err == nil {
		t.Fatal("expected error for missing required DATABASE_URL")
	}
}

func TestLoad_PrefixScoping(t *testing.T) {
	t.Setenv("APP_DATABASE_URL", "postgres://app/db")
	t.Setenv("OTHER_DATABASE_URL", "postgres://other/db")

	got, err := config.Load[sampleConfig]("APP")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.DBURL != "postgres://app/db" {
		t.Errorf("prefix not honoured: got %q", got.DBURL)
	}
}

func TestMustLoad_PanicsOnError(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("MustLoad should panic on missing required field")
		}
	}()
	_ = config.MustLoad[sampleConfig]("SVC_PANIC_")
}
