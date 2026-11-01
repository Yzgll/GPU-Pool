package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gpu-pool/agent/internal/gpu"
	"gpu-pool/agent/internal/identity"
)

// AgentSnapshot 表示 Worker Agent 当前节点状态。
type AgentSnapshot struct {
	NodeID      string        `json:"node_id"`
	Hostname    string        `json:"hostname"`
	CollectedAt time.Time     `json:"collected_at"`
	GPUs        []gpu.GPUInfo `json:"gpus"`
}

// getIdentityPath 获取节点身份文件路径。
func getIdentityPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("get user config directory: %w", err)
	}

	return filepath.Join(configDir, "gpu-pool", "node.json"), nil
}

func main() {
	// 1. 获取本机节点身份文件路径。
	identityPath, err := getIdentityPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Identity path error: %v\n", err)
		os.Exit(1)
	}

	// 2. 加载或创建节点身份。
	nodeIdentity, err := identity.LoadOrCreate(identityPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Identity initialization failed: %v\n", err)
		os.Exit(1)
	}

	// 3. 获取本机主机名。
	hostname, err := os.Hostname()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Get hostname failed: %v\n", err)
		os.Exit(1)
	}

	// 4. 创建 NVIDIA GPU 采集器。
	collector := gpu.NewNVIDIACollector()

	// 5. 采集 GPU 信息。
	gpus, err := collector.Collect()
	if err != nil {
		fmt.Fprintf(os.Stderr, "GPU collection failed: %v\n", err)
		os.Exit(1)
	}

	// 6. 组合完整节点信息。
	snapshot := AgentSnapshot{
		NodeID:      nodeIdentity.NodeID,
		Hostname:    hostname,
		CollectedAt: time.Now().UTC(),
		GPUs:        gpus,
	}

	// 7. 输出 JSON。
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(snapshot); err != nil {
		fmt.Fprintf(os.Stderr, "JSON encoding failed: %v\n", err)
		os.Exit(1)
	}
}
