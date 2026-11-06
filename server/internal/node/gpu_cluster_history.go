package node

import "time"

type GPUClusterHistoryPoint struct {
	CollectedAt    time.Time `json:"collected_at"`
	GPUCount       int       `json:"gpu_count"`
	AvgUtilization float64   `json:"avg_utilization"`
	MemoryUsedMB   int64     `json:"memory_used_mb"`
	MemoryTotalMB  int64     `json:"memory_total_mb"`
	AvgTemperature float64   `json:"avg_temperature"`
	TotalPowerW    float64   `json:"total_power_w"`
}

type GPUClusterHistoryResult struct {
	Range           string                   `json:"range"`
	IntervalSeconds int                      `json:"interval_seconds"`
	Points          []GPUClusterHistoryPoint `json:"points"`
}
type GPUCompareDevice struct {
	NodeID   string `json:"node_id"`
	Hostname string `json:"hostname"`
	GPUIndex int    `json:"gpu_index"`
	GPUName  string `json:"gpu_name"`
}

type GPUComparePoint struct {
	CollectedAt  time.Time `json:"collected_at"`
	NodeID       string    `json:"node_id"`
	GPUIndex     int       `json:"gpu_index"`
	Utilization  float64   `json:"utilization"`
	MemoryUsedMB int64     `json:"memory_used_mb"`
	Temperature  float64   `json:"temperature"`
}

type GPUCompareResult struct {
	Range           string             `json:"range"`
	IntervalSeconds int                `json:"interval_seconds"`
	Devices         []GPUCompareDevice `json:"devices"`
	Points          []GPUComparePoint  `json:"points"`
}
