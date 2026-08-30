package account

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	mongoadapter "github.com/ghanageo/ghanageo/services/api/internal/adapters/mongo"
	domain "github.com/ghanageo/ghanageo/services/api/internal/domain/account"
)

func authenticationTestService(t *testing.T) (*Service, *mongoadapter.AccountRepo, *mongoadapter.SessionRepo) {
	return authenticationTestServiceWithMailer(t, nil)
}

func authenticationTestServiceWithMailer(t *testing.T, mail Mailer) (*Service, *mongoadapter.AccountRepo, *mongoadapter.SessionRepo) {
	t.Helper()
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://127.0.0.1:27117/?directConnection=true"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	store, err := mongoadapter.Connect(ctx, uri, fmt.Sprintf("ghanageo_auth_test_%d", time.Now().UnixNano()))
	if err != nil {
		if os.Getenv("GHANAGEO_REQUIRE_MONGO_TRANSACTIONS") == "true" {
			t.Fatalf("required Mongo integration test unavailable: %v", err)
		}
		t.Skipf("Mongo integration test unavailable: %v", err)
	}
	var hello bson.M
	if err := store.DB().RunCommand(ctx, bson.D{{Key: "hello", Value: 1}}).Decode(&hello); err != nil || hello["setName"] == nil {
		_ = store.Close(context.Background())
		if os.Getenv("GHANAGEO_REQUIRE_MONGO_TRANSACTIONS") == "true" {
			t.Fatalf("Mongo transaction capability is required (hello error=%v, setName=%v)", err, hello["setName"])
		}
		t.Skip("Mongo transactions require a replica set")
	}
	t.Cleanup(func() {
		_ = store.DB().Drop(context.Background())
		_ = store.Close(context.Background())
	})
	accounts := mongoadapter.NewAccountRepo(store)
	sessions := mongoadapter.NewSessionRepo(store)
	service := NewService(accounts, sessions, mongoadapter.NewOneTimeTokenRepo(store), nil, mail,
		slog.New(slog.NewTextHandler(io.Discard, nil)), "GhanaGeo")
	return service, accounts, sessions
}

type failOnceResetMailer struct {
	mu       sync.Mutex
	attempts int
}

func (*failOnceResetMailer) SendVerification(context.Context, string, string) error { return nil }
func (m *failOnceResetMailer) SendPasswordReset(_ context.Context, _, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.attempts++
	if m.attempts == 1 {
		return fmt.Errorf("provider unavailable")
	}
	return nil
}

func TestPasswordResetDeliveryFailureAllowsImmediateRetry(t *testing.T) {
	mailer := &failOnceResetMailer{}
	service, accounts, _ := authenticationTestServiceWithMailer(t, mailer)
	ctx := context.Background()
	a := domain.Account{ID: "acc_reset_retry", Email: "retry@example.com", EmailVerified: true, Role: domain.RoleDeveloper}
	if err := accounts.Create(ctx, a); err != nil {
		t.Fatal(err)
	}

	first := service.RequestPasswordReset(ctx, a.Email, "127.0.0.1")
	second := service.RequestPasswordReset(ctx, a.Email, "127.0.0.1")
	if first.Message != second.Message {
		t.Fatalf("enumeration-safe response changed across provider failure: %q != %q", first.Message, second.Message)
	}
	mailer.mu.Lock()
	attempts := mailer.attempts
	mailer.mu.Unlock()
	if attempts != 2 {
		t.Fatalf("provider attempts = %d, want immediate retry to reach provider", attempts)
	}
}

func TestAuthenticateRotatesOnlyWhenDueAndHasOneConcurrentWinner(t *testing.T) {
	service, accounts, sessions := authenticationTestService(t)
	ctx := context.Background()
	a := domain.Account{ID: "acc_rotation", Email: "rotation@example.com", EmailVerified: true, Role: domain.RoleDeveloper}
	if err := accounts.Create(ctx, a); err != nil {
		t.Fatal(err)
	}
	a.SessionEpoch = 1

	fresh, err := domain.Issue(a, domain.StageAuthenticated, time.Now().UTC(), "test", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if err := sessions.Create(ctx, fresh.Session); err != nil {
		t.Fatal(err)
	}
	got, _, replacement, err := service.Authenticate(ctx, fresh.Token)
	if err != nil {
		t.Fatal(err)
	}
	if replacement != "" || got.ID != fresh.Session.ID {
		t.Fatalf("fresh session rotated: replacement=%q id=%q", replacement, got.ID)
	}

	due, err := domain.Issue(a, domain.StageAuthenticated, time.Now().UTC().Add(-domain.RotationInterval-time.Minute), "test", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if err := sessions.Create(ctx, due.Session); err != nil {
		t.Fatal(err)
	}
	const attempts = 16
	var winners atomic.Int32
	var wg sync.WaitGroup
	errs := make(chan error, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, token, err := service.Authenticate(ctx, due.Token)
			if err != nil {
				errs <- err
				return
			}
			if token != "" {
				winners.Add(1)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if got := winners.Load(); got != 1 {
		t.Fatalf("Set-Cookie winners = %d, want exactly 1", got)
	}
}
