package gpu

// GPUInfo 描述一张 NVIDIA GPU 的硬件信息和实时状态。
type GPUInfo struct {
	Index         int     `json:"index"`
	UUID          string  `json:"uuid"`
	Name          string  `json:"name"`
	MemoryTotalMB uint64  `json:"memory_total_mb"`
	MemoryUsedMB  uint64  `json:"memory_used_mb"`
	MemoryFreeMB  uint64  `json:"memory_free_mb"`
	Utilization   uint32  `json:"utilization"`
	Temperature   uint32  `json:"temperature"`
	PowerW        float64 `json:"power_w"`
	DriverVersion string  `json:"driver_version"`
}

// GPUList 表示当前节点检测到的全部 GPU。
type GPUList struct {
	GPUs []GPUInfo `json:"gpus"`
}
