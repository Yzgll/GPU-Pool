package main

import (
	"encoding/json"
	"fmt"
	"os"

	"gpu-pool/agent/internal/gpu"
)

func main() {
	// 创建 NVIDIA GPU 采集器
	collector := gpu.NewNVIDIACollector()

	// 采集 GPU 信息
	gpus, err := collector.Collect()
	if err != nil {
		fmt.Fprintf(os.Stderr, "GPU collection failed: %v\n", err)
		os.Exit(1)
	}

	// 封装 GPU 数据
	result := gpu.GPUList{
		GPUs: gpus,
	}

	// JSON 格式化输出
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "JSON encoding failed: %v\n", err)
		os.Exit(1)
	}
}
