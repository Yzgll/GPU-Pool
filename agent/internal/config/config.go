package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 是 Worker Agent 的完整配置。
type Config struct {
	Agent       AgentConfig       `yaml:"agent"`
	Coordinator CoordinatorConfig `yaml:"coordinator"`
	Monitor     MonitorConfig     `yaml:"monitor"`
	Heartbeat   HeartbeatConfig   `yaml:"heartbeat"`
	Storage     StorageConfig     `yaml:"storage"`
}

type AgentConfig struct {
	NodeName string `yaml:"node_name"`
	DataDir  string `yaml:"data_dir"`
}

type CoordinatorConfig struct {
	Endpoint string `yaml:"endpoint"`
}

type MonitorConfig struct {
	CollectionInterval time.Duration `yaml:"collection_interval"`
}

type HeartbeatConfig struct {
	Interval time.Duration `yaml:"interval"`
}

type StorageConfig struct {
	ModelCacheDir string `yaml:"model_cache_dir"`
}

// DefaultConfig 返回默认配置。
func DefaultConfig() Config {
	dataDir := defaultDataDir()

	return Config{
		Agent: AgentConfig{
			DataDir: dataDir,
		},
		Coordinator: CoordinatorConfig{
			Endpoint: "http://127.0.0.1:8080",
		},
		Monitor: MonitorConfig{
			CollectionInterval: 5 * time.Second,
		},
		Heartbeat: HeartbeatConfig{
			Interval: 10 * time.Second,
		},
		Storage: StorageConfig{
			ModelCacheDir: filepath.Join(dataDir, "models"),
		},
	}
}

// defaultDataDir 返回当前用户的 GPU-Pool 数据目录。
func defaultDataDir() string {
	return `E:\GPU-Pool-Data`
}

// Load 从 YAML 文件加载配置。
// 文件不存在时，自动生成默认配置文件。
func Load(path string) (*Config, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("config path cannot be empty")
	}

	expandedPath, err := expandPath(path)
	if err != nil {
		return nil, fmt.Errorf("expand config path: %w", err)
	}

	data, err := os.ReadFile(expandedPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("read config file: %w", err)
		}

		cfg := DefaultConfig()

		if err := cfg.Validate(); err != nil {
			return nil, err
		}

		if err := Save(expandedPath, &cfg); err != nil {
			return nil, fmt.Errorf("create default config: %w", err)
		}

		return &cfg, nil
	}

	cfg := DefaultConfig()

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config YAML: %w", err)
	}

	if err := cfg.ExpandPaths(); err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Save 将配置保存为 YAML 文件。
func Save(path string, cfg *Config) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("config path cannot be empty")
	}

	if cfg == nil {
		return errors.New("config cannot be nil")
	}

	if err := cfg.ExpandPaths(); err != nil {
		return err
	}

	if err := cfg.Validate(); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	expandedPath, err := expandPath(path)
	if err != nil {
		return fmt.Errorf("expand config path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(expandedPath), 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	if err := os.WriteFile(expandedPath, data, 0600); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	return nil
}

// ExpandPaths 展开配置中的环境变量和用户目录。
func (c *Config) ExpandPaths() error {
	var err error

	c.Agent.DataDir, err = expandPath(c.Agent.DataDir)
	if err != nil {
		return fmt.Errorf("expand agent data directory: %w", err)
	}

	c.Storage.ModelCacheDir, err = expandPath(c.Storage.ModelCacheDir)
	if err != nil {
		return fmt.Errorf("expand model cache directory: %w", err)
	}

	return nil
}

// Validate 检查配置是否合法。
func (c *Config) Validate() error {
	if c == nil {
		return errors.New("config cannot be nil")
	}

	if strings.TrimSpace(c.Agent.DataDir) == "" {
		return errors.New("agent.data_dir cannot be empty")
	}

	if strings.TrimSpace(c.Coordinator.Endpoint) == "" {
		return errors.New("coordinator.endpoint cannot be empty")
	}

	if c.Monitor.CollectionInterval <= 0 {
		return errors.New("monitor.collection_interval must be greater than zero")
	}

	if c.Heartbeat.Interval <= 0 {
		return errors.New("heartbeat.interval must be greater than zero")
	}

	if strings.TrimSpace(c.Storage.ModelCacheDir) == "" {
		return errors.New("storage.model_cache_dir cannot be empty")
	}

	return nil
}

// expandPath 展开 Windows 和 Linux 环境变量。
func expandPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}

	path = os.ExpandEnv(path)

	// 支持 ~ 和 ~/path。
	if path == "~" || strings.HasPrefix(path, "~/") ||
		strings.HasPrefix(path, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("get user home directory: %w", err)
		}

		if path == "~" {
			path = home
		} else {
			path = filepath.Join(home, path[2:])
		}
	}

	return filepath.Clean(path), nil
}
