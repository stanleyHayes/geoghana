package dataset

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/audit"
	domain "github.com/ghanageo/ghanageo/services/api/internal/domain/dataset"
)

type releaseRepo struct {
	versions    map[string]domain.Version
	activations int
	audits      int
	auditErr    error
}

func (r *releaseRepo) ListPublished(context.Context) ([]domain.Version, error) {
	var out []domain.Version
	for _, v := range r.versions {
		if v.Status.Published() {
			out = append(out, v)
		}
	}
	return out, nil
}
func (r *releaseRepo) ListAll(context.Context) ([]domain.Version, error) { return nil, nil }
func (r *releaseRepo) Get(_ context.Context, version string) (*domain.Version, error) {
	v, ok := r.versions[version]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &v, nil
}
func (r *releaseRepo) Upsert(_ context.Context, v domain.Version) error {
	r.versions[v.Version] = v
	return nil
}
func (r *releaseRepo) Activate(_ context.Context, previous []domain.Version, next domain.Version) error {
	for _, v := range previous {
		r.versions[v.Version] = v
	}
	r.versions[next.Version] = next
	r.activations++
	return nil
}
func (r *releaseRepo) ActivateAudited(ctx context.Context, previous []domain.Version, next domain.Version, _ audit.Entry) error {
	if r.auditErr != nil {
		return r.auditErr
	}
	r.audits++
	return r.Activate(ctx, previous, next)
}
func (r *releaseRepo) UpdateAudited(_ context.Context, before, after domain.Version, _ audit.Entry) error {
	current, ok := r.versions[before.Version]
	if !ok || current.Status != before.Status || current.Changelog != before.Changelog {
		return errors.New("compare-and-swap conflict")
	}
	if r.auditErr != nil {
		return r.auditErr
	}
	r.versions[after.Version] = after
	r.audits++
	return nil
}

func releasable(version string, status domain.Status) domain.Version {
	sum := sha256.Sum256([]byte("0123456789"))
	return domain.Version{Version: version, Status: status, Artifacts: []domain.Artifact{{Entity: "places", Format: domain.FormatCSV, Filename: "places.csv", SizeBytes: 10, RecordCount: 1, SHA256: hex.EncodeToString(sum[:])}}}
}

