package runner

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/outbox"
)

type fakeQueue struct {
	event     *outbox.Event
	completed bool
	failed    bool
	dead      bool
	delay     time.Duration
}

func (q *fakeQueue) Claim(context.Context, string, time.Time, time.Duration) (*outbox.Event, error) {
	event := q.event
	q.event = nil
	return event, nil
}
func (q *fakeQueue) Complete(context.Context, string, string, time.Time) error {
	q.completed = true
	return nil
}
func (q *fakeQueue) Fail(_ context.Context, event outbox.Event, _ string, _ string, _ time.Time, max int, delay time.Duration) error {
	q.failed, q.dead, q.delay = true, event.Attempts >= max, delay
	return nil
}
func (q *fakeQueue) Depth(context.Context) (outbox.Depth, error) { return outbox.Depth{}, nil }

func TestProcessOneCompletesSuccessfulEvent(t *testing.T) {
	queue := &fakeQueue{event: &outbox.Event{ID: "evt", Topic: outbox.TopicDatasetPublished, Attempts: 1}}
	runner := New(queue, func(context.Context, outbox.Event) error { return nil }, discardLogger(), "worker", time.Second, time.Minute, time.Second, 5)
	worked, err := runner.ProcessOne(context.Background())
	if err != nil || !worked || !queue.completed || queue.failed {
		t.Fatalf("worked=%v completed=%v failed=%v err=%v", worked, queue.completed, queue.failed, err)
	}
}

func TestProcessOneRetriesThenDeadLetters(t *testing.T) {
	for _, tc := range []struct {
		attempts int
		dead     bool
		delay    time.Duration
	}{{2, false, 2 * time.Second}, {5, true, 16 * time.Second}} {
		queue := &fakeQueue{event: &outbox.Event{ID: "evt", Topic: "broken", Attempts: tc.attempts}}
		runner := New(queue, func(context.Context, outbox.Event) error { return errors.New("boom") }, discardLogger(), "worker", time.Second, time.Minute, time.Second, 5)
		worked, err := runner.ProcessOne(context.Background())
		if err != nil || !worked || !queue.failed || queue.dead != tc.dead || queue.delay != tc.delay {
			t.Fatalf("attempts=%d worked=%v failed=%v dead=%v delay=%v err=%v", tc.attempts, worked, queue.failed, queue.dead, queue.delay, err)
		}
	}
}

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
