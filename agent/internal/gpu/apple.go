package gpu

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/mem"
)

const (
	// powermetrics 采样一次通常 1-2 秒完成，5 秒足够且避免阻塞采集。
	powermetricsTimeout = 5 * time.Second
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

	// 尝试通过 sudo powermetrics 获取 GPU 实时利用率与功耗。
	// 若未配置免密 sudo 或命令不可用，会优雅降级为 0，不影响整体采集。
	utilization, powerW := c.collectPowermetrics()

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
			Utilization:   utilization,
			PowerW:        powerW,

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

// collectPowermetrics 通过 sudo powermetrics 采集 GPU 实时利用率与功耗。
//
// 该命令需要 root 权限，因此使用 `sudo -n`（非交互模式）：
// 若未配置免密 sudo，命令会立即失败而不是卡住等待输入密码。
// 失败时返回零值，由上层决定是否继续。
func (c *AppleCollector) collectPowermetrics() (utilization uint32, powerW float64) {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = powermetricsTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"sudo",
		"-n",
		"powermetrics",
		"--samplers", "gpu_power",
		"-n", "1",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf(
			"powermetrics skipped (need passwordless sudo): %v",
			err,
		)
		return 0, 0
	}

	return parsePowermetricsOutput(string(output))
}

// parsePowermetricsOutput 从 powermetrics 输出中解析 GPU 利用率和功耗。
//
// 当前按单 GPU（Apple Silicon 统一内存架构）处理，读取第一个匹配的
// "GPU HW active residency" 和 "GPU Power" 字段。
func parsePowermetricsOutput(output string) (uint32, float64) {
	var (
		utilization uint32
		powerW      float64
	)

	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		switch {
		case strings.HasPrefix(line, "GPU HW active residency:"):
			if value := parsePercentAfterColon(line); value >= 0 {
				utilization = uint32(math.Round(value))
			}

		case strings.HasPrefix(line, "GPU Power:"):
			if value := parseWattAfterColon(line); value >= 0 {
				powerW = value
			}
		}
	}

	if utilization > 100 {
		utilization = 100
	}

	return utilization, powerW
}

// parsePercentAfterColon 解析形如 "GPU HW active residency: 68.45% (...)"
// 冒号后的百分比数值，解析失败返回 -1。
func parsePercentAfterColon(line string) float64 {
	colon := strings.Index(line, ":")
	if colon < 0 {
		return -1
	}

	rest := strings.TrimSpace(line[colon+1:])

	percentIdx := strings.Index(rest, "%")
	if percentIdx < 0 {
		return -1
	}

	value, err := strconv.ParseFloat(
		strings.TrimSpace(rest[:percentIdx]),
		64,
	)
	if err != nil {
		return -1
	}

	return value
}

// parseWattAfterColon 解析形如 "GPU Power: 3.90 W" 或 "GPU Power: 28 mW"
// 冒号后的功率数值，解析失败返回 -1。
//
// powermetrics 的 GPU Power 单位可能是 W、mW 或 µW，
// 这里统一转换为 W。
func parseWattAfterColon(line string) float64 {
	colon := strings.Index(line, ":")
	if colon < 0 {
		return -1
	}

	rest := strings.TrimSpace(line[colon+1:])

	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return -1
	}

	value, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return -1
	}

	// 根据第二个字段的单位进行换算，默认按 W 处理。
	if len(fields) >= 2 {
		switch strings.ToLower(fields[1]) {
		case "mw":
			value /= 1000
		case "uw", "µw":
			value /= 1000000
		}
	}

	return value
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
