package monitor

// CPUInfo 描述 CPU 硬件信息和使用情况。
type CPUInfo struct {
	ModelName    string  `json:"model_name"`
	LogicalCores int     `json:"logical_cores"`
	UsagePercent float64 `json:"usage_percent"`
}

// MemoryInfo 描述系统物理内存使用情况。
type MemoryInfo struct {
	TotalBytes     uint64  `json:"total_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	UsedPercent    float64 `json:"used_percent"`
}

// DiskInfo 描述一个磁盘分区。
type DiskInfo struct {
	Device      string  `json:"device"`
	MountPoint  string  `json:"mount_point"`
	FileSystem  string  `json:"file_system"`
	TotalBytes  uint64  `json:"total_bytes"`
	UsedBytes   uint64  `json:"used_bytes"`
	FreeBytes   uint64  `json:"free_bytes"`
	UsedPercent float64 `json:"used_percent"`
}

// NetworkInfo 描述一张网络接口的累计流量。
type NetworkInfo struct {
	Name      string `json:"name"`
	BytesSent uint64 `json:"bytes_sent"`
	BytesRecv uint64 `json:"bytes_recv"`
}

// SystemInfo 描述当前节点的系统资源。
type SystemInfo struct {
	CPU      CPUInfo       `json:"cpu"`
	Memory   MemoryInfo    `json:"memory"`
	Disks    []DiskInfo    `json:"disks"`
	Networks []NetworkInfo `json:"networks"`
}
