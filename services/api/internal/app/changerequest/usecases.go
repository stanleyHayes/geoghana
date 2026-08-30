// Package changerequest implements permission-scoped moderation commands.
package changerequest

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/account"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/audit"
	domain "github.com/ghanageo/ghanageo/services/api/internal/domain/changerequest"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
)

type Page struct {
	Data       []domain.ChangeRequest
	NextCursor string
}
type Filter struct {
	Cursor                                        string
	Limit                                         int
	State                                         domain.State
	TargetKind, TargetID, SubmitterID, ReviewerID string
}

type Repository interface {
	Create(context.Context, domain.ChangeRequest, string, audit.Entry) (domain.ChangeRequest, bool, error)
	Get(context.Context, string) (domain.ChangeRequest, error)
	List(context.Context, Filter) (Page, error)
	Transition(context.Context, domain.ChangeRequest, int64, audit.Entry) error
	Revise(context.Context, domain.ChangeRequest, int64, audit.Entry) error
	AddComment(context.Context, domain.ChangeRequest, int64, audit.Entry) error
}

type Actor struct {
	ID, Email, IP, RequestID string
	Role                     account.Role
}
type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) *Service                  { return &Service{repo: repo, now: time.Now} }
func (s *Service) WithClock(now func() time.Time) *Service { s.now = now; return s }

type SubmitCommand struct {
	Target        domain.Target
	ProposedPatch map[string]any
	Before, After map[string]any
	Evidence      domain.Evidence
}

func authorize(a Actor, p account.Permission) error {
	if strings.TrimSpace(a.ID) == "" {
		return apierr.New(apierr.Unauthenticated, "Authentication is required.")
	}
	if !account.Can(a.Role, p) {
		return apierr.New(apierr.PermissionDenied, "Your role cannot perform this action.").WithDetail("requiredPermission", string(p))
	}
	return nil
}

func auditEntry(a Actor, action audit.Action, cr domain.ChangeRequest, before, after map[string]any, reason string) (audit.Entry, error) {
	e, err := audit.New(audit.Actor{Kind: audit.ActorAdmin, ID: a.ID, Label: a.Email, IP: a.IP}, action,
		audit.Target{Kind: "change_request", ID: cr.ID, Label: cr.Target.Kind + ":" + cr.Target.ID})
	if err != nil {
		return audit.Entry{}, err
	}
	return e.WithRequest(a.RequestID).WithChange(before, after).WithReason(reason), nil
}

func (s *Service) Submit(ctx context.Context, a Actor, cmd SubmitCommand) (domain.ChangeRequest, error) {
	if err := authorize(a, account.PermProposeChange); err != nil {
		return domain.ChangeRequest{}, err
	}
	cr, err := domain.New(cmd.Target, cmd.ProposedPatch, cmd.Before, cmd.After, cmd.Evidence, a.ID, s.now())
	if err != nil {
		return domain.ChangeRequest{}, apierr.Wrap(apierr.InvalidArgument, "The change request is invalid.", err)
	}
	cr.History[0].RequestID = strings.TrimSpace(a.RequestID)
	e, err := auditEntry(a, audit.ActionChangeRequestSubmitted, cr, nil, map[string]any{"state": string(cr.State), "snapshotDigest": cr.Snapshot.Digest}, "")
	if err != nil {
		return domain.ChangeRequest{}, apierr.Wrap(apierr.Internal, "Could not prepare audit evidence.", err)
	}
	stored, _, err := s.repo.Create(ctx, cr, a.RequestID, e)
	if err != nil {
		return domain.ChangeRequest{}, apierr.Wrap(apierr.Internal, "Could not submit the change request.", err)
	}
	return stored, nil
}

func (s *Service) Get(ctx context.Context, a Actor, id string) (domain.ChangeRequest, error) {
	if err := authorize(a, account.PermProposeChange); err != nil {
		return domain.ChangeRequest{}, err
	}
	cr, err := s.repo.Get(ctx, strings.TrimSpace(id))
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ChangeRequest{}, apierr.New(apierr.NotFound, "No such change request.")
	}
	if err != nil {
		return domain.ChangeRequest{}, apierr.Wrap(apierr.Internal, "Could not read the change request.", err)
	}
	if !account.Can(a.Role, account.PermReviewChange) && cr.SubmitterID != a.ID {
		return domain.ChangeRequest{}, apierr.New(apierr.PermissionDenied, "You can only view your own change requests.")
	}
	return cr, nil
}

