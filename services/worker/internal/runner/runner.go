package runner

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/outbox"
)

type Queue interface {
	Claim(context.Context, string, time.Time, time.Duration) (*outbox.Event, error)
	Complete(context.Context, string, string, time.Time) error
	Fail(context.Context, outbox.Event, string, string, time.Time, int, time.Duration) error
	Depth(context.Context) (outbox.Depth, error)
}

type Handler func(context.Context, outbox.Event) error

type Runner struct {
	queue        Queue
	handle       Handler
	log          *slog.Logger
	workerID     string
	pollInterval time.Duration
	lease        time.Duration
	baseRetry    time.Duration
	maxAttempts  int
}

func New(queue Queue, handle Handler, log *slog.Logger, workerID string, pollInterval, lease, baseRetry time.Duration, maxAttempts int) *Runner {
	return &Runner{queue: queue, handle: handle, log: log, workerID: workerID, pollInterval: pollInterval, lease: lease, baseRetry: baseRetry, maxAttempts: maxAttempts}
}

// ProcessOne claims and handles at most one event. Its boolean result says
// whether work was claimed, which lets tests and one-shot operations avoid
// timing-based assertions.
func (r *Runner) ProcessOne(ctx context.Context) (bool, error) {
	now := time.Now().UTC()
	event, err := r.queue.Claim(ctx, r.workerID, now, r.lease)
	if err != nil || event == nil {
		return false, err
	}
	started := time.Now()
	if err := r.handle(ctx, *event); err != nil {
		delay := r.retryDelay(event.Attempts)
		if failErr := r.queue.Fail(ctx, *event, r.workerID, truncate(err.Error(), 2048), time.Now().UTC(), r.maxAttempts, delay); failErr != nil {
			return true, fmt.Errorf("handle %s: %v; record failure: %w", event.ID, err, failErr)
		}
		r.log.Error("outbox event failed", "event_id", event.ID, "topic", event.Topic, "attempt", event.Attempts, "retry_in", delay, "err", err)
		return true, nil
	}
	if err := r.queue.Complete(ctx, event.ID, r.workerID, time.Now().UTC()); err != nil {
		return true, err
	}
	r.log.Info("outbox event completed", "event_id", event.ID, "topic", event.Topic, "attempt", event.Attempts, "latency_ms", time.Since(started).Milliseconds())
	return true, nil
}

func (r *Runner) Run(ctx context.Context) error {
	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()
	depthTicker := time.NewTicker(30 * time.Second)
	defer depthTicker.Stop()
	for {
		worked, err := r.ProcessOne(ctx)
		if err != nil {
			r.log.Error("outbox poll failed", "err", err)
		}
		if worked {
			continue
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		case <-depthTicker.C:
			if depth, depthErr := r.queue.Depth(ctx); depthErr == nil {
				r.log.Info("outbox depth", "pending", depth.Pending, "processing", depth.Processing, "dead", depth.Dead)
			}
		}
	}
}

func (r *Runner) retryDelay(attempt int) time.Duration {
	exponent := math.Max(0, math.Min(float64(attempt-1), 8))
	return time.Duration(float64(r.baseRetry) * math.Pow(2, exponent))
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}
