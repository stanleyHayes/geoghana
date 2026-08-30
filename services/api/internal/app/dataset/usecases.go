// Package dataset serves the public dataset catalogue and its downloads
// (Spec §7.2, story GEO-8.3).
package dataset

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/audit"
	domain "github.com/ghanageo/ghanageo/services/api/internal/domain/dataset"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
)

// Repository is the persistence port for dataset versions.
type Repository interface {
	ListPublished(ctx context.Context) ([]domain.Version, error)
	ListAll(ctx context.Context) ([]domain.Version, error)
	Get(ctx context.Context, version string) (*domain.Version, error)
	Upsert(ctx context.Context, v domain.Version) error
	Activate(ctx context.Context, previous []domain.Version, next domain.Version) error
	ActivateAudited(ctx context.Context, previous []domain.Version, next domain.Version, evidence audit.Entry) error
	UpdateAudited(ctx context.Context, before, after domain.Version, evidence audit.Entry) error
}

// Service reads the dataset catalogue and opens artifact files.
type Service struct {
	repo Repository
	// exportDir is the root that every artifact resolves under. Nothing
	// outside it is ever opened.
	exportDir string
	auditSink AuditSink
}

type AuditSink interface {
	Append(context.Context, audit.Entry) (audit.Entry, error)
}

type Actor struct{ ID, Email, IP, RequestID string }

type Readiness struct {
	Version       string           `json:"version"`
	Status        string           `json:"status"`
	Ready         bool             `json:"ready"`
	ArtifactCount int              `json:"artifactCount"`
	TotalBytes    int64            `json:"totalBytes"`
	BytesVerified bool             `json:"bytesVerified"`
	Checks        []ReadinessCheck `json:"checks"`
}
type ReadinessCheck struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
}

func NewService(repo Repository, exportDir string) *Service {
	return &Service{repo: repo, exportDir: exportDir}
}

func (s *Service) WithAudit(sink AuditSink) *Service { s.auditSink = sink; return s }

func (s *Service) Readiness(ctx context.Context, version string) (Readiness, error) {
	v, err := s.repo.Get(ctx, strings.TrimSpace(version))
	if errors.Is(err, domain.ErrNotFound) {
		return Readiness{}, apierr.New(apierr.NotFound, "No such dataset version.").WithDetail("version", version)
	}
	if err != nil {
		return Readiness{}, apierr.Wrap(apierr.Internal, "Could not read the version.", err)
	}
	r := Readiness{Version: v.Version, Status: string(v.Status), ArtifactCount: len(v.Artifacts)}
	statusOK := v.Status == domain.StatusApproved || v.Status == domain.StatusRolledBack || v.Status == domain.StatusPublished
	r.Checks = append(r.Checks, ReadinessCheck{Name: "lifecycle", Passed: statusOK, Detail: "release must be approved, rolled back, or already published"})
	artifactsOK := len(v.Artifacts) > 0
	for _, a := range v.Artifacts {
		r.TotalBytes += a.SizeBytes
		if a.SafeFilename() != nil || a.SizeBytes <= 0 || a.RecordCount < 0 || len(a.SHA256) != 64 {
			artifactsOK = false
			r.Checks = append(r.Checks, ReadinessCheck{Name: "artifact:" + a.Entity + ":" + string(a.Format), Passed: false, Detail: "stored artifact metadata is invalid"})
			continue
		}
		path := filepath.Join(s.exportDir, v.Version, a.Filename)
		fh, openErr := os.Open(path)
		if openErr != nil {
			artifactsOK = false
			r.Checks = append(r.Checks, ReadinessCheck{Name: "artifact:" + a.Entity + ":" + string(a.Format), Passed: false, Detail: "artifact bytes are unavailable for verification"})
			continue
		}
		h := sha256.New()
		n, copyErr := io.Copy(h, fh)
		closeErr := fh.Close()
		verified := copyErr == nil && closeErr == nil && n == a.SizeBytes && strings.EqualFold(hex.EncodeToString(h.Sum(nil)), a.SHA256)
		if !verified {
			artifactsOK = false
		}
		detail := "size and SHA-256 match the stored release metadata"
		if !verified {
			detail = "artifact bytes could not be verified against stored size and SHA-256"
		}
		r.Checks = append(r.Checks, ReadinessCheck{Name: "artifact:" + a.Entity + ":" + string(a.Format), Passed: verified, Detail: detail})
	}
	r.BytesVerified = artifactsOK
	r.Checks = append(r.Checks, ReadinessCheck{Name: "artifacts", Passed: artifactsOK, Detail: "artifacts require safe names, byte sizes, record counts, and SHA-256 digests"})
	r.Ready = statusOK && artifactsOK
	return r, nil
}

