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

	output, err := exec.CommandContext(
		ctx,
		"system_profiler",
		"SPDisplaysDataType",
		"-json",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("system_profiler failed: %w", err)
	}

	devices, err := parseAppleOutput(string(output))
	if err != nil {
		return nil, err
	}

	if len(devices) == 0 {
		return nil, fmt.Errorf("no Apple GPU detected")
	}

	memory, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("collect unified memory: %w", err)
	}

	hostname, _ := os.Hostname()
	gpus := make([]GPUInfo, 0, len(devices))

	for i, device := range devices {
		gpus = append(gpus, GPUInfo{
			Index:         i,
			UUID:          fmt.Sprintf("apple-%s-%d", hostname, i),
			Name:          device.Name,
			Vendor:        device.Vendor,
			Backend:       "Metal",
			MemoryType:    "unified",
			DriverVersion: device.Metal,

			// Apple Silicon 采用统一内存架构，CPU 与 GPU 共享同一块物理内存，
			// 因此没有独立“显存”概念。这里记录统一内存总量，
			// 并通过 MemoryType=unified 与 MemoryAvailableKnown=false
			// 明确告知消费方：该值不等于可分配给模型的独立显存。
			MemoryTotalMB: memory.Total / 1024 / 1024,
			MemoryUsedMB:  memory.Used / 1024 / 1024,
			MemoryFreeMB:  memory.Available / 1024 / 1024,

			// 精确的可分配显存上限需 root 权限读取 iogpu.wired_limit_mb，
			// 普通用户无法获取，标记为未知。
			MemoryAvailableKnown: false,
		})
	}

	return gpus, nil
}

// appleGPUDevice 是 system_profiler SPDisplaysDataType -json 输出中的单块 GPU。
type appleGPUDevice struct {
	Name   string
	Vendor string
	Metal  string
}

// parseAppleOutput 解析 system_profiler 的 JSON 输出。
func parseAppleOutput(output string) ([]appleGPUDevice, error) {
	var result struct {
		Displays []struct {
			Name   string `json:"sppci_model"`
			Vendor string `json:"spdisplays_vendor"`
			Metal  string `json:"spdisplays_mtlgpufamilysupport"`
		} `json:"SPDisplaysDataType"`
	}

	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return nil, fmt.Errorf("parse Apple GPU information: %w", err)
	}

	devices := make([]appleGPUDevice, 0, len(result.Displays))

	for _, d := range result.Displays {
		name := strings.TrimSpace(d.Name)
		if name == "" {
			name = "Apple GPU"
		}

		devices = append(devices, appleGPUDevice{
			Name:   name,
			Vendor: normalizeAppleEnum(d.Vendor, "Apple"),
			Metal:  normalizeAppleMetal(d.Metal),
		})
	}

	return devices, nil
}

// normalizeAppleEnum 把形如 sppci_vendor_Apple 的枚举值规范化为 Apple。
// 找不到分隔符时返回 fallback。
func normalizeAppleEnum(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}

	if i := strings.LastIndex(value, "_"); i >= 0 && i+1 < len(value) {
		return value[i+1:]
	}

	return value
}

// normalizeAppleMetal 把形如 spdisplays_metal4 的枚举值规范化为 Metal 4。
// 该值来自 spdisplays_mtlgpufamilysupport 字段，
// 表示 GPU 支持的 Metal 版本，是 Apple GPU 最接近“驱动版本”的能力标识。
func normalizeAppleMetal(metal string) string {
	metal = strings.TrimSpace(metal)
	if metal == "" {
		return ""
	}

	// 去掉枚举前缀，如 spdisplays_metal4 -> metal4。
	if i := strings.LastIndex(metal, "_"); i >= 0 && i+1 < len(metal) {
		metal = metal[i+1:]
	}

	// metal4 -> Metal 4。
	if strings.HasPrefix(strings.ToLower(metal), "metal") {
		version := metal[len("metal"):]
		if version == "" {
			return "Metal"
		}
		return "Metal " + version
	}

	return metal
}