func (s *Service) List(ctx context.Context, a Actor, f Filter) (Page, error) {
	if err := authorize(a, account.PermProposeChange); err != nil {
		return Page{}, err
	}
	if f.Limit < 0 || f.Limit > 100 {
		return Page{}, apierr.New(apierr.InvalidArgument, "Limit must be between 1 and 100.")
	}
	if f.State != "" && !f.State.Valid() {
		return Page{}, apierr.New(apierr.InvalidArgument, "State is invalid.")
	}
	if !account.Can(a.Role, account.PermReviewChange) {
		f.SubmitterID = a.ID
		f.ReviewerID = ""
	}
	page, err := s.repo.List(ctx, f)
	if err != nil {
		return Page{}, apierr.Wrap(apierr.InvalidArgument, "Could not list change requests.", err)
	}
	return page, nil
}

func transitionAction(to domain.State) audit.Action {
	switch to {
	case domain.StateInReview:
		return audit.ActionChangeRequestReviewStarted
	case domain.StateChangesRequested:
		return audit.ActionChangeRequestChangesRequested
	case domain.StateApproved:
		return audit.ActionChangeRequestApproved
	case domain.StateRejected:
		return audit.ActionChangeRequestRejected
	default:
		return audit.ActionChangeRequestSubmitted
	}
}

func (s *Service) Transition(ctx context.Context, a Actor, id string, expectedVersion int64, to domain.State, comment string) (domain.ChangeRequest, error) {
	if err := authorize(a, account.PermReviewChange); err != nil {
		return domain.ChangeRequest{}, err
	}
	current, err := s.repo.Get(ctx, strings.TrimSpace(id))
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ChangeRequest{}, apierr.New(apierr.NotFound, "No such change request.")
	}
	if err != nil {
		return domain.ChangeRequest{}, apierr.Wrap(apierr.Internal, "Could not read the change request.", err)
	}
	if expectedVersion != current.Version {
		if transitionReplay(current, a, to) {
			return current, nil
		}
		return domain.ChangeRequest{}, apierr.New(apierr.Conflict, "The change request changed. Refresh and try again.").WithDetail("currentVersion", current.Version)
	}
	next, err := current.Transition(to, a.ID, comment, a.RequestID, s.now())
	if err != nil {
		code := apierr.InvalidArgument
		if errors.Is(err, domain.ErrConflict) {
			code = apierr.Conflict
		}
		return domain.ChangeRequest{}, apierr.Wrap(code, "That moderation transition is not allowed.", err)
	}
	e, err := auditEntry(a, transitionAction(to), next, map[string]any{"state": string(current.State), "version": current.Version}, map[string]any{"state": string(next.State), "version": next.Version}, comment)
	if err != nil {
		return domain.ChangeRequest{}, apierr.Wrap(apierr.Internal, "Could not prepare audit evidence.", err)
	}
	if err := s.repo.Transition(ctx, next, expectedVersion, e); errors.Is(err, domain.ErrConflict) {
		if stored, getErr := s.repo.Get(ctx, current.ID); getErr == nil && transitionReplay(stored, a, to) {
			return stored, nil
		}
		return domain.ChangeRequest{}, apierr.New(apierr.Conflict, "The change request changed. Refresh and try again.")
	} else if err != nil {
		return domain.ChangeRequest{}, apierr.Wrap(apierr.Internal, "Could not update the change request.", err)
	}
	return next, nil
}

type ReviseCommand struct {
	ProposedPatch map[string]any
	After         map[string]any
	Evidence      domain.Evidence
	Comment       string
}