func writeArtifact(t *testing.T, root, version string) {
	t.Helper()
	dir := filepath.Join(root, version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "places.csv"), []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReleaseLifecycleAndChangelogAreAuditedAndSequential(t *testing.T) {
	repo := &releaseRepo{versions: map[string]domain.Version{"2026.08.5": {Version: "2026.08.5", Status: domain.StatusDraft}}}
	svc := NewService(repo, t.TempDir())
	actor := Actor{ID: "admin", Email: "admin@example.test", RequestID: "request-1"}
	if _, err := svc.AdvanceAs(context.Background(), actor, "2026.08.5", domain.StatusReview, "skip"); err == nil {
		t.Fatal("draft must not skip validation")
	}
	if _, err := svc.UpdateChangelogAs(context.Background(), actor, "2026.08.5", "Canonical boundary corrections.", "prepare review"); err != nil {
		t.Fatal(err)
	}
	for _, status := range []domain.Status{domain.StatusValidation, domain.StatusReview, domain.StatusApproved} {
		if _, err := svc.AdvanceAs(context.Background(), actor, "2026.08.5", status, "review gate passed"); err != nil {
			t.Fatalf("advance to %s: %v", status, err)
		}
	}
	if got := repo.versions["2026.08.5"]; got.Status != domain.StatusApproved || got.Changelog == "" {
		t.Fatalf("release = %#v", got)
	}
	if repo.audits != 4 {
		t.Fatalf("audit count = %d, want 4", repo.audits)
	}
	if _, err := svc.UpdateChangelogAs(context.Background(), actor, "2026.08.5", "", "clear"); err == nil {
		t.Fatal("empty release notes must fail")
	}
}

func TestOpenArtifactRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	version := releasable("2026.08.6", domain.StatusPublished)
	repo := &releaseRepo{versions: map[string]domain.Version{version.Version: version}}
	dir := filepath.Join(root, version.Version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(outside, []byte("0123456789"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "places.csv")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := NewService(repo, root).OpenArtifact(context.Background(), version.Version, "places", "csv"); err == nil {
		t.Fatal("artifact symlink escape was accepted")
	}
}

func TestReadinessAndPublishRequireExactConfirmation(t *testing.T) {
	repo := &releaseRepo{versions: map[string]domain.Version{"2026.08.4": releasable("2026.08.4", domain.StatusApproved)}}
	root := t.TempDir()
	svc := NewService(repo, root)
	missing, err := svc.Readiness(context.Background(), "2026.08.4")
	if err != nil || missing.Ready {
		t.Fatalf("missing artifact readiness = %#v, %v", missing, err)
	}
	writeArtifact(t, root, "2026.08.4")
	r, err := svc.Readiness(context.Background(), "2026.08.4")
	if err != nil || !r.Ready {
		t.Fatalf("readiness = %#v, %v", r, err)
	}
	actor := Actor{ID: "admin", Email: "admin@example.test"}
	if _, err := svc.PublishAs(context.Background(), actor, "2026.08.4", "wrong", "now"); err == nil {
		t.Fatal("publish accepted wrong confirmation")
	}
	if repo.activations != 0 {
		t.Fatal("refused publish mutated repository")
	}
	v, err := svc.PublishAs(context.Background(), actor, "2026.08.4", "2026.08.4", "now")
	if err != nil || !v.Status.Published() {
		t.Fatalf("publish = %#v, %v", v, err)
	}
	if _, err := svc.PublishAs(context.Background(), actor, "2026.08.4", "2026.08.4", "later"); err != nil {
		t.Fatalf("idempotent publish: %v", err)
	}
	if repo.activations != 1 {
		t.Fatalf("activations = %d, want 1", repo.activations)
	}
	if repo.audits != 1 {
		t.Fatalf("audits = %d, want 1", repo.audits)
	}
}

func TestPublishAuditFailureDoesNotMutateCatalogue(t *testing.T) {
	root := t.TempDir()
	writeArtifact(t, root, "2026.08.4")
	repo := &releaseRepo{versions: map[string]domain.Version{"2026.08.4": releasable("2026.08.4", domain.StatusApproved)}, auditErr: errors.New("audit unavailable")}
	svc := NewService(repo, root)
	if _, err := svc.PublishAs(context.Background(), Actor{ID: "admin"}, "2026.08.4", "2026.08.4", "now"); err == nil {
		t.Fatal("publish succeeded without audit")
	}
	if repo.activations != 0 || repo.versions["2026.08.4"].Status != domain.StatusApproved {
		t.Fatal("audit failure mutated catalogue")
	}
}

func TestRollbackRequiresReasonAndRolledBackTarget(t *testing.T) {
	repo := &releaseRepo{versions: map[string]domain.Version{
		"2026.08.4": releasable("2026.08.4", domain.StatusPublished),
		"2026.08.3": releasable("2026.08.3", domain.StatusRolledBack),
		"2026.08.2": releasable("2026.08.2", domain.StatusApproved),
	}}
	svc := NewService(repo, t.TempDir())
	actor := Actor{ID: "admin", Email: "admin@example.test"}
	if _, _, err := svc.RollbackAs(context.Background(), actor, "2026.08.3", "2026.08.3", "", "now"); err == nil {
		t.Fatal("rollback accepted empty reason")
	}
	if _, _, err := svc.RollbackAs(context.Background(), actor, "2026.08.2", "2026.08.2", "incident", "now"); err == nil {
		t.Fatal("rollback accepted never-published target")
	}
	from, restored, err := svc.RollbackAs(context.Background(), actor, "2026.08.3", "2026.08.3", "incident", "now")
	if err != nil || from.Version != "2026.08.4" || restored.Version != "2026.08.3" {
		t.Fatalf("rollback = %#v %#v %v", from, restored, err)
	}
}
