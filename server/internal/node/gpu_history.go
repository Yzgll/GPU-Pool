package node

import "time"

// GPUHistoryRecord 表示一条 GPU 历史资源记录。
type GPUHistoryRecord struct {
	ID       int64  `json:"id"`
	NodeID   string `json:"node_id"`
	Hostname string `json:"hostname"`
	GPUUUID  string `json:"gpu_uuid"`
	GPUIndex int    `json:"gpu_index"`
	Name     string `json:"name"`

	MemoryTotalMB int64 `json:"memory_total_mb"`
	MemoryUsedMB  int64 `json:"memory_used_mb"`
	MemoryFreeMB  int64 `json:"memory_free_mb"`

	Utilization uint32  `json:"utilization"`
	Temperature uint32  `json:"temperature"`
	PowerW      float64 `json:"power_w"`

	DriverVersion string    `json:"driver_version"`
	CollectedAt   time.Time `json:"collected_at"`
}

// GPUHistoryResult 表示分页查询的 GPU 历史结果。
type GPUHistoryResult struct {
	Total   int                `json:"total"`
	Limit   int                `json:"limit"`
	Offset  int                `json:"offset"`
	Records []GPUHistoryRecord `json:"records"`
}
