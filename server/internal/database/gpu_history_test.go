package database

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestGPUHistoryIntegration(t *testing.T) {
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

	schema := fmt.Sprintf("gpu_history_test_%d", time.Now().UnixNano())

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

	// 准备测试时间。
	t1 := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	t2 := t1.Add(10 * time.Second)

	// 创建测试节点。
	for _, n := range []struct {
		id       string
		hostname string
	}{
		{"node-a", "worker-a"},
		{"node-b", "worker-b"},
	} {
		_, err := pool.Exec(ctx, `
			INSERT INTO nodes (
				node_id, hostname, status,
				collected_at, last_heartbeat, latest_snapshot
			)
			VALUES ($1, $2, 'online', $3, NOW(), '{}'::jsonb)
		`, n.id, n.hostname, t2)

		if err != nil {
			t.Fatalf("insert node %s: %v", n.id, err)
		}
	}

	// 插入 GPU 历史快照。
	insertSnapshot := func(
		nodeID string,
		collectedAt time.Time,
		gpuInfo string,
	) {
		t.Helper()

		_, err := pool.Exec(ctx, `
			INSERT INTO resource_snapshots (
				node_id, collected_at, system_info, gpu_info
			)
			VALUES ($1, $2, '{}'::jsonb, $3::jsonb)
		`, nodeID, collectedAt, gpuInfo)

		if err != nil {
			t.Fatalf("insert snapshot: %v", err)
		}
	}

	// 节点 A：第一次心跳有两张 GPU。
	insertSnapshot("node-a", t1, `[
		{
			"index": 0,
			"uuid": "GPU-A0",
			"name": "RTX 3050",
			"memory_total_mb": 4096,
			"memory_used_mb": 1000,
			"memory_free_mb": 3096,
			"utilization": 20,
			"temperature": 45,
			"power_w": 10.5,
			"driver_version": "572.83"
		},
		{
			"index": 1,
			"uuid": "GPU-A1",
			"name": "RTX 4060",
			"memory_total_mb": 8192,
			"memory_used_mb": 2000,
			"memory_free_mb": 6192,
			"utilization": 40,
			"temperature": 50,
			"power_w": 20.5,
			"driver_version": "572.83"
		}
	]`)

	// 节点 A：第二次心跳，GPU 0 资源发生变化。
	insertSnapshot("node-a", t2, `[
		{
			"index": 0,
			"uuid": "GPU-A0",
			"name": "RTX 3050",
			"memory_total_mb": 4096,
			"memory_used_mb": 1500,
			"memory_free_mb": 2596,
			"utilization": 60,
			"temperature": 55,
			"power_w": 30.5,
			"driver_version": "572.83"
		}
	]`)

	// 节点 B：第一次心跳有一张 GPU。
	insertSnapshot("node-b", t1, `[
		{
			"index": 0,
			"uuid": "GPU-B0",
			"name": "RTX 4090",
			"memory_total_mb": 24576,
			"memory_used_mb": 8000,
			"memory_free_mb": 16576,
			"utilization": 75,
			"temperature": 65,
			"power_w": 200.5,
			"driver_version": "572.83"
		}
	]`)

	// 1. 查询全部节点 GPU 历史。
	all, err := repo.GetGPUHistory(
		ctx, "", nil, nil, nil, 10, 0,
	)
	if err != nil {
		t.Fatalf("query all GPU history: %v", err)
	}

	if all.Total != 4 || len(all.Records) != 4 {
		t.Fatalf("expected 4 records, got total=%d records=%d",
			all.Total, len(all.Records))
	}

	// 2. 验证时间倒序。
	if !all.Records[0].CollectedAt.Equal(t2) {
		t.Fatalf("expected latest record first, got %v",
			all.Records[0].CollectedAt)
	}

	// 3. 查询指定节点。
	byNode, err := repo.GetGPUHistory(
		ctx, "node-a", nil, nil, nil, 10, 0,
	)
	if err != nil {
		t.Fatal(err)
	}

	if byNode.Total != 3 {
		t.Fatalf("expected 3 records for node-a, got %d",
			byNode.Total)
	}

	// 4. 查询指定 GPU 序号。
	gpuIndex := 0

	byGPU, err := repo.GetGPUHistory(
		ctx, "node-a", &gpuIndex, nil, nil, 10, 0,
	)
	if err != nil {
		t.Fatal(err)
	}

	if byGPU.Total != 2 {
		t.Fatalf("expected 2 GPU 0 records, got %d",
			byGPU.Total)
	}

	if byGPU.Records[0].MemoryUsedMB != 1500 {
		t.Fatalf("unexpected memory usage: %d",
			byGPU.Records[0].MemoryUsedMB)
	}

	if byGPU.Records[0].Utilization != 60 {
		t.Fatalf("unexpected utilization: %d",
			byGPU.Records[0].Utilization)
	}

	// 5. 查询指定时间范围。
	timeRange, err := repo.GetGPUHistory(
		ctx, "", nil, &t1, &t1, 10, 0,
	)
	if err != nil {
		t.Fatal(err)
	}

	if timeRange.Total != 3 {
		t.Fatalf("expected 3 records in time range, got %d",
			timeRange.Total)
	}

	// 6. 验证分页。
	page, err := repo.GetGPUHistory(
		ctx, "", nil, nil, nil, 2, 1,
	)
	if err != nil {
		t.Fatal(err)
	}

	if page.Total != 4 || len(page.Records) != 2 {
		t.Fatalf("unexpected pagination: total=%d records=%d",
			page.Total, len(page.Records))
	}

	// 7. 查询不存在的节点。
	empty, err := repo.GetGPUHistory(
		ctx, "node-not-exist", nil, nil, nil, 10, 0,
	)
	if err != nil {
		t.Fatal(err)
	}

	if empty.Total != 0 || len(empty.Records) != 0 {
		t.Fatalf("expected empty result, got %d records",
			empty.Total)
	}

	// 8. 验证 GPU 历史数据字段。
	record := byGPU.Records[0]

	if record.GPUUUID != "GPU-A0" {
		t.Fatalf("unexpected GPU UUID: %s", record.GPUUUID)
	}

	if record.Hostname != "worker-a" {
		t.Fatalf("unexpected hostname: %s", record.Hostname)
	}

	if record.MemoryTotalMB != 4096 {
		t.Fatalf("unexpected total memory: %d",
			record.MemoryTotalMB)
	}

	if record.Temperature != 55 {
		t.Fatalf("unexpected temperature: %d",
			record.Temperature)
	}

	if record.PowerW != 30.5 {
		t.Fatalf("unexpected power: %f", record.PowerW)
	}

	if record.DriverVersion != "572.83" {
		t.Fatalf("unexpected driver version: %s",
			record.DriverVersion)
	}

	t.Log("GPU history integration tests passed")
}

// TestGPUHistoryInvalidParams 验证 GPU 历史查询参数校验。
func TestGPUHistoryInvalidParams(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	repo, err := NewNodeRepository(pool)
	if err != nil {
		t.Fatal(err)
	}

	negativeGPU := -1
	from := time.Date(2026, 11, 5, 0, 0, 10, 0, time.UTC)
	to := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		gpuIndex *int
		from     *time.Time
		to       *time.Time
		limit    int
		offset   int
	}{
		{
			name:  "zero limit",
			limit: 0,
		},
		{
			name:   "limit too large",
			limit:  201,
			offset: 0,
		},
		{
			name:   "negative offset",
			limit:  10,
			offset: -1,
		},
		{
			name:     "negative GPU index",
			gpuIndex: &negativeGPU,
			limit:    10,
		},
		{
			name:  "invalid time range",
			from:  &from,
			to:    &to,
			limit: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := repo.GetGPUHistory(
				ctx,
				"",
				tt.gpuIndex,
				tt.from,
				tt.to,
				tt.limit,
				tt.offset,
			)

			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
		})
	}
}
