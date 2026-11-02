package node

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

// 节点超过该时间没有心跳，就判定为离线。
const OfflineAfter = 30 * time.Second

// NodeStatus 表示节点当前在线状态。
type NodeStatus string

const (
	StatusOnline  NodeStatus = "online"
	StatusOffline NodeStatus = "offline"
)

// Node 表示一个注册到 Coordinator 的 Worker 节点。
type Node struct {
	NodeID        string          `json:"node_id"`
	Hostname      string          `json:"hostname"`
	CollectedAt   time.Time       `json:"collected_at"`
	LastHeartbeat time.Time       `json:"last_heartbeat"`
	Status        NodeStatus      `json:"status"`
	Snapshot      json.RawMessage `json:"snapshot"`
}

// NodeStats 表示节点在线状态统计。
type NodeStats struct {
	Total   int `json:"total"`
	Online  int `json:"online"`
	Offline int `json:"offline"`
}

// Registry 负责管理所有已注册节点。
type Registry struct {
	mu    sync.RWMutex
	nodes map[string]Node
}

// NewRegistry 创建一个内存节点注册表。
func NewRegistry() *Registry {
	return &Registry{
		nodes: make(map[string]Node),
	}
}

// Upsert 注册节点或更新已有节点的心跳信息。
func (r *Registry) Upsert(snapshot []byte) (*Node, error) {
	if r == nil {
		return nil, fmt.Errorf("node registry cannot be nil")
	}

	var metadata struct {
		NodeID      string    `json:"node_id"`
		Hostname    string    `json:"hostname"`
		CollectedAt time.Time `json:"collected_at"`
	}

	if err := json.Unmarshal(snapshot, &metadata); err != nil {
		return nil, fmt.Errorf("invalid heartbeat JSON: %w", err)
	}

	if metadata.NodeID == "" {
		return nil, fmt.Errorf("node_id cannot be empty")
	}

	if metadata.Hostname == "" {
		return nil, fmt.Errorf("hostname cannot be empty")
	}

	if metadata.CollectedAt.IsZero() {
		return nil, fmt.Errorf("collected_at cannot be empty")
	}

	n := Node{
		NodeID:        metadata.NodeID,
		Hostname:      metadata.Hostname,
		CollectedAt:   metadata.CollectedAt,
		LastHeartbeat: time.Now().UTC(),
		Status:        StatusOnline,
		Snapshot:      append(json.RawMessage(nil), snapshot...),
	}

	r.mu.Lock()
	r.nodes[n.NodeID] = n
	r.mu.Unlock()

	return &n, nil
}

// GetAll 返回所有节点，并根据最后心跳时间计算在线状态。
func (r *Registry) GetAll() []Node {
	if r == nil {
		return []Node{}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	nodes := make([]Node, 0, len(r.nodes))

	for _, n := range r.nodes {
		n.Status = calculateStatus(n.LastHeartbeat, time.Now())
		nodes = append(nodes, n)
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].NodeID < nodes[j].NodeID
	})

	return nodes
}

// GetByID 根据节点 ID 查询节点详情。
func (r *Registry) GetByID(nodeID string) (*Node, bool) {
	if r == nil {
		return nil, false
	}

	r.mu.RLock()
	n, exists := r.nodes[nodeID]
	r.mu.RUnlock()

	if !exists {
		return nil, false
	}

	n.Status = calculateStatus(n.LastHeartbeat, time.Now())

	return &n, true
}

// GetStats 统计当前节点总数及在线、离线数量。
func (r *Registry) GetStats() NodeStats {
	if r == nil {
		return NodeStats{}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := NodeStats{
		Total: len(r.nodes),
	}

	now := time.Now()

	for _, n := range r.nodes {
		status := calculateStatus(n.LastHeartbeat, now)

		switch status {
		case StatusOnline:
			stats.Online++
		case StatusOffline:
			stats.Offline++
		}
	}

	return stats
}

// calculateStatus 根据最后心跳时间判断节点是否在线。
func calculateStatus(lastHeartbeat, now time.Time) NodeStatus {
	if lastHeartbeat.IsZero() || now.Sub(lastHeartbeat) > OfflineAfter {
		return StatusOffline
	}

	return StatusOnline
}
