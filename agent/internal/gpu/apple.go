package gpu

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/mem"
)

type AppleCollector struct {
	Timeout time.Duration
}

func NewAppleCollector() *AppleCollector {
	return &AppleCollector{
		Timeout: 10 * time.Second,
	}
}

func (c *AppleCollector) Collect() ([]GPUInfo, error) {
	if runtime.GOOS != "darwin" {
		return nil, fmt.Errorf("Apple GPU collector requires macOS")
	}

	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"system_profiler",
		"SPDisplaysDataType",
		"-json",
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("system_profiler failed: %w", err)
	}

	var result struct {
		Displays []struct {
			Name     string `json:"sppci_model"`
			Chipset  string `json:"sppci_vendor"`
			Metal    string `json:"spdisplays_metal"`
			DeviceID string `json:"spdisplays_device-id"`
		} `json:"SPDisplaysDataType"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("parse Apple GPU information: %w", err)
	}

	if len(result.Displays) == 0 {
		return nil, fmt.Errorf("no Apple GPU detected")
	}

	memory, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("collect unified memory: %w", err)
	}

	hostname, _ := os.Hostname()
	gpus := make([]GPUInfo, 0, len(result.Displays))

	for _, device := range result.Displays {
		name := strings.TrimSpace(device.Name)
		if name == "" {
			name = "Apple GPU"
		}

		gpu := GPUInfo{
			Index:      len(gpus),
			UUID:       fmt.Sprintf("apple-%s-%d", hostname, len(gpus)),
			Name:       name,
			Vendor:     "Apple",
			Backend:    "Metal",
			MemoryType: "unified",

			// Unified memory is shared with the CPU.
			// Do not treat all system memory as allocatable GPU memory.
			MemoryTotalMB: memory.Total / 1024 / 1024,
			MemoryUsedMB:  memory.Used / 1024 / 1024,
			MemoryFreeMB:  memory.Available / 1024 / 1024,

			// Safe Metal allocation budget is not known yet.
			MemoryAvailableKnown: false,
		}

		gpus = append(gpus, gpu)
	}

	return gpus, nil
}
