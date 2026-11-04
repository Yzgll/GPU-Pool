package node

import (
	"encoding/json"
	"time"
)

// ResourceSnapshot 表示一次完整的资源采集记录。
type ResourceSnapshot struct {
	ID          int64           `json:"id"`
	NodeID      string          `json:"node_id"`
	CollectedAt time.Time       `json:"collected_at"`
	SystemInfo  json.RawMessage `json:"system_info"`
	GPUInfo     json.RawMessage `json:"gpu_info"`
}

// ResourceHistory 表示分页查询的资源历史结果。
type ResourceHistory struct {
	Total   int                `json:"total"`
	Limit   int                `json:"limit"`
	Offset  int                `json:"offset"`
	Records []ResourceSnapshot `json:"records"`
}
