package identity

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Identity 表示当前 Worker 节点的唯一身份。
type Identity struct {
	NodeID    string    `json:"node_id"`
	CreatedAt time.Time `json:"created_at"`
}

// LoadOrCreate 加载已有节点身份。
// 如果身份文件不存在，则生成新的 UUID 并持久化。
func LoadOrCreate(path string) (*Identity, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("identity file path cannot be empty")
	}

	// 优先读取已有身份文件。
	data, err := os.ReadFile(path)
	if err == nil {
		var id Identity

		if err := json.Unmarshal(data, &id); err != nil {
			return nil, fmt.Errorf("decode identity file: %w", err)
		}

		if !isValidUUID(id.NodeID) {
			return nil, fmt.Errorf("invalid node_id in identity file")
		}

		if id.CreatedAt.IsZero() {
			return nil, fmt.Errorf("created_at is missing in identity file")
		}

		return &id, nil
	}

	// 只有文件不存在时才允许创建新身份。
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read identity file: %w", err)
	}

	// 创建身份文件所在目录。
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create identity directory: %w", err)
	}

	// 生成新的 UUID。
	nodeID, err := generateUUID()
	if err != nil {
		return nil, fmt.Errorf("generate node ID: %w", err)
	}

	id := &Identity{
		NodeID:    nodeID,
		CreatedAt: time.Now().UTC(),
	}

	// 序列化为 JSON。
	data, err = json.MarshalIndent(id, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode identity: %w", err)
	}

	// 使用临时文件写入，避免程序中断时留下不完整的身份文件。
	tempFile, err := os.CreateTemp(dir, ".node-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("create temporary identity file: %w", err)
	}

	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	if _, err := tempFile.Write(data); err != nil {
		tempFile.Close()
		return nil, fmt.Errorf("write identity file: %w", err)
	}

	if err := tempFile.Sync(); err != nil {
		tempFile.Close()
		return nil, fmt.Errorf("sync identity file: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		return nil, fmt.Errorf("close identity file: %w", err)
	}

	// Windows 和 Linux 下都使用同目录临时文件进行原子替换。
	if err := os.Rename(tempPath, path); err != nil {
		// 如果另一个 Agent 实例已经创建了身份文件，则读取已有身份。
		if _, statErr := os.Stat(path); statErr == nil {
			return LoadOrCreate(path)
		}

		return nil, fmt.Errorf("persist identity file: %w", err)
	}

	return id, nil
}

// generateUUID 生成符合 UUID v4 格式的随机标识。
func generateUUID() (string, error) {
	var uuid [16]byte

	if _, err := rand.Read(uuid[:]); err != nil {
		return "", err
	}

	// 设置 UUID version 4。
	uuid[6] = (uuid[6] & 0x0f) | 0x40

	// 设置 UUID variant。
	uuid[8] = (uuid[8] & 0x3f) | 0x80

	encoded := make([]byte, 36)

	hex.Encode(encoded[0:8], uuid[0:4])
	encoded[8] = '-'

	hex.Encode(encoded[9:13], uuid[4:6])
	encoded[13] = '-'

	hex.Encode(encoded[14:18], uuid[6:8])
	encoded[18] = '-'

	hex.Encode(encoded[19:23], uuid[8:10])
	encoded[23] = '-'

	hex.Encode(encoded[24:36], uuid[10:16])

	return string(encoded), nil
}

// isValidUUID 验证 UUID v4 格式。
func isValidUUID(value string) bool {
	if len(value) != 36 {
		return false
	}

	if value[8] != '-' ||
		value[13] != '-' ||
		value[18] != '-' ||
		value[23] != '-' {
		return false
	}

	raw := strings.ReplaceAll(value, "-", "")

	if len(raw) != 32 {
		return false
	}

	decoded := make([]byte, 16)

	if _, err := hex.Decode(decoded, []byte(raw)); err != nil {
		return false
	}

	// 检查 UUID version 和 variant。
	return decoded[6]>>4 == 4 && decoded[8]>>6 == 2
}