func (s *Service) Revise(ctx context.Context, a Actor, id string, expectedVersion int64, cmd ReviseCommand) (domain.ChangeRequest, error) {
	if err := authorize(a, account.PermProposeChange); err != nil {
		return domain.ChangeRequest{}, err
	}
	current, err := s.repo.Get(ctx, strings.TrimSpace(id))
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ChangeRequest{}, apierr.New(apierr.NotFound, "No such change request.")
	}
	if err != nil {
		return domain.ChangeRequest{}, apierr.Wrap(apierr.Internal, "Could not read the change request.", err)
	}
	if expectedVersion != current.Version {
		if transitionReplay(current, a, domain.StateSubmitted) {
			return current, nil
		}
		return domain.ChangeRequest{}, apierr.New(apierr.Conflict, "The change request changed. Refresh and try again.")
	}
	next, err := current.Revise(a.ID, cmd.ProposedPatch, cmd.After, cmd.Evidence, cmd.Comment, a.RequestID, s.now())
	if err != nil {
		return domain.ChangeRequest{}, apierr.Wrap(apierr.InvalidArgument, "The revised proposal is invalid.", err)
	}
	e, err := auditEntry(a, audit.ActionChangeRequestSubmitted, next, map[string]any{"state": string(current.State), "version": current.Version, "snapshotDigest": current.Snapshot.Digest}, map[string]any{"state": string(next.State), "version": next.Version, "snapshotDigest": next.Snapshot.Digest}, cmd.Comment)
	if err != nil {
		return domain.ChangeRequest{}, apierr.Wrap(apierr.Internal, "Could not prepare audit evidence.", err)
	}
	if err := s.repo.Revise(ctx, next, expectedVersion, e); errors.Is(err, domain.ErrConflict) {
		if stored, getErr := s.repo.Get(ctx, current.ID); getErr == nil && transitionReplay(stored, a, domain.StateSubmitted) {
			return stored, nil
		}
		return domain.ChangeRequest{}, apierr.New(apierr.Conflict, "The change request changed. Refresh and try again.")
	} else if err != nil {
		return domain.ChangeRequest{}, apierr.Wrap(apierr.Internal, "Could not revise the change request.", err)
	}
	return next, nil
}

func (s *Service) AddReviewerComment(ctx context.Context, a Actor, id string, expectedVersion int64, body string) (domain.ChangeRequest, error) {
	if err := authorize(a, account.PermReviewChange); err != nil {
		return domain.ChangeRequest{}, err
	}
	current, err := s.repo.Get(ctx, strings.TrimSpace(id))
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ChangeRequest{}, apierr.New(apierr.NotFound, "No such change request.")
	}
	if err != nil {
		return domain.ChangeRequest{}, apierr.Wrap(apierr.Internal, "Could not read the change request.", err)
	}
	if current.State == domain.StateApproved || current.State == domain.StateRejected {
		return domain.ChangeRequest{}, apierr.New(apierr.InvalidArgument, "A terminal change request cannot be commented on.")
	}
	if expectedVersion != current.Version {
		if commentReplay(current, a) {
			return current, nil
		}
		return domain.ChangeRequest{}, apierr.New(apierr.Conflict, "The change request changed. Refresh and try again.")
	}
	next, err := current.AddComment(a.ID, body, a.RequestID, s.now())
	if err != nil {
		return domain.ChangeRequest{}, apierr.Wrap(apierr.InvalidArgument, "The reviewer comment is invalid.", err)
	}
	e, err := auditEntry(a, audit.ActionChangeRequestCommented, next, map[string]any{"version": current.Version}, map[string]any{"version": next.Version, "commentId": next.Comments[len(next.Comments)-1].ID}, "")
	if err != nil {
		return domain.ChangeRequest{}, apierr.Wrap(apierr.Internal, "Could not prepare audit evidence.", err)
	}
	if err := s.repo.AddComment(ctx, next, expectedVersion, e); errors.Is(err, domain.ErrConflict) {
		if stored, getErr := s.repo.Get(ctx, current.ID); getErr == nil && commentReplay(stored, a) {
			return stored, nil
		}
		return domain.ChangeRequest{}, apierr.New(apierr.Conflict, "The change request changed. Refresh and try again.")
	} else if err != nil {
		return domain.ChangeRequest{}, apierr.Wrap(apierr.Internal, "Could not add the reviewer comment.", err)
	}
	return next, nil
}

func transitionReplay(cr domain.ChangeRequest, a Actor, to domain.State) bool {
	if strings.TrimSpace(a.RequestID) == "" {
		return false
	}
	for _, event := range cr.History {
		if event.RequestID == a.RequestID && event.ActorID == a.ID && event.To == to {
			return true
		}
	}
	return false
}

func commentReplay(cr domain.ChangeRequest, a Actor) bool {
	if strings.TrimSpace(a.RequestID) == "" {
		return false
	}
	for _, comment := range cr.Comments {
		if comment.RequestID == a.RequestID && comment.AuthorID == a.ID {
			return true
		}
	}
	return false
}
