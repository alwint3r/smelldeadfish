package ingest

import (
	"context"
	"errors"
	"log"
	"sync"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
)

var ErrQueueClosed = errors.New("trace queue closed")

type QueueOptions struct {
	Size   int
	Logger *log.Logger
}

type QueueSink struct {
	sink      TraceSink
	queue     chan *coltracepb.ExportTraceServiceRequest
	closed    chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
	logger    *log.Logger
}

func NewQueueSink(sink TraceSink, opts QueueOptions) *QueueSink {
	size := opts.Size
	if size <= 0 {
		if opts.Logger != nil {
			opts.Logger.Printf("msg=queue_sink_invalid_size size=%d fallback=1", size)
		}
		size = 1
	}
	queue := &QueueSink{
		sink:   sink,
		queue:  make(chan *coltracepb.ExportTraceServiceRequest, size),
		closed: make(chan struct{}),
		logger: opts.Logger,
	}
	queue.wg.Add(1)
	go queue.run()
	return queue
}

func (q *QueueSink) Consume(ctx context.Context, req *coltracepb.ExportTraceServiceRequest) error {
	if q == nil || q.sink == nil || req == nil {
		return nil
	}
	select {
	case <-q.closed:
		return ErrQueueClosed
	default:
	}
	select {
	case q.queue <- req:
		return nil
	case <-q.closed:
		return ErrQueueClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (q *QueueSink) Close() error {
	if q == nil {
		return nil
	}
	didClose := false
	q.closeOnce.Do(func() {
		didClose = true
		close(q.closed)
	})
	q.wg.Wait()
	if didClose {
		if closer, ok := q.sink.(interface{ Close() error }); ok {
			return closer.Close()
		}
	}
	return nil
}

func (q *QueueSink) run() {
	defer q.wg.Done()
	for {
		select {
		case req := <-q.queue:
			q.consume(req)
		case <-q.closed:
			for {
				select {
				case req := <-q.queue:
					q.consume(req)
				default:
					return
				}
			}
		}
	}
}

func (q *QueueSink) consume(req *coltracepb.ExportTraceServiceRequest) {
	if req == nil || q.sink == nil {
		return
	}
	if err := q.sink.Consume(context.Background(), req); err != nil && q.logger != nil {
		q.logger.Printf("msg=queue_sink_consume_error error=%q", err.Error())
	}
}
