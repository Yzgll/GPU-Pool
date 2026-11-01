package monitor

import (
	"testing"
)

func TestCollectSystemInfo(t *testing.T) {
	info, err := CollectSystemInfo()
	if err != nil {
		t.Fatalf("CollectSystemInfo failed: %v", err)
	}

	if info == nil {
		t.Fatal("system info should not be nil")
	}

	// CPU 检查
	if info.CPU.LogicalCores <= 0 {
		t.Errorf("invalid logical CPU cores: %d", info.CPU.LogicalCores)
	}

	if info.CPU.UsagePercent < 0 || info.CPU.UsagePercent > 100 {
		t.Errorf("invalid CPU usage: %.2f", info.CPU.UsagePercent)
	}

	t.Logf("CPU Model: %s", info.CPU.ModelName)
	t.Logf("CPU Logical Cores: %d", info.CPU.LogicalCores)
	t.Logf("CPU Usage: %.2f%%", info.CPU.UsagePercent)

	// 内存检查
	if info.Memory.TotalBytes == 0 {
		t.Error("total memory should be greater than zero")
	}

	if info.Memory.UsedBytes > info.Memory.TotalBytes {
		t.Errorf(
			"used memory exceeds total memory: used=%d total=%d",
			info.Memory.UsedBytes,
			info.Memory.TotalBytes,
		)
	}

	if info.Memory.UsedPercent < 0 || info.Memory.UsedPercent > 100 {
		t.Errorf("invalid memory usage: %.2f", info.Memory.UsedPercent)
	}

	t.Logf("Memory Total: %d bytes", info.Memory.TotalBytes)
	t.Logf("Memory Used: %d bytes", info.Memory.UsedBytes)
	t.Logf("Memory Available: %d bytes", info.Memory.AvailableBytes)
	t.Logf("Memory Usage: %.2f%%", info.Memory.UsedPercent)

	// 磁盘检查
	if len(info.Disks) == 0 {
		t.Error("no disk partitions detected")
	}

	for _, d := range info.Disks {
		if d.MountPoint == "" {
			t.Error("disk mount point should not be empty")
		}

		if d.TotalBytes == 0 {
			t.Errorf("disk %s has zero total capacity", d.MountPoint)
		}

		t.Logf(
			"Disk: device=%s mount=%s total=%d used=%d free=%d usage=%.2f%%",
			d.Device,
			d.MountPoint,
			d.TotalBytes,
			d.UsedBytes,
			d.FreeBytes,
			d.UsedPercent,
		)
	}

	// 网络检查
	if len(info.Networks) == 0 {
		t.Error("no network interfaces detected")
	}

	for _, n := range info.Networks {
		if n.Name == "" {
			t.Error("network interface name should not be empty")
		}

		t.Logf(
			"Network: name=%s sent=%d recv=%d",
			n.Name,
			n.BytesSent,
			n.BytesRecv,
		)
	}
}

func TestCollectSystemInfoRepeated(t *testing.T) {
	// 连续采集两次，验证采集过程可重复执行。
	first, err := CollectSystemInfo()
	if err != nil {
		t.Fatalf("first collection failed: %v", err)
	}

	second, err := CollectSystemInfo()
	if err != nil {
		t.Fatalf("second collection failed: %v", err)
	}

	if first == nil || second == nil {
		t.Fatal("system info should not be nil")
	}

	if first.Memory.TotalBytes != second.Memory.TotalBytes {
		t.Errorf(
			"memory total changed unexpectedly: first=%d second=%d",
			first.Memory.TotalBytes,
			second.Memory.TotalBytes,
		)
	}

	if first.CPU.LogicalCores != second.CPU.LogicalCores {
		t.Errorf(
			"CPU core count changed: first=%d second=%d",
			first.CPU.LogicalCores,
			second.CPU.LogicalCores,
		)
	}
}
