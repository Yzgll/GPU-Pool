package database

import (
	"context"
	"fmt"
	"time"

	"gpu-pool/server/internal/node"

	"github.com/jackc/pgx/v5"
)

// GetResourceHistory 分页查询指定节点的资源历史。
func (r *NodeRepository) GetResourceHistory(
	ctx context.Context,
	nodeID string,
	from *time.Time,
	to *time.Time,
	limit int,
	offset int,
) (*node.ResourceHistory, error) {
	if r == nil || r.pool == nil {
		return nil, fmt.Errorf("node repository unavailable")
	}

	if nodeID == "" {
		return nil, fmt.Errorf("node_id cannot be empty")
	}

	if limit <= 0 || limit > 200 {
		return nil, fmt.Errorf("limit must be between 1 and 200")
	}

	if offset < 0 {
		return nil, fmt.Errorf("offset cannot be negative")
	}

	if from != nil && to != nil && from.After(*to) {
		return nil, fmt.Errorf("from cannot be after to")
	}

	result := &node.ResourceHistory{
		Limit:   limit,
		Offset:  offset,
		Records: make([]node.ResourceSnapshot, 0),
	}

	// 先查询符合条件的历史记录总数。
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM resource_snapshots
		WHERE node_id = $1
		  AND ($2::timestamptz IS NULL OR collected_at >= $2)
		  AND ($3::timestamptz IS NULL OR collected_at <= $3)
	`,
		nodeID,
		from,
		to,
	).Scan(&result.Total)

	if err != nil {
		return nil, fmt.Errorf("count resource history: %w", err)
	}

	// 按采集时间倒序查询资源历史。
	rows, err := r.pool.Query(ctx, `
		SELECT
			id,
			node_id,
			collected_at,
			system_info,
			gpu_info
		FROM resource_snapshots
		WHERE node_id = $1
		  AND ($2::timestamptz IS NULL OR collected_at >= $2)
		  AND ($3::timestamptz IS NULL OR collected_at <= $3)
		ORDER BY collected_at DESC
		LIMIT $4 OFFSET $5
	`,
		nodeID,
		from,
		to,
		limit,
		offset,
	)

	if err != nil {
		return nil, fmt.Errorf("query resource history: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var record node.ResourceSnapshot

		if err := rows.Scan(
			&record.ID,
			&record.NodeID,
			&record.CollectedAt,
			&record.SystemInfo,
			&record.GPUInfo,
		); err != nil {
			return nil, fmt.Errorf("scan resource history: %w", err)
		}

		result.Records = append(result.Records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate resource history: %w", err)
	}

	return result, nil
}

// GetLatestResourceSnapshot 查询指定节点最新的一条资源快照。
func (r *NodeRepository) GetLatestResourceSnapshot(
	ctx context.Context,
	nodeID string,
) (*node.ResourceSnapshot, error) {
	if r == nil || r.pool == nil {
		return nil, fmt.Errorf("node repository unavailable")
	}

	if nodeID == "" {
		return nil, fmt.Errorf("node_id cannot be empty")
	}

	var result node.ResourceSnapshot

	err := r.pool.QueryRow(ctx, `
		SELECT
			id,
			node_id,
			collected_at,
			system_info,
			gpu_info
		FROM resource_snapshots
		WHERE node_id = $1
		ORDER BY collected_at DESC
		LIMIT 1
	`, nodeID).Scan(
		&result.ID,
		&result.NodeID,
		&result.CollectedAt,
		&result.SystemInfo,
		&result.GPUInfo,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}

		return nil, fmt.Errorf("get latest resource snapshot: %w", err)
	}

	return &result, nil
}
