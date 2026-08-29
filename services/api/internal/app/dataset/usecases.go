// Package dataset serves the public dataset catalogue and its downloads
// (Spec §7.2, story GEO-8.3).
package dataset

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	domain "github.com/ghanageo/ghanageo/services/api/internal/domain/dataset"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
)

// Repository is the persistence port for dataset versions.
type Repository interface {
	ListPublished(ctx context.Context) ([]domain.Version, error)
	Get(ctx context.Context, version string) (*domain.Version, error)
	Upsert(ctx context.Context, v domain.Version) error
}

// Service reads the dataset catalogue and opens artifact files.
type Service struct {
	repo Repository
	// exportDir is the root that every artifact resolves under. Nothing
	// outside it is ever opened.
	exportDir string
}

func NewService(repo Repository, exportDir string) *Service {
	return &Service{repo: repo, exportDir: exportDir}
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
	full := filepath.Join(root, a.Filename)
	// Belt and braces: even with a safe filename, confirm the resolved path
	// did not escape. A symlink inside the export directory could otherwise.
	resolved, err := filepath.Abs(full)
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
