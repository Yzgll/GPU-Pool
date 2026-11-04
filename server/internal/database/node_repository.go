package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gpu-pool/server/internal/node"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NodeRepository 负责节点及资源快照的数据库操作。
type NodeRepository struct {
	pool *pgxpool.Pool
}

// NewNodeRepository 创建节点 Repository。
func NewNodeRepository(pool *pgxpool.Pool) (*NodeRepository, error) {
	if pool == nil {
		return nil, fmt.Errorf("database pool cannot be nil")
	}

	return &NodeRepository{pool: pool}, nil
}

// UpsertNode 注册节点或更新节点最新状态。
func (r *NodeRepository) UpsertNode(
	ctx context.Context,
	snapshot []byte,
) (*node.Node, error) {
	if r == nil || r.pool == nil {
		return nil, fmt.Errorf("node repository unavailable")
	}

	var metadata struct {
		NodeID      string    `json:"node_id"`
		Hostname    string    `json:"hostname"`
		CollectedAt time.Time `json:"collected_at"`
	}

	if err := json.Unmarshal(snapshot, &metadata); err != nil {
		return nil, fmt.Errorf("invalid heartbeat JSON: %w", err)
	}

	if metadata.NodeID == "" || metadata.Hostname == "" || metadata.CollectedAt.IsZero() {
		return nil, fmt.Errorf("invalid heartbeat metadata")
	}

	var result node.Node

	err := r.pool.QueryRow(ctx, `
		INSERT INTO nodes (
			node_id,
			hostname,
			status,
			collected_at,
			last_heartbeat,
			latest_snapshot
		)
		VALUES ($1, $2, 'online', $3, NOW(), $4::jsonb)
		ON CONFLICT (node_id) DO UPDATE SET
			hostname = EXCLUDED.hostname,
			status = 'online',
			collected_at = EXCLUDED.collected_at,
			last_heartbeat = NOW(),
			latest_snapshot = EXCLUDED.latest_snapshot,
			updated_at = NOW()
		RETURNING
			node_id,
			hostname,
			collected_at,
			last_heartbeat,
			status,
			latest_snapshot
	`,
		metadata.NodeID,
		metadata.Hostname,
		metadata.CollectedAt,
		string(snapshot),
	).Scan(
		&result.NodeID,
		&result.Hostname,
		&result.CollectedAt,
		&result.LastHeartbeat,
		&result.Status,
		&result.Snapshot,
	)

	if err != nil {
		return nil, fmt.Errorf("upsert node: %w", err)
	}

	return &result, nil
}

// GetNode 根据节点 ID 查询节点。
func (r *NodeRepository) GetNode(
	ctx context.Context,
	nodeID string,
) (*node.Node, error) {
	if r == nil || r.pool == nil {
		return nil, fmt.Errorf("node repository unavailable")
	}

	var result node.Node

	err := r.pool.QueryRow(ctx, `
		SELECT
			node_id,
			hostname,
			collected_at,
			last_heartbeat,
			status,
			latest_snapshot
		FROM nodes
		WHERE node_id = $1
	`, nodeID).Scan(
		&result.NodeID,
		&result.Hostname,
		&result.CollectedAt,
		&result.LastHeartbeat,
		&result.Status,
		&result.Snapshot,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("get node: %w", err)
	}

	if time.Since(result.LastHeartbeat) > node.OfflineAfter {
		result.Status = node.StatusOffline
	} else {
		result.Status = node.StatusOnline
	}

	return &result, nil
}

// ListNodes 查询全部节点。
func (r *NodeRepository) ListNodes(
	ctx context.Context,
) ([]node.Node, error) {
	if r == nil || r.pool == nil {
		return nil, fmt.Errorf("node repository unavailable")
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			node_id,
			hostname,
			collected_at,
			last_heartbeat,
			status,
			latest_snapshot
		FROM nodes
		ORDER BY node_id
	`)
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	defer rows.Close()

	nodes := make([]node.Node, 0)

	now := time.Now()

	for rows.Next() {
		var result node.Node

		if err := rows.Scan(
			&result.NodeID,
			&result.Hostname,
			&result.CollectedAt,
			&result.LastHeartbeat,
			&result.Status,
			&result.Snapshot,
		); err != nil {
			return nil, fmt.Errorf("scan node: %w", err)
		}

		if now.Sub(result.LastHeartbeat) > node.OfflineAfter {
			result.Status = node.StatusOffline
		} else {
			result.Status = node.StatusOnline
		}

		nodes = append(nodes, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate nodes: %w", err)
	}

	return nodes, nil
}

// GetNodeStats 统计节点总数及在线、离线数量。
func (r *NodeRepository) GetNodeStats(
	ctx context.Context,
) (node.NodeStats, error) {
	nodes, err := r.ListNodes(ctx)
	if err != nil {
		return node.NodeStats{}, err
	}

	stats := node.NodeStats{
		Total: len(nodes),
	}

	for _, n := range nodes {
		switch n.Status {
		case node.StatusOnline:
			stats.Online++
		case node.StatusOffline:
			stats.Offline++
		}
	}

	return stats, nil
}

// SaveSnapshot 保存一条资源历史快照。
func (r *NodeRepository) SaveSnapshot(
	ctx context.Context,
	snapshot []byte,
) error {
	if r == nil || r.pool == nil {
		return fmt.Errorf("node repository unavailable")
	}

	var metadata struct {
		NodeID      string          `json:"node_id"`
		CollectedAt time.Time       `json:"collected_at"`
		System      json.RawMessage `json:"system"`
		GPUs        json.RawMessage `json:"gpus"`
	}

	if err := json.Unmarshal(snapshot, &metadata); err != nil {
		return fmt.Errorf("invalid snapshot JSON: %w", err)
	}

	if metadata.NodeID == "" || metadata.CollectedAt.IsZero() ||
		len(metadata.System) == 0 || len(metadata.GPUs) == 0 {
		return fmt.Errorf("invalid snapshot metadata")
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO resource_snapshots (
			node_id,
			collected_at,
			system_info,
			gpu_info
		)
		VALUES ($1, $2, $3::jsonb, $4::jsonb)
		ON CONFLICT (node_id, collected_at) DO NOTHING
	`,
		metadata.NodeID,
		metadata.CollectedAt,
		string(metadata.System),
		string(metadata.GPUs),
	)

	if err != nil {
		return fmt.Errorf("save resource snapshot: %w", err)
	}

	return nil
}