func (s *Service) record(ctx context.Context, actor Actor, action audit.Action, version string, before, after map[string]any, reason string, cause error) {
	if s.auditSink == nil {
		return
	}
	e, err := audit.New(audit.Actor{Kind: audit.ActorAdmin, ID: actor.ID, Label: actor.Email, IP: actor.IP}, action, audit.Target{Kind: "dataset", ID: version, Label: version})
	if err != nil {
		return
	}
	e = e.WithChange(before, after).WithRequest(actor.RequestID).WithReason(reason)
	if cause != nil {
		e = e.Failed(cause)
	}
	_, _ = s.auditSink.Append(ctx, e)
}

func (s *Service) List(ctx context.Context) ([]domain.Version, error) {
	vs, err := s.repo.ListPublished(ctx)
	if err != nil {
		return nil, apierr.Wrap(apierr.Internal, "Could not list dataset versions.", err)
	}
	return vs, nil
}

// Downloads returns the artifacts for a published version.
//
// A version that exists but is not published is NOT_FOUND rather than
// forbidden: its existence is not public information, so distinguishing the
// two would leak the release schedule.
func (s *Service) Downloads(ctx context.Context, version string) (*domain.Version, error) {
	v, err := s.repo.Get(ctx, version)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, apierr.New(apierr.NotFound, "No such dataset version.").WithDetail("version", version)
	}
	if err != nil {
		return nil, apierr.Wrap(apierr.Internal, "Could not read dataset version.", err)
	}
	if !v.Status.Published() {
		return nil, apierr.New(apierr.NotFound, "No such dataset version.").WithDetail("version", version)
	}
	return v, nil
}

// OpenArtifact resolves an artifact to a readable file.
//
// The path is rebuilt from the RECORDED filename, never from the request, and
// the result is checked to still sit under exportDir after resolution. Two
// independent guards, because a path traversal here would serve any file the
// process can read.
func (s *Service) OpenArtifact(
	ctx context.Context, version, entity, format string,
) (io.ReadSeekCloser, domain.Artifact, error) {
	var zero domain.Artifact

	f, err := domain.ParseFormat(format)
	if err != nil {
		return nil, zero, apierr.New(apierr.InvalidArgument, "Unknown download format.").
			WithDetail("format", format)
	}

	v, err := s.Downloads(ctx, version)
	if err != nil {
		return nil, zero, err
	}

	a, err := v.FindArtifact(entity, f)
	if err != nil {
		return nil, zero, apierr.New(apierr.NotFound, "No such download for this version.").
			WithDetail("entity", entity).WithDetail("format", format)
	}
	if err := a.SafeFilename(); err != nil {
		return nil, zero, apierr.Wrap(apierr.Internal, "Artifact reference is unusable.", err)
	}

	root, err := filepath.Abs(filepath.Join(s.exportDir, version))
	if err != nil {
		return nil, zero, apierr.Wrap(apierr.Internal, "Could not resolve export directory.", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, zero, apierr.Wrap(apierr.Internal, "Could not resolve export directory.", err)
	}
	full := filepath.Join(root, a.Filename)
	// Resolve every symlink before containment checking. filepath.Abs alone
	// would allow a catalogued artifact symlink to escape the export root.
	resolved, err := filepath.EvalSymlinks(full)
	if err != nil || !isUnder(resolved, root) {
		return nil, zero, apierr.New(apierr.NotFound, "No such download for this version.")
	}

	fh, err := os.Open(resolved)
	if errors.Is(err, os.ErrNotExist) {
		// Catalogued but missing on disk. Say so precisely: this is an
		// operational fault on our side, not a bad request.
		return nil, zero, apierr.Wrap(apierr.Internal,
			"This download is catalogued but its file is missing.",
			fmt.Errorf("artifact %s/%s: %w", version, a.Filename, err))
	}
	if err != nil {
		return nil, zero, apierr.Wrap(apierr.Internal, "Could not open the download.", err)
	}
	return fh, a, nil
}

func isUnder(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !filepath.IsAbs(rel) &&
		!(len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator))
}

