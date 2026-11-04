package node

import "time"

// GPUDevice 表示 GPU 硬件及当前资源状态。
type GPUDevice struct {
	NodeID        string    `json:"node_id"`
	Hostname      string    `json:"hostname"`
	GPUUUID       string    `json:"gpu_uuid"`
	GPUIndex      int       `json:"gpu_index"`
	Name          string    `json:"name"`
	MemoryTotalMB int64     `json:"memory_total_mb"`
	MemoryUsedMB  int64     `json:"memory_used_mb"`
	MemoryFreeMB  int64     `json:"memory_free_mb"`
	Utilization   uint32    `json:"utilization"`
	Temperature   uint32    `json:"temperature"`
	PowerW        float64   `json:"power_w"`
	DriverVersion string    `json:"driver_version"`
	CollectedAt   time.Time `json:"collected_at"`
}

// GPUStats 表示 GPU 资源统计。
type GPUStats struct {
	TotalGPUs          int     `json:"total_gpus"`
	TotalMemoryMB      int64   `json:"total_memory_mb"`
	UsedMemoryMB       int64   `json:"used_memory_mb"`
	FreeMemoryMB       int64   `json:"free_memory_mb"`
	AverageUtilization float64 `json:"average_utilization"`
}
