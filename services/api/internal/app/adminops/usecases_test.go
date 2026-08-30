package adminops

import (
	"context"
	"testing"

	domain "github.com/ghanageo/ghanageo/services/api/internal/domain/adminops"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/audit"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/dataset"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
)

type recordingRepo struct{ called bool }

func (r *recordingRepo) Dashboard(context.Context) (domain.Dashboard, error) {
	r.called = true
	return domain.Dashboard{}, nil
}
func (r *recordingRepo) Health(context.Context) (domain.Health, error) {
	r.called = true
	return domain.Health{}, nil
}
func (r *recordingRepo) Audit(context.Context, AuditQuery) (domain.Page[audit.Entry], error) {
	r.called = true
	return domain.Page[audit.Entry]{}, nil
}
func (r *recordingRepo) SourceRuns(context.Context, PageQuery) (domain.Page[domain.SourceRun], error) {
	r.called = true
	return domain.Page[domain.SourceRun]{}, nil
}
func (r *recordingRepo) SourceRun(context.Context, string) (domain.SourceRun, error) {
	r.called = true
	return domain.SourceRun{}, nil
}
func (r *recordingRepo) SourceRecords(context.Context, string, SourceRecordQuery) (domain.SourceRecordPage, error) {
	r.called = true
	return domain.SourceRecordPage{}, nil
}
func (r *recordingRepo) SourceRecord(context.Context, string, string) (domain.SourceRecord, error) {
	r.called = true
	return domain.SourceRecord{}, nil
}
func (r *recordingRepo) DatasetReleases(context.Context, PageQuery) (domain.Page[dataset.Version], error) {
	r.called = true
	return domain.Page[dataset.Version]{}, nil
}

func TestInvalidPaginationIsRejectedBeforeRepository(t *testing.T) {
	for _, q := range []PageQuery{{Limit: 101}, {Limit: -1}, {Cursor: "not base64***"}} {
		repo := &recordingRepo{}
		_, err := NewService(repo).SourceRuns(context.Background(), q)
		if apierr.From(err).Code != apierr.InvalidArgument {
			t.Fatalf("error = %v, want INVALID_ARGUMENT", err)
		}
		if repo.called {
			t.Fatal("repository called for invalid pagination")
		}
	}
}

func TestAuditOutcomeAllowList(t *testing.T) {
	repo := &recordingRepo{}
	_, err := NewService(repo).Audit(context.Background(), AuditQuery{Outcome: "maybe"})
	if apierr.From(err).Code != apierr.InvalidArgument {
		t.Fatalf("error = %v, want INVALID_ARGUMENT", err)
	}
	if repo.called {
		t.Fatal("repository called for invalid outcome")
	}
}
