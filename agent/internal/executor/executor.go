package executor

import (
	"context"
	"errors"
	"log"
	"time"
)

// Run 启动 Agent 周期任务。
//
// collect 是每次执行的资源采集任务。
// interval 是资源采集周期。
//
// 启动时立即执行一次，之后按照 interval 周期执行。
// 单次采集失败不会导致 Agent 退出。
func Run(
	ctx context.Context,
	interval time.Duration,
	collect func() error,
) error {
	if ctx == nil {
		return errors.New("context cannot be nil")
	}

	if interval <= 0 {
		return errors.New("collection interval must be greater than zero")
	}

	if collect == nil {
		return errors.New("collect function cannot be nil")
	}

	// 启动时立即采集一次。
	runCollection(collect)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-ticker.C:
			// 周期性采集。
			runCollection(collect)
		}
	}
}

// runCollection 执行一次采集任务。
func runCollection(collect func() error) {
	if err := collect(); err != nil {
		log.Printf("Resource collection failed: %v", err)
	}
}
