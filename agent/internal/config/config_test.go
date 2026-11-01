package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Coordinator.Endpoint == "" {
		t.Error("coordinator endpoint should not be empty")
	}

	if cfg.Monitor.CollectionInterval != 5*time.Second {
		t.Errorf("unexpected collection interval: %v", cfg.Monitor.CollectionInterval)
	}

	if cfg.Heartbeat.Interval != 10*time.Second {
		t.Errorf("unexpected heartbeat interval: %v", cfg.Heartbeat.Interval)
	}

	if cfg.Agent.DataDir == "" {
		t.Error("agent data directory should not be empty")
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	cfg := DefaultConfig()
	cfg.Agent.NodeName = "test-node"
	cfg.Coordinator.Endpoint = "http://127.0.0.1:9090"

	if err := Save(path, &cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Agent.NodeName != "test-node" {
		t.Errorf("unexpected node name: %s", loaded.Agent.NodeName)
	}

	if loaded.Coordinator.Endpoint != "http://127.0.0.1:9090" {
		t.Errorf("unexpected endpoint: %s", loaded.Coordinator.Endpoint)
	}
}

func TestLoadCreatesDefaultConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new", "config.yaml")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg == nil {
		t.Fatal("config should not be nil")
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file was not created: %v", err)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.yaml")

	if err := os.WriteFile(path, []byte("agent: [invalid"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected YAML parsing error")
	}
}

func TestValidateInvalidConfig(t *testing.T) {
	tests := []struct {
		name   string
		modify func(*Config)
	}{
		{
			name: "empty data directory",
			modify: func(c *Config) {
				c.Agent.DataDir = ""
			},
		},
		{
			name: "empty endpoint",
			modify: func(c *Config) {
				c.Coordinator.Endpoint = ""
			},
		},
		{
			name: "invalid collection interval",
			modify: func(c *Config) {
				c.Monitor.CollectionInterval = 0
			},
		},
		{
			name: "invalid heartbeat interval",
			modify: func(c *Config) {
				c.Heartbeat.Interval = -time.Second
			},
		},
		{
			name: "empty model cache directory",
			modify: func(c *Config) {
				c.Storage.ModelCacheDir = ""
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.modify(&cfg)

			if err := cfg.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestExpandPath(t *testing.T) {
	t.Setenv("GPU_POOL_TEST_DIR", "D:\\GPU-Pool")

	got, err := expandPath("$GPU_POOL_TEST_DIR/models")
	if err != nil {
		t.Fatalf("expandPath failed: %v", err)
	}

	if !strings.Contains(got, "GPU-Pool") {
		t.Errorf("environment variable was not expanded: %s", got)
	}
}

func TestLoadEmptyPath(t *testing.T) {
	_, err := Load("")
	if err == nil {
		t.Fatal("expected error for empty config path")
	}
}
