package node

import "time"

// Pool 表示一个 GPU 资源池。
type Pool struct {
	PoolID      int64     `json:"pool_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	NodeCount int `json:"node_count"`
}

// PoolNode 表示资源池与节点的关联。
type PoolNode struct {
	PoolID   int64     `json:"pool_id"`
	NodeID   string    `json:"node_id"`
	Hostname string    `json:"hostname"`
	JoinedAt time.Time `json:"joined_at"`
}

// PoolStats 表示资源池的 GPU 资源统计。
type PoolStats struct {
	PoolID int64 `json:"pool_id"`

	NodeCount int `json:"node_count"`
	GPUCount  int `json:"gpu_count"`

	MemoryTotalMB int64 `json:"memory_total_mb"`
	MemoryUsedMB  int64 `json:"memory_used_mb"`
	MemoryFreeMB  int64 `json:"memory_free_mb"`
}
