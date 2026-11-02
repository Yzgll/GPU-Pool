package agent

import (
	"fmt"
	"time"

	"gpu-pool/agent/internal/gpu"
	"gpu-pool/agent/internal/monitor"
)

// Snapshot represents the current worker node state.
type Snapshot struct {
	NodeID      string             `json:"node_id"`
	Hostname    string             `json:"hostname"`
	CollectedAt time.Time          `json:"collected_at"`
	System      monitor.SystemInfo `json:"system"`
	GPUs        []gpu.GPUInfo      `json:"gpus"`
}

// GPUCollector defines the GPU collection interface.
type GPUCollector interface {
	Collect() ([]gpu.GPUInfo, error)
}

// Collector collects system and GPU resources.
type Collector struct {
	nodeID       string
	hostname     string
	gpuCollector GPUCollector
}

// NewCollector creates a resource collector.
func NewCollector(
	nodeID string,
	hostname string,
	gpuCollector GPUCollector,
) *Collector {
	return &Collector{
		nodeID:       nodeID,
		hostname:     hostname,
		gpuCollector: gpuCollector,
	}
}

// Collect collects one complete resource snapshot.
func (c *Collector) Collect() (*Snapshot, error) {
	if c == nil {
		return nil, fmt.Errorf("collector cannot be nil")
	}

	if c.gpuCollector == nil {
		return nil, fmt.Errorf("GPU collector cannot be nil")
	}

	gpus, err := c.gpuCollector.Collect()
	if err != nil {
		return nil, fmt.Errorf("collect GPU information: %w", err)
	}

	systemInfo, err := monitor.CollectSystemInfo()
	if err != nil {
		return nil, fmt.Errorf("collect system information: %w", err)
	}

	snapshot := &Snapshot{
		NodeID:      c.nodeID,
		Hostname:    c.hostname,
		CollectedAt: time.Now().UTC(),
		System:      *systemInfo,
		GPUs:        gpus,
	}

	return snapshot, nil
}
