package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gpu-pool/server/internal/node"
)

func (r *NodeRepository) GetGPUCompareHistory(
	ctx context.Context,
	selected []node.GPUCompareDevice,
	from time.Time,
	to time.Time,
	intervalSeconds int,
) (*node.GPUCompareResult, error) {
	if r == nil || r.pool == nil {
		return nil, fmt.Errorf("node repository unavailable")
	}
	if len(selected) == 0 || len(selected) > 8 {
		return nil, fmt.Errorf("select between 1 and 8 GPUs")
	}
	if !from.Before(to) || intervalSeconds <= 0 {
		return nil, fmt.Errorf("invalid time range")
	}

	for _, device := range selected {
		if device.NodeID == "" || device.GPUIndex < 0 {
			return nil, fmt.Errorf("invalid GPU selection")
		}
	}

	selectionJSON, err := json.Marshal(selected)
	if err != nil {
		return nil, fmt.Errorf("encode GPU selection: %w", err)
	}

	result := &node.GPUCompareResult{
		IntervalSeconds: intervalSeconds,
		Devices:         make([]node.GPUCompareDevice, 0),
		Points:          make([]node.GPUComparePoint, 0),
	}

	rows, err := r.pool.Query(ctx, `
		WITH selected AS (
			SELECT DISTINCT node_id, gpu_index
			FROM jsonb_to_recordset($1::jsonb)
				AS x(node_id text, gpu_index integer)
		),
		samples AS (
			SELECT
				to_timestamp(
					floor(extract(epoch FROM rs.collected_at)
						/ $4::double precision) * $4
				) AS bucket,
				rs.collected_at,
				rs.node_id,
				n.hostname,
				(gpu->>'index')::integer AS gpu_index,
				gpu->>'name' AS gpu_name,
				COALESCE((gpu->>'utilization')::double precision, 0)
					AS utilization,
				COALESCE((gpu->>'memory_used_mb')::bigint, 0)
					AS memory_used_mb,
				COALESCE((gpu->>'temperature')::double precision, 0)
					AS temperature
			FROM resource_snapshots rs
			JOIN selected s ON s.node_id = rs.node_id
			JOIN nodes n ON n.node_id = rs.node_id
			CROSS JOIN LATERAL
				jsonb_array_elements(rs.gpu_info) AS gpu_data(gpu)
			WHERE rs.collected_at >= $2
			  AND rs.collected_at <= $3
			  AND (gpu->>'index')::integer = s.gpu_index
		),
		latest AS (
			SELECT DISTINCT ON (bucket, node_id, gpu_index)
				bucket,
				node_id,
				hostname,
				gpu_index,
				gpu_name,
				utilization,
				memory_used_mb,
				temperature
			FROM samples
			ORDER BY bucket, node_id, gpu_index, collected_at DESC
		)
		SELECT
			bucket,
			node_id,
			hostname,
			gpu_index,
			gpu_name,
			utilization,
			memory_used_mb,
			temperature
		FROM latest
		ORDER BY bucket ASC, node_id, gpu_index
	`, selectionJSON, from, to, intervalSeconds)

	if err != nil {
		return nil, fmt.Errorf("query GPU comparison: %w", err)
	}
	defer rows.Close()

	seen := make(map[string]bool)

	for rows.Next() {
		var point node.GPUComparePoint
		var hostname, gpuName string

		if err := rows.Scan(
			&point.CollectedAt,
			&point.NodeID,
			&hostname,
			&point.GPUIndex,
			&gpuName,
			&point.Utilization,
			&point.MemoryUsedMB,
			&point.Temperature,
		); err != nil {
			return nil, fmt.Errorf("scan GPU comparison: %w", err)
		}

		key := fmt.Sprintf("%s:%d", point.NodeID, point.GPUIndex)

		if !seen[key] {
			result.Devices = append(result.Devices, node.GPUCompareDevice{
				NodeID:   point.NodeID,
				Hostname: hostname,
				GPUIndex: point.GPUIndex,
				GPUName:  gpuName,
			})
			seen[key] = true
		}

		result.Points = append(result.Points, point)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate GPU comparison: %w", err)
	}

	return result, nil
}
