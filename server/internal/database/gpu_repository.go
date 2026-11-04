package database

import (
	"context"
	"fmt"

	"gpu-pool/server/internal/node"
)

// ListGPUs 查询所有节点的 GPU 最新资源。
func (r *NodeRepository) ListGPUs(
	ctx context.Context,
) ([]node.GPUDevice, error) {

	if r == nil || r.pool == nil {
		return nil, fmt.Errorf("node repository unavailable")
	}

	rows, err := r.pool.Query(ctx, `
        SELECT
            n.node_id,
            n.hostname,
            g.gpu_uuid,
            g.gpu_index,
            g.name,
            g.memory_total_mb,
            COALESCE((s.gpu->>'memory_used_mb')::BIGINT, 0),
            COALESCE((s.gpu->>'memory_free_mb')::BIGINT, 0),
            COALESCE((s.gpu->>'utilization')::INTEGER, 0),
            COALESCE((s.gpu->>'temperature')::INTEGER, 0),
            COALESCE((s.gpu->>'power_w')::DOUBLE PRECISION, 0),
            g.driver_version,
            s.collected_at
        FROM gpu_devices g
        JOIN nodes n ON n.node_id = g.node_id
        LEFT JOIN LATERAL (
            SELECT
                rs.gpu_info,
                rs.collected_at,
                gpu
            FROM resource_snapshots rs
            CROSS JOIN LATERAL jsonb_array_elements(rs.gpu_info) AS gpu
            WHERE rs.node_id = n.node_id
              AND gpu->>'uuid' = g.gpu_uuid
            ORDER BY rs.collected_at DESC
            LIMIT 1
        ) s ON TRUE
        ORDER BY n.hostname, g.gpu_index
    `)

	if err != nil {
		return nil, fmt.Errorf("query GPUs: %w", err)
	}
	defer rows.Close()

	result := make([]node.GPUDevice, 0)

	for rows.Next() {
		var gpu node.GPUDevice

		err := rows.Scan(
			&gpu.NodeID,
			&gpu.Hostname,
			&gpu.GPUUUID,
			&gpu.GPUIndex,
			&gpu.Name,
			&gpu.MemoryTotalMB,
			&gpu.MemoryUsedMB,
			&gpu.MemoryFreeMB,
			&gpu.Utilization,
			&gpu.Temperature,
			&gpu.PowerW,
			&gpu.DriverVersion,
			&gpu.CollectedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("scan GPU: %w", err)
		}

		result = append(result, gpu)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// GetGPUsByNode 查询指定节点的 GPU。
func (r *NodeRepository) GetGPUsByNode(
	ctx context.Context,
	nodeID string,
) ([]node.GPUDevice, error) {

	gpus, err := r.ListGPUs(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]node.GPUDevice, 0)

	for _, gpu := range gpus {
		if gpu.NodeID == nodeID {
			result = append(result, gpu)
		}
	}

	return result, nil
}
