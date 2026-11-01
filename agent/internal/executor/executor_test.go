package executor

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunImmediateCollection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var count atomic.Int32

	done := make(chan error, 1)

	go func() {
		done <- Run(ctx, time.Hour, func() error {
			count.Add(1)
			return nil
		})
	}()

	deadline := time.After(time.Second)

	for count.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("initial collection was not executed")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	cancel()

	if err := <-done; err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if count.Load() != 1 {
		t.Errorf("expected 1 collection, got %d", count.Load())
	}
}

func TestRunPeriodicCollection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var count atomic.Int32

	done := make(chan error, 1)

	go func() {
		done <- Run(ctx, 50*time.Millisecond, func() error {
			count.Add(1)
			return nil
		})
	}()

	time.Sleep(180 * time.Millisecond)
	cancel()

	if err := <-done; err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if count.Load() < 3 {
		t.Errorf("expected at least 3 collections, got %d", count.Load())
	}
}

func TestRunInvalidInterval(t *testing.T) {
	err := Run(
		context.Background(),
		0,
		func() error {
			return nil
		},
	)

	if err == nil {
		t.Fatal("expected error for zero interval")
	}
}

func TestRunNilCollect(t *testing.T) {
	err := Run(
		context.Background(),
		time.Second,
		nil,
	)

	if err == nil {
		t.Fatal("expected error for nil collect function")
	}
}

func TestRunCollectionFailureContinues(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var count atomic.Int32

	done := make(chan error, 1)

	go func() {
		done <- Run(ctx, 50*time.Millisecond, func() error {
			count.Add(1)
			return errors.New("simulated collection failure")
		})
	}()

	time.Sleep(180 * time.Millisecond)
	cancel()

	if err := <-done; err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if count.Load() < 3 {
		t.Errorf("collection should continue after failures, got %d", count.Load())
	}
}

func TestRunNilContext(t *testing.T) {
	err := Run(
		nil,
		time.Second,
		func() error {
			return nil
		},
	)

	if err == nil {
		t.Fatal("expected error for nil context")
	}
}