// Publish marks a version live, demoting whichever version was live before.
//
// Exactly one version is published at a time. Two live versions would make
// "the current dataset" ambiguous, and every consumer pinning to it would get
// a different answer depending on ordering.
func (s *Service) Publish(ctx context.Context, version, at string) (domain.Version, error) {
	return s.publish(ctx, version, at, nil)
}

func (s *Service) publish(ctx context.Context, version, at string, actor *Actor) (domain.Version, error) {
	v, err := s.repo.Get(ctx, version)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Version{}, apierr.New(apierr.NotFound, "No such dataset version.").
			WithDetail("version", version)
	}
	if err != nil {
		return domain.Version{}, apierr.Wrap(apierr.Internal, "Could not read the version.", err)
	}
	if v.Status == domain.StatusPublished {
		return *v, nil
	}
	if v.Status != domain.StatusApproved && v.Status != domain.StatusRolledBack {
		return domain.Version{}, apierr.New(apierr.InvalidArgument, "That version is not approved for publication.").WithDetail("status", string(v.Status))
	}
	if len(v.Artifacts) == 0 {
		// Publishing a version with no downloads would advertise a release
		// nobody can actually consume.
		return domain.Version{}, apierr.New(apierr.InvalidArgument,
			"That version has no artifacts. Build them first with `data export`.")
	}
	// Demote whatever is live first. Without this the catalogue can hold two
	// published versions, and "the current dataset" stops having an answer —
	// which is precisely what a versioned public dataset exists to prevent.
	live, err := s.repo.ListPublished(ctx)
	if err != nil {
		return domain.Version{}, apierr.Wrap(apierr.Internal, "Could not read the catalogue.", err)
	}
	previous := make([]domain.Version, 0, len(live))
	for _, c := range live {
		if c.Version == version {
			continue
		}
		c.Status = domain.StatusRolledBack
		previous = append(previous, c)
	}

	v.Status = domain.StatusPublished
	v.PublishedAt = at
	var activateErr error
	if actor == nil {
		activateErr = s.repo.Activate(ctx, previous, *v)
	} else {
		e, buildErr := audit.New(audit.Actor{Kind: audit.ActorAdmin, ID: actor.ID, Label: actor.Email, IP: actor.IP}, audit.ActionDatasetPublished, audit.Target{Kind: "dataset", ID: version, Label: version})
		if buildErr != nil {
			return domain.Version{}, apierr.Wrap(apierr.Internal, "Could not prepare audit evidence.", buildErr)
		}
		e = e.WithRequest(actor.RequestID).WithChange(map[string]any{"status": string(domain.StatusApproved)}, map[string]any{"status": string(v.Status), "artifacts": len(v.Artifacts)})
		activateErr = s.repo.ActivateAudited(ctx, previous, *v, e)
	}
	if activateErr != nil {
		return domain.Version{}, apierr.Wrap(apierr.Internal, "Could not publish.", activateErr)
	}
	return *v, nil
}

func (s *Service) PublishAs(ctx context.Context, actor Actor, version, confirmation, at string) (domain.Version, error) {
	if strings.TrimSpace(confirmation) != version {
		err := apierr.New(apierr.InvalidArgument, "Confirmation must exactly match the target version.")
		s.record(ctx, actor, audit.ActionDatasetPublished, version, nil, nil, "", err)
		return domain.Version{}, err
	}
	ready, err := s.Readiness(ctx, version)
	if err == nil && !ready.Ready {
		err = apierr.New(apierr.InvalidArgument, "Dataset release is not ready to publish.")
	}
	if err != nil {
		s.record(ctx, actor, audit.ActionDatasetPublished, version, nil, nil, "", err)
		return domain.Version{}, err
	}
	v, err := s.publish(ctx, version, at, &actor)
	if err != nil {
		s.record(ctx, actor, audit.ActionDatasetPublished, version, map[string]any{"status": ready.Status}, nil, "", err)
	}
	return v, err
}

