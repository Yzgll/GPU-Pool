package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/joho/godotenv"

	"gpu-pool/server/internal/api"
	"gpu-pool/server/internal/database"
)

func main() {
	// 加载项目根目录的环境变量文件。
	if err := godotenv.Load("../.env"); err != nil {
		log.Printf("No .env file loaded: %v", err)
	}

	// 启动时检查 PostgreSQL 连接。
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := database.NewPool(ctx)
	if err != nil {
		log.Fatalf("Database initialization failed: %v", err)
	}
	defer pool.Close()

	log.Println("PostgreSQL connection established")
	// 启动时自动执行数据库迁移。
	migrationCtx, migrationCancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)

	if err := database.Migrate(
		migrationCtx,
		pool,
		"./migrations",
	); err != nil {
		migrationCancel()
		log.Fatalf("Database migration failed: %v", err)
	}

	migrationCancel()

	log.Println("Database migrations completed")

	mux := http.NewServeMux()

	// Health check.
	mux.HandleFunc("GET /health", func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","service":"coordinator"}`))
	})

	// 初始化 PostgreSQL Repository。
	repository, err := database.NewNodeRepository(pool)
	if err != nil {
		log.Fatalf("Initialize node repository failed: %v", err)
	}

	// 将 API 请求接入 PostgreSQL。
	handler := api.NewHandler(repository)
	handler.RegisterRoutes(mux)

	addr := ":8080"

	log.Printf("Coordinator starting on %s", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Coordinator stopped: %v", err)
	}
}
