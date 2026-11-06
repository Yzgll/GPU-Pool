package database

import (
	"context"
	"fmt"
	"time"

	"gpu-pool/server/internal/node"
)

func (r *NodeRepository) GetGPUClusterHistory(
	ctx context.Context,
	from time.Time,
	to time.Time,
	intervalSeconds int,
) (*node.GPUClusterHistoryResult, error) {
	if r == nil || r.pool == nil {
		return nil, fmt.Errorf("node repository unavailable")
	}

	if intervalSeconds <= 0 || !from.Before(to) {
		return nil, fmt.Errorf("invalid time range or interval")
	}

	result := &node.GPUClusterHistoryResult{
		IntervalSeconds: intervalSeconds,
		Points:          make([]node.GPUClusterHistoryPoint, 0),
	}

	rows, err := r.pool.Query(ctx, `
		WITH samples AS (
			SELECT
				to_timestamp(
					floor(
						extract(epoch FROM rs.collected_at) / $3::double precision
					) * $3
				) AS bucket,
				rs.collected_at,
				rs.node_id,
				COALESCE(
					NULLIF(gpu->>'uuid', ''),
					'index:' || (gpu->>'index')
				) AS gpu_key,
				COALESCE((gpu->>'utilization')::double precision, 0) AS utilization,
				COALESCE((gpu->>'memory_used_mb')::bigint, 0) AS memory_used_mb,
				COALESCE((gpu->>'memory_total_mb')::bigint, 0) AS memory_total_mb,
				COALESCE((gpu->>'temperature')::double precision, 0) AS temperature,
				COALESCE((gpu->>'power_w')::double precision, 0) AS power_w
			FROM resource_snapshots rs
			CROSS JOIN LATERAL
				jsonb_array_elements(rs.gpu_info) AS gpu_data(gpu)
			WHERE rs.collected_at >= $1
			  AND rs.collected_at <= $2
		),
		latest_per_gpu AS (
			SELECT DISTINCT ON (bucket, node_id, gpu_key)
				bucket,
				node_id,
				gpu_key,
				utilization,
				memory_used_mb,
				memory_total_mb,
				temperature,
				power_w
			FROM samples
			ORDER BY bucket, node_id, gpu_key, collected_at DESC
		)
		SELECT
			bucket,
			COUNT(*)::integer,
			AVG(utilization),
			SUM(memory_used_mb)::bigint,
			SUM(memory_total_mb)::bigint,
			AVG(temperature),
			SUM(power_w)
		FROM latest_per_gpu
		GROUP BY bucket
		ORDER BY bucket ASC
	`, from, to, intervalSeconds)

	if err != nil {
		return nil, fmt.Errorf("query GPU cluster history: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var point node.GPUClusterHistoryPoint

		if err := rows.Scan(
			&point.CollectedAt,
			&point.GPUCount,
			&point.AvgUtilization,
			&point.MemoryUsedMB,
			&point.MemoryTotalMB,
			&point.AvgTemperature,
			&point.TotalPowerW,
		); err != nil {
			return nil, fmt.Errorf("scan GPU cluster history: %w", err)
		}

		result.Points = append(result.Points, point)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate GPU cluster history: %w", err)
	}

	return result, nil
}
