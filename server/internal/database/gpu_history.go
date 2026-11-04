package database

import (
	"context"
	"fmt"
	"time"

	"gpu-pool/server/internal/node"
)

// GetGPUHistory 分页查询 GPU 历史资源。
// nodeID 为空时查询所有节点。
// gpuIndex 为 nil 时查询节点的全部 GPU。
func (r *NodeRepository) GetGPUHistory(
	ctx context.Context,
	nodeID string,
	gpuIndex *int,
	from *time.Time,
	to *time.Time,
	limit int,
	offset int,
) (*node.GPUHistoryResult, error) {

	if r == nil || r.pool == nil {
		return nil, fmt.Errorf("node repository unavailable")
	}

	if limit <= 0 || limit > 200 {
		return nil, fmt.Errorf("limit must be between 1 and 200")
	}

	if offset < 0 {
		return nil, fmt.Errorf("offset cannot be negative")
	}

	if gpuIndex != nil && *gpuIndex < 0 {
		return nil, fmt.Errorf("gpu_index cannot be negative")
	}

	if from != nil && to != nil && from.After(*to) {
		return nil, fmt.Errorf("from cannot be after to")
	}

	result := &node.GPUHistoryResult{
		Limit:   limit,
		Offset:  offset,
		Records: make([]node.GPUHistoryRecord, 0),
	}

	// 统计符合条件的 GPU 历史记录总数。
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM resource_snapshots rs
		JOIN nodes n ON n.node_id = rs.node_id
		CROSS JOIN LATERAL
			jsonb_array_elements(rs.gpu_info) AS gpu_data(gpu)
		WHERE ($1 = '' OR rs.node_id = $1)
		  AND ($2::integer IS NULL OR (gpu->>'index')::integer = $2)
		  AND ($3::timestamptz IS NULL OR rs.collected_at >= $3)
		  AND ($4::timestamptz IS NULL OR rs.collected_at <= $4)
	`,
		nodeID,
		gpuIndex,
		from,
		to,
	).Scan(&result.Total)

	if err != nil {
		return nil, fmt.Errorf("count GPU history: %w", err)
	}

	// 查询 GPU 历史记录。
	rows, err := r.pool.Query(ctx, `
		SELECT
			rs.id,
			rs.node_id,
			n.hostname,
			gpu->>'uuid',
			(gpu->>'index')::integer,
			gpu->>'name',
			COALESCE((gpu->>'memory_total_mb')::bigint, 0),
			COALESCE((gpu->>'memory_used_mb')::bigint, 0),
			COALESCE((gpu->>'memory_free_mb')::bigint, 0),
			COALESCE((gpu->>'utilization')::integer, 0),
			COALESCE((gpu->>'temperature')::integer, 0),
			COALESCE((gpu->>'power_w')::double precision, 0),
			COALESCE(gpu->>'driver_version', ''),
			rs.collected_at
		FROM resource_snapshots rs
		JOIN nodes n ON n.node_id = rs.node_id
		CROSS JOIN LATERAL
			jsonb_array_elements(rs.gpu_info) AS gpu_data(gpu)
		WHERE ($1 = '' OR rs.node_id = $1)
		  AND ($2::integer IS NULL OR (gpu->>'index')::integer = $2)
		  AND ($3::timestamptz IS NULL OR rs.collected_at >= $3)
		  AND ($4::timestamptz IS NULL OR rs.collected_at <= $4)
		ORDER BY rs.collected_at DESC, rs.node_id, (gpu->>'index')::integer
		LIMIT $5 OFFSET $6
	`,
		nodeID,
		gpuIndex,
		from,
		to,
		limit,
		offset,
	)

	if err != nil {
		return nil, fmt.Errorf("query GPU history: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var record node.GPUHistoryRecord

		err := rows.Scan(
			&record.ID,
			&record.NodeID,
			&record.Hostname,
			&record.GPUUUID,
			&record.GPUIndex,
			&record.Name,
			&record.MemoryTotalMB,
			&record.MemoryUsedMB,
			&record.MemoryFreeMB,
			&record.Utilization,
			&record.Temperature,
			&record.PowerW,
			&record.DriverVersion,
			&record.CollectedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("scan GPU history: %w", err)
		}

		result.Records = append(result.Records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate GPU history: %w", err)
	}

	return result, nil
}
