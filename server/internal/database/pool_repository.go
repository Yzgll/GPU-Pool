package database

import (
	"context"
	"errors"
	"fmt"

	"gpu-pool/server/internal/node"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// 资源池业务错误。
var (
	ErrPoolNotFound        = errors.New("resource pool not found")
	ErrNodeNotFound        = errors.New("node not found")
	ErrNodeOffline         = errors.New("node is offline")
	ErrNodeAlreadyAssigned = errors.New("node already belongs to a resource pool")
	ErrPoolDisabled        = errors.New("resource pool is disabled")
)

// CreatePool 创建资源池。
func (r *NodeRepository) CreatePool(
	ctx context.Context,
	name string,
	description string,
) (*node.Pool, error) {
	if r == nil || r.pool == nil {
		return nil, fmt.Errorf("node repository unavailable")
	}
	if name == "" {
		return nil, fmt.Errorf("pool name cannot be empty")
	}
	if len(name) > 64 {
		return nil, fmt.Errorf("pool name cannot exceed 64 characters")
	}

	var result node.Pool

	err := r.pool.QueryRow(ctx, `
        INSERT INTO resource_pools (name, description)
        VALUES ($1, $2)
        RETURNING pool_id, name, description, status,
                  created_at, updated_at
    `, name, description).Scan(
		&result.PoolID,
		&result.Name,
		&result.Description,
		&result.Status,
		&result.CreatedAt,
		&result.UpdatedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, fmt.Errorf("pool name already exists")
		}
		return nil, fmt.Errorf("create resource pool: %w", err)
	}

	return &result, nil
}

