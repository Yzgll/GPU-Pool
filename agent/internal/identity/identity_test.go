package identity

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOrCreateFirstStart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", "node.json")

	id, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("LoadOrCreate failed: %v", err)
	}

	if !isValidUUID(id.NodeID) {
		t.Fatalf("invalid UUID: %s", id.NodeID)
	}

	if id.CreatedAt.IsZero() {
		t.Fatal("CreatedAt should not be zero")
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("identity file was not created: %v", err)
	}
}

func TestLoadOrCreateReuseIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "node.json")

	first, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("first LoadOrCreate failed: %v", err)
	}

	second, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("second LoadOrCreate failed: %v", err)
	}

	if first.NodeID != second.NodeID {
		t.Fatalf(
			"node ID changed: first=%s second=%s",
			first.NodeID,
			second.NodeID,
		)
	}

	if !first.CreatedAt.Equal(second.CreatedAt) {
		t.Fatal("CreatedAt changed after reload")
	}
}

func TestLoadOrCreateCorruptedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "node.json")

	if err := os.WriteFile(path, []byte("{invalid json"), 0600); err != nil {
		t.Fatalf("write corrupted file failed: %v", err)
	}

	_, err := LoadOrCreate(path)
	if err == nil {
		t.Fatal("expected error for corrupted identity file")
	}
}

func TestLoadOrCreateInvalidUUID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "node.json")

	content := `{
		"node_id": "invalid-uuid",
		"created_at": "2026-11-01T00:00:00Z"
	}`

	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write identity file failed: %v", err)
	}

	_, err := LoadOrCreate(path)
	if err == nil {
		t.Fatal("expected error for invalid UUID")
	}
}

func TestLoadOrCreateMissingCreatedAt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "node.json")

	content := `{
		"node_id": "7f3b9c2e-8a41-4d62-9b75-1c8e6a4f203d"
	}`

	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write identity file failed: %v", err)
	}

	_, err := LoadOrCreate(path)
	if err == nil {
		t.Fatal("expected error for missing CreatedAt")
	}
}

func TestLoadOrCreateEmptyPath(t *testing.T) {
	_, err := LoadOrCreate("")
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestGenerateUUIDUnique(t *testing.T) {
	first, err := generateUUID()
	if err != nil {
		t.Fatalf("generate first UUID failed: %v", err)
	}

	second, err := generateUUID()
	if err != nil {
		t.Fatalf("generate second UUID failed: %v", err)
	}

	if first == second {
		t.Fatal("generated UUIDs should be different")
	}

	if !isValidUUID(first) || !isValidUUID(second) {
		t.Fatal("generated UUID has invalid format")
	}
}

func TestIsValidUUID(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{
			name:  "valid UUID v4",
			value: "7f3b9c2e-8a41-4d62-9b75-1c8e6a4f203d",
			want:  true,
		},
		{
			name:  "invalid length",
			value: "123456",
			want:  false,
		},
		{
			name:  "invalid version",
			value: "7f3b9c2e-8a41-1d62-9b75-1c8e6a4f203d",
			want:  false,
		},
		{
			name:  "invalid variant",
			value: "7f3b9c2e-8a41-4d62-1b75-1c8e6a4f203d",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidUUID(tt.value)
			if got != tt.want {
				t.Errorf("isValidUUID(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}
