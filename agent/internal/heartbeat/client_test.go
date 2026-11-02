package heartbeat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		timeout  time.Duration
		wantErr  bool
	}{
		{"valid endpoint", "http://127.0.0.1:8080", 5 * time.Second, false},
		{"empty endpoint", "", 5 * time.Second, true},
		{"invalid scheme", "ftp://127.0.0.1:8080", 5 * time.Second, true},
		{"invalid timeout", "http://127.0.0.1:8080", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.endpoint, tt.timeout)
			if (err != nil) != tt.wantErr {
				t.Fatalf("unexpected error: %v", err)
			}
			if err == nil && client == nil {
				t.Fatal("expected client, got nil")
			}
		})
	}
}

func TestSendSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}

			if r.URL.Path != "/api/v1/heartbeat" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}

			if r.Header.Get("Content-Type") != "application/json" {
				t.Error("unexpected Content-Type")
			}

			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decode body: %v", err)
			}

			if payload["node_id"] != "test-node-001" {
				t.Errorf("unexpected node_id: %v", payload["node_id"])
			}

			w.WriteHeader(http.StatusOK)
		},
	))
	defer server.Close()

	client, err := NewClient(server.URL, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	err = client.Send(context.Background(), map[string]any{
		"node_id":  "test-node-001",
		"hostname": "test-host",
	})
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
}

func TestSendServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
	))
	defer server.Close()

	client, err := NewClient(server.URL, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	err = client.Send(context.Background(), map[string]string{
		"node_id": "test-node",
	})
	if err == nil {
		t.Fatal("expected server error")
	}
}

func TestSendTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(200 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		},
	))
	defer server.Close()

	client, err := NewClient(server.URL, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	err = client.Send(context.Background(), map[string]string{
		"node_id": "test-node",
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestSendNilPayload(t *testing.T) {
	client, err := NewClient("http://127.0.0.1:8080", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	if err := client.Send(context.Background(), nil); err == nil {
		t.Fatal("expected nil payload error")
	}
}

func TestSendCanceledContext(t *testing.T) {
	client, err := NewClient("http://127.0.0.1:8080", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = client.Send(ctx, map[string]string{
		"node_id": "test-node",
	})
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}
