package ingest

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
)

type countingSink struct {
	mu     sync.Mutex
	count  int
	closed bool
}

func (c *countingSink) Consume(_ context.Context, _ *coltracepb.ExportTraceServiceRequest) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.count++
	return nil
}

func (c *countingSink) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

func (c *countingSink) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.count
}

func (c *countingSink) Closed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

type blockingSink struct {
	startOnce sync.Once
	started   chan struct{}
	release   chan struct{}
}

func (b *blockingSink) Consume(_ context.Context, _ *coltracepb.ExportTraceServiceRequest) error {
	b.startOnce.Do(func() {
		close(b.started)
	})
	<-b.release
	return nil
}

func TestQueueSinkDrainsOnClose(t *testing.T) {
	sink := &countingSink{}
	queue := NewQueueSink(sink, QueueOptions{Size: 2})
	req := &coltracepb.ExportTraceServiceRequest{}
	for i := 0; i < 3; i++ {
		if err := queue.Consume(context.Background(), req); err != nil {
			t.Fatalf("consume: %v", err)
		}
	}
	if err := queue.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if got := sink.Count(); got != 3 {
		t.Fatalf("expected 3 requests consumed, got %d", got)
	}
	if !sink.Closed() {
		t.Fatalf("expected sink to be closed")
	}
}

func TestQueueSinkBackpressure(t *testing.T) {
	sink := &blockingSink{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	queue := NewQueueSink(sink, QueueOptions{Size: 1})
	req := &coltracepb.ExportTraceServiceRequest{}
	if err := queue.Consume(context.Background(), req); err != nil {
		t.Fatalf("consume: %v", err)
	}
	<-sink.started
	if err := queue.Consume(context.Background(), req); err != nil {
		t.Fatalf("consume: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := queue.Consume(ctx, req); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
	close(sink.release)
	if err := queue.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}
