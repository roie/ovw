package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDefaultConfigValues(t *testing.T) {
	cfg := Default()

	if !reflect.DeepEqual(cfg.Roots, []string{"~/Code", "~/Projects"}) {
		t.Fatalf("Roots = %#v", cfg.Roots)
	}
	if cfg.MaxDepth != 0 {
		t.Fatalf("MaxDepth = %d", cfg.MaxDepth)
	}
	if cfg.ScanNestedProjects {
		t.Fatal("ScanNestedProjects = true")
	}
	if cfg.StaleDays != 30 {
		t.Fatalf("StaleDays = %d", cfg.StaleDays)
	}
	if !reflect.DeepEqual(cfg.Statuses, []string{"active", "parked", "shipped", "idea"}) {
		t.Fatalf("Statuses = %#v", cfg.Statuses)
	}
	if cfg.SortBy != "activity" || cfg.SortDir != "desc" {
		t.Fatalf("sort = %q %q", cfg.SortBy, cfg.SortDir)
	}
	if cfg.Stack.Aliases["Cloudflare Workers"] != "CF" {
		t.Fatalf("Cloudflare alias = %q", cfg.Stack.Aliases["Cloudflare Workers"])
	}
	if !cfg.Cache.Enabled {
		t.Fatal("cache disabled by default")
	}
}

func TestPathHelpersUseHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")

	paths, err := Paths()
	if err != nil {
		t.Fatalf("Paths() error = %v", err)
	}

	if paths.Config != filepath.Join(home, ".config", "ovw", "config.toml") {
		t.Fatalf("Config path = %q", paths.Config)
	}
	if paths.Metadata != filepath.Join(home, ".local", "share", "ovw", "projects.json") {
		t.Fatalf("Metadata path = %q", paths.Metadata)
	}
	if paths.Cache != filepath.Join(home, ".cache", "ovw", "projects.json") {
		t.Fatalf("Cache path = %q", paths.Cache)
	}
}

func TestLoadWriteRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := Default()
	cfg.Roots = []string{"~/dev"}
	cfg.StaleDays = 7
	cfg.Stack.Aliases["Cloudflare Workers"] = "Workers"

	if err := Write(path, cfg); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !reflect.DeepEqual(loaded.Roots, cfg.Roots) {
		t.Fatalf("Roots = %#v", loaded.Roots)
	}
	if loaded.StaleDays != 7 {
		t.Fatalf("StaleDays = %d", loaded.StaleDays)
	}
	if loaded.Stack.Aliases["Cloudflare Workers"] != "Workers" {
		t.Fatalf("Alias = %q", loaded.Stack.Aliases["Cloudflare Workers"])
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config not written: %v", err)
	}
}