// ListPools 查询所有资源池。
func (r *NodeRepository) ListPools(
	ctx context.Context,
) ([]node.Pool, error) {
	if r == nil || r.pool == nil {
		return nil, fmt.Errorf("node repository unavailable")
	}

	rows, err := r.pool.Query(ctx, `
        SELECT
            p.pool_id,
            p.name,
            p.description,
            p.status,
            p.created_at,
            p.updated_at,
            COUNT(pn.node_id)
        FROM resource_pools p
        LEFT JOIN pool_nodes pn ON pn.pool_id = p.pool_id
        GROUP BY p.pool_id
        ORDER BY p.created_at DESC
    `)
	if err != nil {
		return nil, fmt.Errorf("list resource pools: %w", err)
	}
	defer rows.Close()

	result := make([]node.Pool, 0)

	for rows.Next() {
		var item node.Pool

		if err := rows.Scan(
			&item.PoolID,
			&item.Name,
			&item.Description,
			&item.Status,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.NodeCount,
		); err != nil {
			return nil, fmt.Errorf("scan resource pool: %w", err)
		}

		result = append(result, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// GetPool 查询资源池详情。
func (r *NodeRepository) GetPool(
	ctx context.Context,
	poolID int64,
) (*node.Pool, error) {
	if r == nil || r.pool == nil {
		return nil, fmt.Errorf("node repository unavailable")
	}

	var result node.Pool

	err := r.pool.QueryRow(ctx, `
        SELECT
            p.pool_id,
            p.name,
            p.description,
            p.status,
            p.created_at,
            p.updated_at,
            COUNT(pn.node_id)
        FROM resource_pools p
        LEFT JOIN pool_nodes pn ON pn.pool_id = p.pool_id
        WHERE p.pool_id = $1
        GROUP BY p.pool_id
    `, poolID).Scan(
		&result.PoolID,
		&result.Name,
		&result.Description,
		&result.Status,
		&result.CreatedAt,
		&result.UpdatedAt,
		&result.NodeCount,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPoolNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get resource pool: %w", err)
	}

	return &result, nil
}

// UpdatePool 修改资源池名称和描述。
func (r *NodeRepository) UpdatePool(
	ctx context.Context,
	poolID int64,
	name string,
	description string,
) error {
	if r == nil || r.pool == nil {
		return fmt.Errorf("node repository unavailable")
	}
	if name == "" || len(name) > 64 {
		return fmt.Errorf("pool name must contain 1-64 characters")
	}

	tag, err := r.pool.Exec(ctx, `
        UPDATE resource_pools
        SET name = $2,
            description = $3,
            updated_at = NOW()
        WHERE pool_id = $1
    `, poolID, name, description)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("pool name already exists")
		}
		return fmt.Errorf("update resource pool: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrPoolNotFound
	}

	return nil
}

// SetPoolStatus 设置资源池状态。
func (r *NodeRepository) SetPoolStatus(
	ctx context.Context,
	poolID int64,
	status string,
) error {
	if r == nil || r.pool == nil {
		return fmt.Errorf("node repository unavailable")
	}
	if status != "active" && status != "disabled" {
		return fmt.Errorf("invalid pool status: %s", status)
	}

	tag, err := r.pool.Exec(ctx, `
        UPDATE resource_pools
        SET status = $2,
            updated_at = NOW()
        WHERE pool_id = $1
    `, poolID, status)

	if err != nil {
		return fmt.Errorf("update resource pool status: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrPoolNotFound
	}

	return nil
}

// DeletePool 删除资源池。
// 根据外键 ON DELETE CASCADE 自动解除节点关联，不删除节点。
func (r *NodeRepository) DeletePool(
	ctx context.Context,
	poolID int64,
) error {
	if r == nil || r.pool == nil {
		return fmt.Errorf("node repository unavailable")
	}

	tag, err := r.pool.Exec(ctx, `
        DELETE FROM resource_pools
        WHERE pool_id = $1
    `, poolID)

	if err != nil {
		return fmt.Errorf("delete resource pool: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrPoolNotFound
	}

	return nil
}

// AddNodeToPool 将在线节点加入资源池。
func (r *NodeRepository) AddNodeToPool(
	ctx context.Context,
	poolID int64,
	nodeID string,
) error {
	if r == nil || r.pool == nil {
		return fmt.Errorf("node repository unavailable")
	}
	if nodeID == "" {
		return fmt.Errorf("node_id cannot be empty")
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin add node transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// 锁定资源池，防止并发禁用与添加操作冲突。
	var status string

	err = tx.QueryRow(ctx, `
        SELECT status
        FROM resource_pools
        WHERE pool_id = $1
        FOR UPDATE
    `, poolID).Scan(&status)

	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPoolNotFound
	}
	if err != nil {
		return fmt.Errorf("query resource pool: %w", err)
	}

	if status != "active" {
		return ErrPoolDisabled
	}

	// 检查节点是否存在且在线。
	var online bool

	err = tx.QueryRow(ctx, `
        SELECT last_heartbeat >= NOW() - INTERVAL '30 seconds'
        FROM nodes
        WHERE node_id = $1
    `, nodeID).Scan(&online)

	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNodeNotFound
	}
	if err != nil {
		return fmt.Errorf("check node status: %w", err)
	}

	if !online {
		return ErrNodeOffline
	}

	// node_id 是主键，保证一个节点只能加入一个资源池。
	_, err = tx.Exec(ctx, `
        INSERT INTO pool_nodes (node_id, pool_id)
        VALUES ($1, $2)
    `, nodeID, poolID)

	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrNodeAlreadyAssigned
		}

		return fmt.Errorf("add node to resource pool: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit add node transaction: %w", err)
	}

	return nil
}

// RemoveNodeFromPool 从资源池移除节点。
func (r *NodeRepository) RemoveNodeFromPool(
	ctx context.Context,
	poolID int64,
	nodeID string,
) error {
	if r == nil || r.pool == nil {
		return fmt.Errorf("node repository unavailable")
	}

	tag, err := r.pool.Exec(ctx, `
        DELETE FROM pool_nodes
        WHERE pool_id = $1 AND node_id = $2
    `, poolID, nodeID)

	if err != nil {
		return fmt.Errorf("remove node from resource pool: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("node is not a member of this resource pool")
	}

	return nil
}

// ListPoolNodes 查询资源池中的节点。
func (r *NodeRepository) ListPoolNodes(
	ctx context.Context,
	poolID int64,
) ([]node.PoolNode, error) {
	if r == nil || r.pool == nil {
		return nil, fmt.Errorf("node repository unavailable")
	}

	rows, err := r.pool.Query(ctx, `
        SELECT
            pn.pool_id,
            pn.node_id,
            n.hostname,
            pn.joined_at
        FROM pool_nodes pn
        JOIN nodes n ON n.node_id = pn.node_id
        WHERE pn.pool_id = $1
        ORDER BY pn.joined_at ASC
    `, poolID)

	if err != nil {
		return nil, fmt.Errorf("list pool nodes: %w", err)
	}
	defer rows.Close()

	result := make([]node.PoolNode, 0)

	for rows.Next() {
		var item node.PoolNode

		if err := rows.Scan(
			&item.PoolID,
			&item.NodeID,
			&item.Hostname,
			&item.JoinedAt,
		); err != nil {
			return nil, fmt.Errorf("scan pool node: %w", err)
		}

		result = append(result, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// GetPoolStats 统计资源池的 GPU 数量和显存。
// GPU 动态资源取每张 GPU 最新的快照。
func (r *NodeRepository) GetPoolStats(
	ctx context.Context,
	poolID int64,
) (*node.PoolStats, error) {
	if r == nil || r.pool == nil {
		return nil, fmt.Errorf("node repository unavailable")
	}

	result := &node.PoolStats{
		PoolID: poolID,
	}

	// 先确认资源池存在。
	var exists bool

	err := r.pool.QueryRow(ctx, `
        SELECT EXISTS (
            SELECT 1 FROM resource_pools WHERE pool_id = $1
        )
    `, poolID).Scan(&exists)

	if err != nil {
		return nil, fmt.Errorf("check resource pool: %w", err)
	}
	if !exists {
		return nil, ErrPoolNotFound
	}

	err = r.pool.QueryRow(ctx, `
        SELECT COUNT(*)
        FROM pool_nodes
        WHERE pool_id = $1
    `, poolID).Scan(&result.NodeCount)

	if err != nil {
		return nil, fmt.Errorf("count pool nodes: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
        SELECT
            g.memory_total_mb,
            COALESCE((latest.gpu->>'memory_used_mb')::BIGINT, 0),
            COALESCE((latest.gpu->>'memory_free_mb')::BIGINT, 0)
        FROM pool_nodes pn
        JOIN gpu_devices g ON g.node_id = pn.node_id
        LEFT JOIN LATERAL (
            SELECT gpu
            FROM resource_snapshots rs
            CROSS JOIN LATERAL jsonb_array_elements(rs.gpu_info) AS gpu
            WHERE rs.node_id = pn.node_id
              AND gpu->>'uuid' = g.gpu_uuid
            ORDER BY rs.collected_at DESC
            LIMIT 1
        ) latest ON TRUE
        WHERE pn.pool_id = $1
    `, poolID)

	if err != nil {
		return nil, fmt.Errorf("query pool GPU stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var total, used, free int64

		if err := rows.Scan(&total, &used, &free); err != nil {
			return nil, fmt.Errorf("scan pool GPU stats: %w", err)
		}

		result.GPUCount++
		result.MemoryTotalMB += total
		result.MemoryUsedMB += used
		result.MemoryFreeMB += free
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pool GPU stats: %w", err)
	}

	return result, nil
}