// UpsertNodeWithSnapshot 在一个事务中更新节点、GPU 设备和资源历史。
func (r *NodeRepository) UpsertNodeWithSnapshot(
	ctx context.Context,
	snapshot []byte,
) (*node.Node, error) {
	if r == nil || r.pool == nil {
		return nil, fmt.Errorf("node repository unavailable")
	}

	var metadata struct {
		NodeID      string          `json:"node_id"`
		Hostname    string          `json:"hostname"`
		CollectedAt time.Time       `json:"collected_at"`
		System      json.RawMessage `json:"system"`
		GPUs        []struct {
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
		} `json:"gpus"`
	}

	if err := json.Unmarshal(snapshot, &metadata); err != nil {
		return nil, fmt.Errorf("invalid heartbeat JSON: %w", err)
	}

	if metadata.NodeID == "" ||
		metadata.Hostname == "" ||
		metadata.CollectedAt.IsZero() ||
		len(metadata.System) == 0 {
		return nil, fmt.Errorf("invalid heartbeat metadata")
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin heartbeat transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var result node.Node

	err = tx.QueryRow(ctx, `
		INSERT INTO nodes (
			node_id, hostname, status,
			collected_at, last_heartbeat, latest_snapshot
		)
		VALUES ($1, $2, 'online', $3, NOW(), $4::jsonb)
		ON CONFLICT (node_id) DO UPDATE SET
			hostname = EXCLUDED.hostname,
			status = 'online',
			collected_at = EXCLUDED.collected_at,
			last_heartbeat = NOW(),
			latest_snapshot = EXCLUDED.latest_snapshot,
			updated_at = NOW()
		RETURNING node_id, hostname, collected_at,
			last_heartbeat, status, latest_snapshot
	`,
		metadata.NodeID,
		metadata.Hostname,
		metadata.CollectedAt,
		string(snapshot),
	).Scan(
		&result.NodeID,
		&result.Hostname,
		&result.CollectedAt,
		&result.LastHeartbeat,
		&result.Status,
		&result.Snapshot,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert node: %w", err)
	}

	// 当前快照代表节点的完整 GPU 列表。
	_, err = tx.Exec(ctx,
		`DELETE FROM gpu_devices WHERE node_id = $1`,
		metadata.NodeID,
	)
	if err != nil {
		return nil, fmt.Errorf("clear old GPU devices: %w", err)
	}

	for _, gpu := range metadata.GPUs {
		if gpu.Index < 0 || gpu.UUID == "" || gpu.Name == "" {
			return nil, fmt.Errorf("invalid GPU metadata")
		}

		if gpu.MemoryTotalMB > uint64(1<<63-1) {
			return nil, fmt.Errorf("GPU memory value exceeds PostgreSQL BIGINT")
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO gpu_devices (
				node_id, gpu_uuid, gpu_index,
				name, memory_total_mb, driver_version
			)
			VALUES ($1, $2, $3, $4, $5, $6)
		`,
			metadata.NodeID,
			gpu.UUID,
			gpu.Index,
			gpu.Name,
			int64(gpu.MemoryTotalMB),
			gpu.DriverVersion,
		)
		if err != nil {
			return nil, fmt.Errorf("save GPU device %s: %w", gpu.UUID, err)
		}
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO resource_snapshots (
			node_id, collected_at, system_info, gpu_info
		)
		VALUES ($1, $2, $3::jsonb, $4::jsonb)
		ON CONFLICT (node_id, collected_at) DO NOTHING
	`,
		metadata.NodeID,
		metadata.CollectedAt,
		string(metadata.System),
		mustJSON(metadata.GPUs),
	)
	if err != nil {
		return nil, fmt.Errorf("save resource snapshot: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit heartbeat transaction: %w", err)
	}

	return &result, nil
}

// mustJSON 将结构体编码为 JSON。
func mustJSON(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}
