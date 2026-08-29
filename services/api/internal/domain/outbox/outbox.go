// Package outbox defines durable background-work events.
package outbox

import "time"

const TopicDatasetPublished = "dataset.published"

type Status string

const (
	StatusPending    Status = "pending"
	StatusProcessing Status = "processing"
	StatusCompleted  Status = "completed"
	StatusDead       Status = "dead"
)

// Event is safe to retry. Handlers must be idempotent because a worker can
// complete a side effect and lose its lease before acknowledging it.
type Event struct {
	ID          string
	Topic       string
	Payload     map[string]any
	Status      Status
	Attempts    int
	AvailableAt time.Time
	CreatedAt   time.Time
	LockedAt    *time.Time
	LockedBy    string
	LastError   string
	CompletedAt *time.Time
}

type Depth struct {
	Pending, Processing, Dead int64
}
