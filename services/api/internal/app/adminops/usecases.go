package adminops

import (
	"context"
	"encoding/base64"
	"errors"

	domain "github.com/ghanageo/ghanageo/services/api/internal/domain/adminops"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/audit"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/dataset"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
)

type Repository interface {
	Dashboard(context.Context) (domain.Dashboard, error)
	Health(context.Context) (domain.Health, error)
	Audit(context.Context, AuditQuery) (domain.Page[audit.Entry], error)
	SourceRuns(context.Context, PageQuery) (domain.Page[domain.SourceRun], error)
	SourceRun(context.Context, string) (domain.SourceRun, error)
	SourceRecords(context.Context, string, SourceRecordQuery) (domain.SourceRecordPage, error)
	SourceRecord(context.Context, string, string) (domain.SourceRecord, error)
	DatasetReleases(context.Context, PageQuery) (domain.Page[dataset.Version], error)
}

type PageQuery struct {
	Cursor string
	Limit  int
}
type AuditQuery struct {
	PageQuery
	Actor, Action, Target, Outcome string
}
type SourceRecordQuery struct {
	PageQuery
	ReasonCode string
}

type Service struct{ repo Repository }

func NewService(repo Repository) *Service { return &Service{repo: repo} }

func validatePage(q PageQuery) error {
	if q.Limit < 0 || q.Limit > 100 {
		return apierr.New(apierr.InvalidArgument, "Limit must be between 1 and 100.")
	}
	if q.Cursor != "" {
		if _, err := base64.RawURLEncoding.DecodeString(q.Cursor); err != nil {
			return apierr.New(apierr.InvalidArgument, "Cursor is invalid.")
		}
	}
	return nil
}

func (s *Service) Dashboard(ctx context.Context) (domain.Dashboard, error) {
	v, err := s.repo.Dashboard(ctx)
	if err != nil {
		return domain.Dashboard{}, apierr.Wrap(apierr.Internal, "Could not read the operator dashboard.", err)
	}
	return v, nil
}
func (s *Service) Health(ctx context.Context) (domain.Health, error) {
	v, err := s.repo.Health(ctx)
	if err != nil {
		return domain.Health{}, apierr.Wrap(apierr.Internal, "Could not read system health.", err)
	}
	return v, nil
}
func (s *Service) Audit(ctx context.Context, q AuditQuery) (domain.Page[audit.Entry], error) {
	if err := validatePage(q.PageQuery); err != nil {
		return domain.Page[audit.Entry]{}, err
	}
	if q.Outcome != "" && q.Outcome != string(audit.OutcomeSucceeded) && q.Outcome != string(audit.OutcomeFailed) {
		return domain.Page[audit.Entry]{}, apierr.New(apierr.InvalidArgument, "Outcome must be succeeded or failed.")
	}
	v, err := s.repo.Audit(ctx, q)
	if err != nil {
		return domain.Page[audit.Entry]{}, apierr.Wrap(apierr.Internal, "Could not read the audit log.", err)
	}
	return v, nil
}
func (s *Service) SourceRuns(ctx context.Context, q PageQuery) (domain.Page[domain.SourceRun], error) {
	if err := validatePage(q); err != nil {
		return domain.Page[domain.SourceRun]{}, err
	}
	v, err := s.repo.SourceRuns(ctx, q)
	if err != nil {
		return domain.Page[domain.SourceRun]{}, apierr.Wrap(apierr.Internal, "Could not read source runs.", err)
	}
	return v, nil
}
func (s *Service) SourceRun(ctx context.Context, id string) (domain.SourceRun, error) {
	v, err := s.repo.SourceRun(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.SourceRun{}, apierr.New(apierr.NotFound, "No such source run.")
	}
	if err != nil {
		return domain.SourceRun{}, apierr.Wrap(apierr.Internal, "Could not read the source run.", err)
	}
	return v, nil
}
func (s *Service) SourceRecords(ctx context.Context, runID string, q SourceRecordQuery) (domain.SourceRecordPage, error) {
	if err := validatePage(q.PageQuery); err != nil {
		return domain.SourceRecordPage{}, err
	}
	v, err := s.repo.SourceRecords(ctx, runID, q)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.SourceRecordPage{}, apierr.New(apierr.NotFound, "No such source run.")
	}
	if err != nil {
		return domain.SourceRecordPage{}, apierr.Wrap(apierr.Internal, "Could not read source records.", err)
	}
	return v, nil
}
func (s *Service) SourceRecord(ctx context.Context, runID, recordID string) (domain.SourceRecord, error) {
	v, err := s.repo.SourceRecord(ctx, runID, recordID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.SourceRecord{}, apierr.New(apierr.NotFound, "No such source record.")
	}
	if err != nil {
		return domain.SourceRecord{}, apierr.Wrap(apierr.Internal, "Could not read the source record.", err)
	}
	return v, nil
}
func (s *Service) DatasetReleases(ctx context.Context, q PageQuery) (domain.Page[dataset.Version], error) {
	if err := validatePage(q); err != nil {
		return domain.Page[dataset.Version]{}, err
	}
	v, err := s.repo.DatasetReleases(ctx, q)
	if err != nil {
		return domain.Page[dataset.Version]{}, apierr.Wrap(apierr.Internal, "Could not read dataset releases.", err)
	}
	return v, nil
}
