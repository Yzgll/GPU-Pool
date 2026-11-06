package database

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// createPoolTestEnvironment 创建独立 Schema 和测试 Repository。
func createPoolTestEnvironment(t *testing.T) (
	context.Context,
	*pgxpool.Pool,
	*NodeRepository,
) {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)

	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		cancel()
		t.Fatal(err)
	}

	if err := admin.Ping(ctx); err != nil {
		admin.Close()
		cancel()
		t.Fatal(err)
	}

	schema := fmt.Sprintf(
		"pool_test_%d",
		time.Now().UnixNano(),
	)

	if _, err := admin.Exec(
		ctx,
		"CREATE SCHEMA "+schema,
	); err != nil {
		admin.Close()
		cancel()
		t.Fatal(err)
	}

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
		admin.Close()
		cancel()
		t.Fatal(err)
	}

	config.ConnConfig.RuntimeParams["search_path"] = schema

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
		admin.Close()
		cancel()
		t.Fatal(err)
	}

	if err := Migrate(ctx, pool, "../../migrations"); err != nil {
		pool.Close()
		admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
		admin.Close()
		cancel()
		t.Fatalf("migration failed: %v", err)
	}

	repo, err := NewNodeRepository(pool)
	if err != nil {
		pool.Close()
		admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
		admin.Close()
		cancel()
		t.Fatal(err)
	}

	t.Cleanup(func() {
		pool.Close()

		cleanupCtx, cleanupCancel := context.WithTimeout(
			context.Background(),
			10*time.Second,
		)
		defer cleanupCancel()

		_, _ = admin.Exec(
			cleanupCtx,
			"DROP SCHEMA "+schema+" CASCADE",
		)

		admin.Close()
		cancel()
	})

	return ctx, pool, repo
}

// createPoolTestNode 注册测试节点。
func createPoolTestNode(
	t *testing.T,
	ctx context.Context,
	repo *NodeRepository,
	nodeID string,
	hostname string,
	gpuUUID string,
) {
	t.Helper()

	snapshot := fmt.Sprintf(`{
        "node_id": %q,
        "hostname": %q,
        "collected_at": %q,
        "system": {
            "cpu": {
                "logical_cores": 16
            },
            "memory": {
                "total_bytes": 16000000000
            }
        },
        "gpus": [
            {
                "index": 0,
                "uuid": %q,
                "name": "NVIDIA Test GPU",
                "memory_total_mb": 8192,
                "memory_used_mb": 2048,
                "memory_free_mb": 6144,
                "utilization": 30,
                "temperature": 50,
                "power_w": 120.5,
                "driver_version": "572.83"
            }
        ]
    }`,
		nodeID,
		hostname,
		time.Now().UTC().Format(time.RFC3339Nano),
		gpuUUID,
	)

	_, err := repo.UpsertNodeWithSnapshot(
		ctx,
		[]byte(snapshot),
	)

	if err != nil {
		t.Fatalf("create test node %s: %v", nodeID, err)
	}
}

