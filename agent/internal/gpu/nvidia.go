package gpu

import (
	"context"
	"encoding/csv"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	nvidiaSMITimeout = 5 * time.Second
)

// NVIDIACollector 负责采集 NVIDIA GPU 信息。
type NVIDIACollector struct {
	Timeout time.Duration
}

// NewNVIDIACollector 创建 NVIDIA GPU 采集器。
func NewNVIDIACollector() *NVIDIACollector {
	return &NVIDIACollector{
		Timeout: nvidiaSMITimeout,
	}
}

// Collect 获取本机全部 NVIDIA GPU 信息。
func (c *NVIDIACollector) Collect() ([]GPUInfo, error) {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = nvidiaSMITimeout
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		timeout,
	)
	defer cancel()

	args := []string{
		"--query-gpu=index,uuid,name,memory.total,memory.used,memory.free,utilization.gpu,temperature.gpu,power.draw,driver_version",
		"--format=csv,noheader,nounits",
	}

	cmd := exec.CommandContext(ctx, "nvidia-smi", args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("nvidia-smi timed out after %s", timeout)
		}

		return nil, fmt.Errorf(
			"nvidia-smi execution failed: %w, output: %s",
			err,
			strings.TrimSpace(string(output)),
		)
	}

	gpus, err := parseNVIDIAOutput(string(output))
	if err != nil {
		return nil, fmt.Errorf("parse NVIDIA output: %w", err)
	}

	if len(gpus) == 0 {
		return nil, fmt.Errorf("no NVIDIA GPU detected")
	}

	return gpus, nil
}

// parseNVIDIAOutput 将 NVIDIA-SMI CSV 输出解析为 GPUInfo。
func parseNVIDIAOutput(output string) ([]GPUInfo, error) {
	reader := csv.NewReader(strings.NewReader(output))
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = 10

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("invalid CSV: %w", err)
	}

	gpus := make([]GPUInfo, 0, len(records))

	for rowIndex, record := range records {
		if len(record) != 10 {
			return nil, fmt.Errorf(
				"row %d: expected 10 fields, got %d",
				rowIndex+1,
				len(record),
			)
		}

		for i := range record {
			record[i] = strings.TrimSpace(record[i])
		}

		index, err := strconv.Atoi(record[0])
		if err != nil {
			return nil, fmt.Errorf("row %d: invalid GPU index: %w", rowIndex+1, err)
		}

		total, err := parseRequiredUint(record[3], "memory.total")
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", rowIndex+1, err)
		}

		used, err := parseRequiredUint(record[4], "memory.used")
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", rowIndex+1, err)
		}

		free, err := parseRequiredUint(record[5], "memory.free")
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", rowIndex+1, err)
		}

		if used > total || free > total {
			return nil, fmt.Errorf("row %d: invalid GPU memory values", rowIndex+1)
		}

		utilization, err := parseOptionalUint(record[6])
		if err != nil {
			return nil, fmt.Errorf("row %d: invalid GPU utilization: %w", rowIndex+1, err)
		}

		temperature, err := parseOptionalUint(record[7])
		if err != nil {
			return nil, fmt.Errorf("row %d: invalid GPU temperature: %w", rowIndex+1, err)
		}

		power, err := parseOptionalFloat(record[8])
		if err != nil {
			return nil, fmt.Errorf("row %d: invalid GPU power: %w", rowIndex+1, err)
		}

		if utilization > 100 {
			return nil, fmt.Errorf("row %d: GPU utilization exceeds 100%%", rowIndex+1)
		}

		gpu := GPUInfo{
			Index:         index,
			UUID:          record[1],
			Name:          record[2],
			MemoryTotalMB: total,
			MemoryUsedMB:  used,
			MemoryFreeMB:  free,
			Utilization:   uint32(utilization),
			Temperature:   uint32(temperature),
			PowerW:        power,
			DriverVersion: record[9],

			// 跨平台元数据：NVIDIA 独立显存，可分配预算等于空闲显存。
			Vendor:               "NVIDIA",
			Backend:              "CUDA",
			MemoryType:           "dedicated",
			MemoryAvailableMB:    free,
			MemoryAvailableKnown: true,
		}

		gpus = append(gpus, gpu)
	}

	return gpus, nil
}

// parseRequiredUint 解析必须存在的整数。
func parseRequiredUint(value string, field string) (uint64, error) {
	value = strings.TrimSpace(value)

	result, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", field, err)
	}

	return result, nil
}

// parseOptionalUint 解析可选整数。
// NVIDIA-SMI 返回 N/A 时使用 0 表示暂不可用。
func parseOptionalUint(value string) (uint64, error) {
	value = strings.TrimSpace(value)

	if value == "" || strings.EqualFold(value, "N/A") {
		return 0, nil
	}

	return strconv.ParseUint(value, 10, 32)
}

// parseOptionalFloat 解析可选浮点数。
func parseOptionalFloat(value string) (float64, error) {
	value = strings.TrimSpace(value)

	if value == "" || strings.EqualFold(value, "N/A") {
		return 0, nil
	}

	return strconv.ParseFloat(value, 64)
}
