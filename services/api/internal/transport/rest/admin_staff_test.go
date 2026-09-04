package rest

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	appaccount "github.com/ghanageo/ghanageo/services/api/internal/app/account"
)

// The refusals a staff change can produce must not all collapse into one status.
// "You cannot do this to yourself" and "no such account" lead an operator to
// different next actions, and neither is a permission problem.
func TestWriteStaffErrDistinguishesRefusals(t *testing.T) {
	for _, tc := range []struct {
		name     string
		err      error
		wantCont bool
		wantCode int
	}{
		{"success continues", nil, true, http.StatusOK},
		{"acting on self", appaccount.ErrCannotActOnSelf, false, http.StatusConflict},
		{"unknown role", appaccount.ErrInvalidStaffRole, false, http.StatusBadRequest},
		{"unknown account", appaccount.ErrUnknownStaffTarget, false, http.StatusNotFound},
		{"anything else", errors.New("boom"), false, http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/admin/staff/acc_1/role", nil)

			carryOn := writeStaffErr(recorder, request, tc.err)
			if carryOn != tc.wantCont {
				t.Fatalf("continue = %v, want %v", carryOn, tc.wantCont)
			}
			if tc.err == nil {
				return
			}
			if recorder.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d", recorder.Code, tc.wantCode)
			}
		})
	}
}

// Staff administration is refused outright when the account service is absent,
// rather than panicking, matching how the rest of this transport degrades.
func TestStaffEndpointsWithoutAccountsServiceDoNotPanic(t *testing.T) {
	handler := &Handler{}
	for _, route := range []string{"/admin/staff/acc_1/role", "/admin/staff/acc_1/disabled"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, route, nil)
		// No session, so this refuses at requireSteward before reaching the
		// service. The point is that it answers rather than crashing.
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("%s panicked: %v", route, recovered)
				}
			}()
			handler.Routes().ServeHTTP(recorder, request)
		}()
		if recorder.Code == http.StatusOK {
			t.Fatalf("%s answered 200 with no session", route)
		}
	}
}
