package account

import (
	"context"
	"errors"
	"testing"

	domainaccount "github.com/ghanageo/ghanageo/services/api/internal/domain/account"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/securityalert"
)

type alertRecorder struct{ events []securityalert.Event }

func (r *alertRecorder) Report(_ context.Context, event securityalert.Event) error {
	r.events = append(r.events, event)
	return nil
}

func TestFailedAdminAuthenticationRaisesAlert(t *testing.T) {
	recorder := &alertRecorder{}
	svc := (&Service{}).WithSecurityAlerts(recorder)
	svc.recordSecurityEvent(context.Background(), domainaccount.Account{
		ID: "acc_admin", Email: "admin@example.test", Role: domainaccount.RoleDataAdmin,
	}, "auth.mfa_failed", "203.0.113.10", errors.New("invalid code"))

	if len(recorder.events) != 1 || recorder.events[0].Kind != securityalert.AdminAuthFailed {
		t.Fatalf("events = %+v", recorder.events)
	}
	if recorder.events[0].ActorID != "acc_admin" {
		t.Fatalf("actor = %q", recorder.events[0].ActorID)
	}
}

func TestFailedDeveloperAuthenticationDoesNotRaiseAdminAlert(t *testing.T) {
	recorder := &alertRecorder{}
	svc := (&Service{}).WithSecurityAlerts(recorder)
	svc.recordSecurityEvent(context.Background(), domainaccount.Account{
		ID: "acc_dev", Role: domainaccount.RoleDeveloper,
	}, "auth.login_failed", "203.0.113.11", errors.New("wrong password"))
	if len(recorder.events) != 0 {
		t.Fatalf("developer failure raised admin alert: %+v", recorder.events)
	}
}