// AdvanceAs moves an unpublished release through the review gate one legal
// step at a time. Skipping validation or review is intentionally impossible.
func (s *Service) AdvanceAs(ctx context.Context, actor Actor, version string, to domain.Status, reason string) (domain.Version, error) {
	current, err := s.repo.Get(ctx, strings.TrimSpace(version))
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Version{}, apierr.New(apierr.NotFound, "No such dataset version.")
	}
	if err != nil {
		return domain.Version{}, apierr.Wrap(apierr.Internal, "Could not read the version.", err)
	}
	legal := map[domain.Status]domain.Status{
		domain.StatusDraft:      domain.StatusValidation,
		domain.StatusValidation: domain.StatusReview,
		domain.StatusReview:     domain.StatusApproved,
	}
	if legal[current.Status] != to {
		return domain.Version{}, apierr.New(apierr.InvalidArgument, "That release lifecycle transition is not allowed.").WithDetail("from", string(current.Status)).WithDetail("to", string(to))
	}
	if (to == domain.StatusReview || to == domain.StatusApproved) && strings.TrimSpace(current.Changelog) == "" {
		return domain.Version{}, apierr.New(apierr.InvalidArgument, "Release notes are required before review.")
	}
	if strings.TrimSpace(reason) == "" {
		return domain.Version{}, apierr.New(apierr.InvalidArgument, "A lifecycle reason is required.")
	}
	next := *current
	next.Status = to
	e, buildErr := audit.New(audit.Actor{Kind: audit.ActorAdmin, ID: actor.ID, Label: actor.Email, IP: actor.IP}, audit.ActionDatasetLifecycleChanged, audit.Target{Kind: "dataset", ID: version, Label: version})
	if buildErr != nil {
		return domain.Version{}, apierr.Wrap(apierr.Internal, "Could not prepare audit evidence.", buildErr)
	}
	e = e.WithRequest(actor.RequestID).WithReason(reason).WithChange(map[string]any{"status": string(current.Status)}, map[string]any{"status": string(to)})
	if err := s.repo.UpdateAudited(ctx, *current, next, e); err != nil {
		return domain.Version{}, apierr.Wrap(apierr.Conflict, "The release changed while it was being updated. Refresh and try again.", err)
	}
	return next, nil
}

// UpdateChangelogAs stores public release notes before approval and pairs the
// edit with immutable audit evidence in the same transaction.
func (s *Service) UpdateChangelogAs(ctx context.Context, actor Actor, version, changelog, reason string) (domain.Version, error) {
	current, err := s.repo.Get(ctx, strings.TrimSpace(version))
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Version{}, apierr.New(apierr.NotFound, "No such dataset version.")
	}
	if err != nil {
		return domain.Version{}, apierr.Wrap(apierr.Internal, "Could not read the version.", err)
	}
	if current.Status == domain.StatusPublished || current.Status == domain.StatusRolledBack {
		return domain.Version{}, apierr.New(apierr.InvalidArgument, "Published release notes are immutable.")
	}
	changelog = strings.TrimSpace(changelog)
	if changelog == "" {
		return domain.Version{}, apierr.New(apierr.InvalidArgument, "Release notes are required.")
	}
	if len(changelog) > 20_000 {
		return domain.Version{}, apierr.New(apierr.InvalidArgument, "Release notes are too long.")
	}
	if strings.TrimSpace(reason) == "" {
		return domain.Version{}, apierr.New(apierr.InvalidArgument, "A changelog reason is required.")
	}
	next := *current
	next.Changelog = changelog
	e, buildErr := audit.New(audit.Actor{Kind: audit.ActorAdmin, ID: actor.ID, Label: actor.Email, IP: actor.IP}, audit.ActionDatasetChangelogUpdated, audit.Target{Kind: "dataset", ID: version, Label: version})
	if buildErr != nil {
		return domain.Version{}, apierr.Wrap(apierr.Internal, "Could not prepare audit evidence.", buildErr)
	}
	e = e.WithRequest(actor.RequestID).WithReason(reason).WithChange(map[string]any{"changelog": current.Changelog}, map[string]any{"changelog": changelog})
	if err := s.repo.UpdateAudited(ctx, *current, next, e); err != nil {
		return domain.Version{}, apierr.Wrap(apierr.Conflict, "The release changed while it was being updated. Refresh and try again.", err)
	}
	return next, nil
}

