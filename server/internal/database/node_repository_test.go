package database

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestNodeRepositoryIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()

	if err := admin.Ping(ctx); err != nil {
		t.Fatal(err)
	}

	schema := fmt.Sprintf(
		"node_test_%d",
		time.Now().UnixNano(),
	)

	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}

	defer func() {
		_, _ = admin.Exec(
			context.Background(),
			"DROP SCHEMA "+schema+" CASCADE",
		)
	}()

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}

	config.ConnConfig.RuntimeParams["search_path"] = schema

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	if err := Migrate(ctx, pool, "../../migrations"); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	repo, err := NewNodeRepository(pool)
	if err != nil {
		t.Fatal(err)
	}

	snapshot := []byte(`{
		"node_id": "test-node-001",
		"hostname": "test-worker",
		"collected_at": "2026-11-02T14:00:00Z",
		"system": {
			"cpu": {"logical_cores": 16},
			"memory": {"total_bytes": 16000000000}
		},
		"gpus": [
			{
				"index": 0,
				"uuid": "GPU-test-001",
				"name": "NVIDIA Test GPU",
				"memory_total_mb": 8192,
				"driver_version": "572.83"
			}
		]
	}`)

	// 1. 首次注册。
	first, err := repo.UpsertNodeWithSnapshot(ctx, snapshot)
	if err != nil {
		t.Fatalf("first heartbeat failed: %v", err)
	}

	if first.NodeID != "test-node-001" {
		t.Fatalf("unexpected node ID: %s", first.NodeID)
	}

	// 2. 再次心跳，更新主机名。
	updatedSnapshot := []byte(`{
		"node_id": "test-node-001",
		"hostname": "test-worker-updated",
		"collected_at": "2026-11-02T14:00:10Z",
		"system": {
			"cpu": {"logical_cores": 16},
			"memory": {"total_bytes": 16000000000}
		},
		"gpus": [
			{
				"index": 0,
				"uuid": "GPU-test-001",
				"name": "NVIDIA Test GPU",
				"memory_total_mb": 8192,
				"driver_version": "572.83"
			}
		]
	}`)

	_, err = repo.UpsertNodeWithSnapshot(ctx, updatedSnapshot)
	if err != nil {
		t.Fatalf("second heartbeat failed: %v", err)
	}

	// 3. 查询节点。
	got, err := repo.GetNode(ctx, "test-node-001")
	if err != nil {
		t.Fatal(err)
	}

	if got.Hostname != "test-worker-updated" {
		t.Fatalf("hostname not updated: %s", got.Hostname)
	}

	// 4. 查询 GPU 设备。
	var gpuCount int
	err = pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM gpu_devices WHERE node_id = $1",
		"test-node-001",
	).Scan(&gpuCount)
	if err != nil {
		t.Fatal(err)
	}

	if gpuCount != 1 {
		t.Fatalf("expected 1 GPU, got %d", gpuCount)
	}

	// 5. 查询历史快照。
	var snapshotCount int
	err = pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM resource_snapshots WHERE node_id = $1",
		"test-node-001",
	).Scan(&snapshotCount)
	if err != nil {
		t.Fatal(err)
	}

	if snapshotCount != 2 {
		t.Fatalf("expected 2 snapshots, got %d", snapshotCount)
	}
	// 6. 测试资源历史分页查询。
	history, err := repo.GetResourceHistory(
		ctx,
		"test-node-001",
		nil,
		nil,
		1,
		0,
	)
	if err != nil {
		t.Fatalf("get resource history failed: %v", err)
	}

	if history.Total != 2 {
		t.Fatalf("expected 2 history records, got %d", history.Total)
	}

	if len(history.Records) != 1 {
		t.Fatalf("expected 1 history record, got %d", len(history.Records))
	}

	// 验证倒序排列，第一页应该是最新记录。
	if !history.Records[0].CollectedAt.Equal(
		time.Date(2026, 11, 2, 14, 0, 10, 0, time.UTC),
	) {
		t.Fatalf("unexpected latest history time: %v",
			history.Records[0].CollectedAt)
	}

	// 7. 测试分页第二页。
	history, err = repo.GetResourceHistory(
		ctx,
		"test-node-001",
		nil,
		nil,
		1,
		1,
	)
	if err != nil {
		t.Fatalf("get second history page failed: %v", err)
	}

	if len(history.Records) != 1 {
		t.Fatalf("expected 1 record on second page, got %d",
			len(history.Records))
	}

	// 8. 测试时间范围查询。
	from := time.Date(2026, 11, 2, 14, 0, 5, 0, time.UTC)
	to := time.Date(2026, 11, 2, 14, 0, 15, 0, time.UTC)

	history, err = repo.GetResourceHistory(
		ctx,
		"test-node-001",
		&from,
		&to,
		10,
		0,
	)
	if err != nil {
		t.Fatalf("get history by time range failed: %v", err)
	}

	if history.Total != 1 {
		t.Fatalf("expected 1 record in time range, got %d",
			history.Total)
	}

	// 9. 测试查询最新资源快照。
	latest, err := repo.GetLatestResourceSnapshot(
		ctx,
		"test-node-001",
	)
	if err != nil {
		t.Fatalf("get latest snapshot failed: %v", err)
	}

	if latest == nil {
		t.Fatal("expected latest snapshot, got nil")
	}

	if !latest.CollectedAt.Equal(
		time.Date(2026, 11, 2, 14, 0, 10, 0, time.UTC),
	) {
		t.Fatalf("unexpected latest snapshot time: %v",
			latest.CollectedAt)
	}

	// 10. 测试不存在历史记录的节点。
	empty, err := repo.GetLatestResourceSnapshot(
		ctx,
		"non-existent-node",
	)
	if err != nil {
		t.Fatalf("unexpected error for missing snapshot: %v", err)
	}

	if empty != nil {
		t.Fatal("expected nil snapshot for non-existent node")
	}

	// 11. 测试非法分页参数。
	_, err = repo.GetResourceHistory(
		ctx,
		"test-node-001",
		nil,
		nil,
		0,
		0,
	)
	if err == nil {
		t.Fatal("expected error for invalid limit")
	}
	// 12. 验证离线判定。
	_, err = pool.Exec(ctx, `
		UPDATE nodes
		SET last_heartbeat = NOW() - INTERVAL '60 seconds'
		WHERE node_id = $1
	`, "test-node-001")
	if err != nil {
		t.Fatal(err)
	}

	got, err = repo.GetNode(ctx, "test-node-001")
	if err != nil {
		t.Fatal(err)
	}

	if string(got.Status) != "offline" {
		t.Fatalf("expected offline, got %s", got.Status)
	}
}
