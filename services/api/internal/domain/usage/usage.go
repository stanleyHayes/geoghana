// Package usage defines the secret-free request telemetry shown to developers.
package usage

import (
	"context"
	"time"
)

const Retention = 30 * 24 * time.Hour

type Event struct {
	ID             string
	RequestID      string
	OrganizationID string
	ApplicationID  string
	KeyID          string
	Protocol       string
	Operation      string
	Status         string
	Success        bool
	LatencyMS      int64
	QuotaCost      int
	QuotaLimit     int
	QuotaRemaining int
	Geography      string
	At             time.Time
}

type Breakdown struct {
	Label        string
	Requests     int64
	Errors       int64
	QuotaCost    int64
	AvgLatencyMS float64
}

type Summary struct {
	Since        time.Time
	Requests     int64
	Errors       int64
	QuotaCost    int64
	AvgLatencyMS float64
	ByProtocol   []Breakdown
	ByEndpoint   []Breakdown
	ByGeography  []Breakdown
}

type Page struct {
	Data       []Event
	NextBefore *time.Time
}

type Repository interface {
	Record(context.Context, Event) error
	Summary(context.Context, string, string, time.Time) (Summary, error)
	List(context.Context, string, string, *time.Time, int) (Page, error)
}