// TestPoolRepositoryIntegration 资源池 Repository 集成测试。
func TestPoolRepositoryIntegration(t *testing.T) {
	ctx, pool, repo := createPoolTestEnvironment(t)

	// 1. 创建资源池。
	first, err := repo.CreatePool(
		ctx,
		"test-pool-001",
		"first test pool",
	)
	if err != nil {
		t.Fatalf("create pool failed: %v", err)
	}

	if first.PoolID <= 0 {
		t.Fatalf("invalid pool ID: %d", first.PoolID)
	}

	if first.Status != "active" {
		t.Fatalf("unexpected status: %s", first.Status)
	}

	// 2. 测试重复名称。
	_, err = repo.CreatePool(
		ctx,
		"test-pool-001",
		"duplicate",
	)
	if err == nil {
		t.Fatal("expected duplicate pool name error")
	}

	// 3. 创建第二个资源池。
	second, err := repo.CreatePool(
		ctx,
		"test-pool-002",
		"second test pool",
	)
	if err != nil {
		t.Fatalf("create second pool failed: %v", err)
	}

	// 4. 查询资源池列表。
	pools, err := repo.ListPools(ctx)
	if err != nil {
		t.Fatalf("list pools failed: %v", err)
	}

	if len(pools) != 2 {
		t.Fatalf("expected 2 pools, got %d", len(pools))
	}

	// 5. 查询资源池详情。
	got, err := repo.GetPool(ctx, first.PoolID)
	if err != nil {
		t.Fatalf("get pool failed: %v", err)
	}

	if got.Name != "test-pool-001" {
		t.Fatalf("unexpected pool name: %s", got.Name)
	}

	if got.NodeCount != 0 {
		t.Fatalf("expected 0 nodes, got %d", got.NodeCount)
	}

	// 6. 修改资源池。
	err = repo.UpdatePool(
		ctx,
		first.PoolID,
		"test-pool-updated",
		"updated description",
	)
	if err != nil {
		t.Fatalf("update pool failed: %v", err)
	}

	got, err = repo.GetPool(ctx, first.PoolID)
	if err != nil {
		t.Fatal(err)
	}

	if got.Name != "test-pool-updated" {
		t.Fatalf("pool name not updated: %s", got.Name)
	}

	// 7. 禁用资源池。
	err = repo.SetPoolStatus(
		ctx,
		first.PoolID,
		"disabled",
	)
	if err != nil {
		t.Fatalf("disable pool failed: %v", err)
	}

	// 8. 创建在线节点。
	createPoolTestNode(
		t,
		ctx,
		repo,
		"pool-node-001",
		"worker-001",
		"GPU-pool-test-001",
	)

	// 9. 禁用资源池不能添加节点。
	err = repo.AddNodeToPool(
		ctx,
		first.PoolID,
		"pool-node-001",
	)

	if !errors.Is(err, ErrPoolDisabled) {
		t.Fatalf("expected ErrPoolDisabled, got %v", err)
	}

	// 10. 重新启用资源池。
	err = repo.SetPoolStatus(
		ctx,
		first.PoolID,
		"active",
	)
	if err != nil {
		t.Fatalf("enable pool failed: %v", err)
	}

	// 11. 在线节点加入资源池。
	err = repo.AddNodeToPool(
		ctx,
		first.PoolID,
		"pool-node-001",
	)
	if err != nil {
		t.Fatalf("add online node failed: %v", err)
	}

	// 12. 重复添加节点。
	err = repo.AddNodeToPool(
		ctx,
		first.PoolID,
		"pool-node-001",
	)

	if !errors.Is(err, ErrNodeAlreadyAssigned) {
		t.Fatalf("expected ErrNodeAlreadyAssigned, got %v", err)
	}

	// 13. 同一节点不能加入第二个资源池。
	err = repo.AddNodeToPool(
		ctx,
		second.PoolID,
		"pool-node-001",
	)

	if !errors.Is(err, ErrNodeAlreadyAssigned) {
		t.Fatalf("expected ErrNodeAlreadyAssigned for second pool, got %v", err)
	}

	// 14. 不存在的节点不能加入。
	err = repo.AddNodeToPool(
		ctx,
		first.PoolID,
		"missing-node",
	)

	if !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("expected ErrNodeNotFound, got %v", err)
	}

	// 15. 创建离线节点。
	createPoolTestNode(
		t,
		ctx,
		repo,
		"pool-node-offline",
		"worker-offline",
		"GPU-pool-offline",
	)

	_, err = pool.Exec(ctx, `
        UPDATE nodes
        SET last_heartbeat = NOW() - INTERVAL '60 seconds'
        WHERE node_id = $1
    `, "pool-node-offline")

	if err != nil {
		t.Fatal(err)
	}

	// 16. 离线节点不能加入。
	err = repo.AddNodeToPool(
		ctx,
		first.PoolID,
		"pool-node-offline",
	)

	if !errors.Is(err, ErrNodeOffline) {
		t.Fatalf("expected ErrNodeOffline, got %v", err)
	}

	// 17. 查询资源池节点。
	members, err := repo.ListPoolNodes(ctx, first.PoolID)
	if err != nil {
		t.Fatalf("list pool nodes failed: %v", err)
	}

	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}

	if members[0].NodeID != "pool-node-001" {
		t.Fatalf("unexpected node: %s", members[0].NodeID)
	}

	// 18. 验证 GPU 统计。
	stats, err := repo.GetPoolStats(ctx, first.PoolID)
	if err != nil {
		t.Fatalf("get pool stats failed: %v", err)
	}

	if stats.NodeCount != 1 {
		t.Fatalf("expected 1 node, got %d", stats.NodeCount)
	}

	if stats.GPUCount != 1 {
		t.Fatalf("expected 1 GPU, got %d", stats.GPUCount)
	}

	if stats.MemoryTotalMB != 8192 {
		t.Fatalf("expected 8192 MB total, got %d", stats.MemoryTotalMB)
	}

	if stats.MemoryUsedMB != 2048 {
		t.Fatalf("expected 2048 MB used, got %d", stats.MemoryUsedMB)
	}

	if stats.MemoryFreeMB != 6144 {
		t.Fatalf("expected 6144 MB free, got %d", stats.MemoryFreeMB)
	}

	// 19. 移除节点。
	err = repo.RemoveNodeFromPool(
		ctx,
		first.PoolID,
		"pool-node-001",
	)
	if err != nil {
		t.Fatalf("remove node failed: %v", err)
	}

	members, err = repo.ListPoolNodes(ctx, first.PoolID)
	if err != nil {
		t.Fatal(err)
	}

	if len(members) != 0 {
		t.Fatalf("expected 0 members after removal, got %d", len(members))
	}

	// 20. 移除不存在的关联。
	err = repo.RemoveNodeFromPool(
		ctx,
		first.PoolID,
		"pool-node-001",
	)
	if err == nil {
		t.Fatal("expected error when removing non-member")
	}

	// 21. 删除资源池。
	err = repo.DeletePool(ctx, first.PoolID)
	if err != nil {
		t.Fatalf("delete pool failed: %v", err)
	}

	// 22. 删除不存在的资源池。
	err = repo.DeletePool(ctx, first.PoolID)
	if !errors.Is(err, ErrPoolNotFound) {
		t.Fatalf("expected ErrPoolNotFound, got %v", err)
	}

	// 23. 资源池删除不能删除节点。
	worker, err := repo.GetNode(ctx, "pool-node-001")
	if err != nil {
		t.Fatalf("node should still exist after pool deletion: %v", err)
	}

	if worker.NodeID != "pool-node-001" {
		t.Fatalf("unexpected node ID: %s", worker.NodeID)
	}

	t.Log("Pool repository integration tests passed")
}

// TestPoolRepositoryInvalidParams 测试资源池参数校验。
func TestPoolRepositoryInvalidParams(t *testing.T) {
	ctx, _, repo := createPoolTestEnvironment(t)

	// 空名称。
	_, err := repo.CreatePool(ctx, "", "description")
	if err == nil {
		t.Fatal("expected error for empty pool name")
	}

	// 超长名称。
	_, err = repo.CreatePool(
		ctx,
		"12345678901234567890123456789012345678901234567890123456789012345",
		"description",
	)
	if err == nil {
		t.Fatal("expected error for oversized pool name")
	}

	// 无效状态。
	err = repo.SetPoolStatus(ctx, 1, "unknown")
	if err == nil {
		t.Fatal("expected error for invalid status")
	}

	// 空节点 ID。
	err = repo.AddNodeToPool(ctx, 1, "")
	if err == nil {
		t.Fatal("expected error for empty node ID")
	}

	t.Log("Pool repository invalid parameter tests passed")
}
