package monitor

import (
	"fmt"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

// CollectSystemInfo 采集当前节点的系统资源信息。
func CollectSystemInfo() (*SystemInfo, error) {
	info := &SystemInfo{
		Disks:    make([]DiskInfo, 0),
		Networks: make([]NetworkInfo, 0),
	}

	// 1. 采集 CPU 信息。
	cpuInfo, err := cpu.Info()
	if err != nil {
		return nil, fmt.Errorf("collect CPU info: %w", err)
	}

	if len(cpuInfo) > 0 {
		info.CPU.ModelName = strings.TrimSpace(cpuInfo[0].ModelName)
	}

	logicalCores, err := cpu.Counts(true)
	if err != nil {
		return nil, fmt.Errorf("collect CPU logical cores: %w", err)
	}
	info.CPU.LogicalCores = logicalCores

	// 采集 CPU 总体使用率。
	// interval=0 表示立即返回当前采样结果。
	cpuPercent, err := cpu.Percent(time.Second, false)
	if err != nil {
		return nil, fmt.Errorf("collect CPU usage: %w", err)
	}

	if len(cpuPercent) > 0 {
		info.CPU.UsagePercent = cpuPercent[0]
	}

	// 2. 采集物理内存信息。
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("collect memory info: %w", err)
	}

	info.Memory = MemoryInfo{
		TotalBytes:     memInfo.Total,
		UsedBytes:      memInfo.Used,
		AvailableBytes: memInfo.Available,
		UsedPercent:    memInfo.UsedPercent,
	}

	// 3. 采集磁盘分区信息。
	partitions, err := disk.Partitions(false)
	if err != nil {
		return nil, fmt.Errorf("collect disk partitions: %w", err)
	}

	for _, partition := range partitions {
		usage, err := disk.Usage(partition.Mountpoint)
		if err != nil {
			// 单个分区读取失败不影响其他分区。
			continue
		}

		info.Disks = append(info.Disks, DiskInfo{
			Device:      partition.Device,
			MountPoint:  partition.Mountpoint,
			FileSystem:  partition.Fstype,
			TotalBytes:  usage.Total,
			UsedBytes:   usage.Used,
			FreeBytes:   usage.Free,
			UsedPercent: usage.UsedPercent,
		})
	}

	// 4. 采集网络接口累计流量。
	netStats, err := net.IOCounters(true)
	if err != nil {
		return nil, fmt.Errorf("collect network info: %w", err)
	}

	for _, stat := range netStats {
		// 跳过无名称的接口。
		if strings.TrimSpace(stat.Name) == "" {
			continue
		}

		info.Networks = append(info.Networks, NetworkInfo{
			Name:      stat.Name,
			BytesSent: stat.BytesSent,
			BytesRecv: stat.BytesRecv,
		})
	}

	return info, nil
}
