package main

import (
	"log"
	"net/http"

	"gpu-pool/server/internal/api"
	"gpu-pool/server/internal/node"
)

func main() {
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

	// Initialize in-memory node registry.
	registry := node.NewRegistry()

	// Register API routes.
	handler := api.NewHandler(registry)
	handler.RegisterRoutes(mux)

	addr := ":8080"

	log.Printf("Coordinator starting on %s", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Coordinator stopped: %v", err)
	}
}
