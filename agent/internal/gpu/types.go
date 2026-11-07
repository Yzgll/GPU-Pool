package gpu

// GPUInfo describes a GPU device and its resource state.
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

	// Cross-platform GPU metadata.
	Vendor     string `json:"vendor,omitempty"`
	Backend    string `json:"backend,omitempty"`
	MemoryType string `json:"memory_type,omitempty"`

	// Memory budget available for model allocation.
	// This is not necessarily the same as free device memory.
	MemoryAvailableMB    uint64 `json:"memory_available_mb,omitempty"`
	MemoryAvailableKnown bool   `json:"memory_available_known"`
}

// GPUList represents all GPUs detected on a node.
type GPUList struct {
	GPUs []GPUInfo `json:"gpus"`
}