// Rollback restores a previously published version as the live one.
//
// The version being rolled back FROM is marked rolled_back rather than
// deleted, so the history of what was live and when survives — that record is
// the whole point of versioning a public dataset.
func (s *Service) Rollback(ctx context.Context, to, at string) (from, restored domain.Version, err error) {
	return s.rollback(ctx, to, at, nil, "")
}

func (s *Service) rollback(ctx context.Context, to, at string, actor *Actor, reason string) (from, restored domain.Version, err error) {
	target, err := s.repo.Get(ctx, to)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Version{}, domain.Version{}, apierr.New(apierr.NotFound,
			"No such dataset version.").WithDetail("version", to)
	}
	if err != nil {
		return domain.Version{}, domain.Version{}, apierr.Wrap(apierr.Internal, "Could not read the version.", err)
	}
	if target.Status != domain.StatusRolledBack {
		return domain.Version{}, domain.Version{}, apierr.New(apierr.InvalidArgument, "Only a previously published, rolled-back version can be restored.").WithDetail("status", string(target.Status))
	}
	if len(target.Artifacts) == 0 {
		return domain.Version{}, domain.Version{}, apierr.New(apierr.InvalidArgument,
			"That version has no artifacts to roll back to.")
	}

	current, err := s.repo.ListPublished(ctx)
	if err != nil {
		return domain.Version{}, domain.Version{}, apierr.Wrap(apierr.Internal, "Could not read the catalogue.", err)
	}
	for _, c := range current {
		if c.Version == to {
			return domain.Version{}, domain.Version{}, apierr.New(apierr.InvalidArgument,
				"That version is already the published one.")
		}
	}

	// Demote first. If the second write fails the catalogue is briefly empty,
	// which a consumer can detect; promoting first could leave TWO published
	// versions, which they cannot.
	var previous domain.Version
	demoted := make([]domain.Version, 0, len(current))
	for _, c := range current {
		c.Status = domain.StatusRolledBack
		demoted = append(demoted, c)
		previous = c
	}

	target.Status = domain.StatusPublished
	target.PublishedAt = at
	var activateErr error
	if actor == nil {
		activateErr = s.repo.Activate(ctx, demoted, *target)
	} else {
		e, buildErr := audit.New(audit.Actor{Kind: audit.ActorAdmin, ID: actor.ID, Label: actor.Email, IP: actor.IP}, audit.ActionDatasetRolledBk, audit.Target{Kind: "dataset", ID: to, Label: to})
		if buildErr != nil {
			return domain.Version{}, domain.Version{}, apierr.Wrap(apierr.Internal, "Could not prepare audit evidence.", buildErr)
		}
		e = e.WithRequest(actor.RequestID).WithReason(reason).WithChange(map[string]any{"published": previous.Version}, map[string]any{"published": target.Version})
		activateErr = s.repo.ActivateAudited(ctx, demoted, *target, e)
	}
	if activateErr != nil {
		return domain.Version{}, domain.Version{}, apierr.Wrap(apierr.Internal, "Could not roll back.", activateErr)
	}
	return previous, *target, nil
}

func (s *Service) RollbackAs(ctx context.Context, actor Actor, to, confirmation, reason, at string) (from, restored domain.Version, err error) {
	if strings.TrimSpace(confirmation) != to {
		err = apierr.New(apierr.InvalidArgument, "Confirmation must exactly match the target version.")
		s.record(ctx, actor, audit.ActionDatasetRolledBk, to, nil, nil, reason, err)
		return
	}
	if strings.TrimSpace(reason) == "" {
		err = apierr.New(apierr.InvalidArgument, "A rollback reason is required.")
		s.record(ctx, actor, audit.ActionDatasetRolledBk, to, nil, nil, reason, err)
		return
	}
	from, restored, err = s.rollback(ctx, to, at, &actor, reason)
	if err != nil {
		s.record(ctx, actor, audit.ActionDatasetRolledBk, to, map[string]any{"published": from.Version}, nil, reason, err)
	}
	return
}

// History lists every version regardless of status, newest first.
func (s *Service) History(ctx context.Context) ([]domain.Version, error) {
	vs, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, apierr.Wrap(apierr.Internal, "Could not read the catalogue.", err)
	}
	return vs, nil
}
