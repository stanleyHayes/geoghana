package mongo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/account"
)

func sessionTestStore(t *testing.T) *Store {
	t.Helper()
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://127.0.0.1:27117/?directConnection=true"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	store, err := Connect(ctx, uri, fmt.Sprintf("ghanageo_session_test_%d", time.Now().UnixNano()))
	if err != nil {
		if os.Getenv("GHANAGEO_REQUIRE_MONGO_TRANSACTIONS") == "true" {
			t.Fatalf("required Mongo integration test unavailable: %v", err)
		}
		t.Skipf("Mongo integration test unavailable: %v", err)
	}
	var hello bson.M
	if err := store.db.RunCommand(ctx, bson.D{{Key: "hello", Value: 1}}).Decode(&hello); err != nil || hello["setName"] == nil {
		_ = store.Close(context.Background())
		if os.Getenv("GHANAGEO_REQUIRE_MONGO_TRANSACTIONS") == "true" {
			t.Fatalf("Mongo transaction capability is required (hello error=%v, setName=%v)", err, hello["setName"])
		}
		t.Skip("Mongo transactions require a replica set")
	}
	t.Cleanup(func() {
		_ = store.db.Drop(context.Background())
		_ = store.Close(context.Background())
	})
	return store
}

func TestCreatePasswordResetSerializesConcurrentIssuance(t *testing.T) {
	store := sessionTestStore(t)
	repo := NewOneTimeTokenRepo(store)
	now := time.Now().UTC()
	const attempts = 24
	var winners atomic.Int32
	var wg sync.WaitGroup
	errs := make(chan error, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			issued, err := account.IssueToken("acc_concurrent", account.PurposePasswordReset, now)
			if err != nil {
				errs <- err
				return
			}
			created, err := repo.CreatePasswordReset(context.Background(), issued.Token, 15*time.Minute)
			if err != nil {
				errs <- err
				return
			}
			if created {
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
		t.Fatalf("issuance winners = %d, want exactly 1", got)
	}
}

func TestReleasePasswordResetCannotClearNewerWinner(t *testing.T) {
	store := sessionTestStore(t)
	repo := NewOneTimeTokenRepo(store)
	ctx := context.Background()
	now := time.Now().UTC()
	old, err := account.IssueToken("acc_release", account.PurposePasswordReset, now.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	created, err := repo.CreatePasswordReset(ctx, old.Token, 15*time.Minute)
	if err != nil || !created {
		t.Fatalf("create old token: created=%v err=%v", created, err)
	}
	newer, err := account.IssueToken("acc_release", account.PurposePasswordReset, now)
	if err != nil {
		t.Fatal(err)
	}
	created, err = repo.CreatePasswordReset(ctx, newer.Token, 15*time.Minute)
	if err != nil || !created {
		t.Fatalf("replace old token: created=%v err=%v", created, err)
	}
	// Late failure callbacks for the old delivery race with new requests. None
	// may clear the newer winner or reopen its cooldown.
	const racers = 24
	var wg sync.WaitGroup
	errs := make(chan error, racers*2)
	var unexpectedWinners atomic.Int32
	for i := 0; i < racers; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			if err := repo.ReleasePasswordReset(ctx, old.Token.AccountID, old.Token.Hash); err != nil {
				errs <- err
			}
		}()
		go func() {
			defer wg.Done()
			candidate, issueErr := account.IssueToken("acc_release", account.PurposePasswordReset, now)
			if issueErr != nil {
				errs <- issueErr
				return
			}
			won, createErr := repo.CreatePasswordReset(ctx, candidate.Token, 15*time.Minute)
			if createErr != nil {
				errs <- createErr
				return
			}
			if won {
				unexpectedWinners.Add(1)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if got := unexpectedWinners.Load(); got != 0 {
		t.Fatalf("stale releases reopened cooldown for %d concurrent issuers", got)
	}
	stored, err := repo.ByValue(ctx, newer.Value)
	if err != nil {
		t.Fatalf("late old-token release deleted newer winner: %v", err)
	}
	if stored.Hash != newer.Token.Hash {
		t.Fatal("unexpected reset token remains after conditional release")
	}
}

func TestSessionRotateIsSingleWinnerAndRollsBackOnInsertFailure(t *testing.T) {
	store := sessionTestStore(t)
	repo := NewSessionRepo(store)
	now := time.Now().UTC()
	base := account.Session{ID: "sess_base", AccountID: "acc_1", Role: account.RoleDeveloper,
		Stage: account.StageAuthenticated, TokenHash: account.HashToken("base"), Epoch: 1,
		IssuedAt: now.Add(-time.Hour), LastUsedAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour)}
	if err := repo.Create(context.Background(), base); err != nil {
		t.Fatal(err)
	}

	// A failed replacement insert must roll the supersede update back.
	bad := base
	bad.TokenHash = account.HashToken("replacement")
	if err := repo.Rotate(context.Background(), base.ID, bad); err == nil {
		t.Fatal("duplicate replacement unexpectedly succeeded")
	}
	var afterFailure sessionDoc
	if err := repo.col().FindOne(context.Background(), bson.M{"_id": base.ID}).Decode(&afterFailure); err != nil {
		t.Fatal(err)
	}
	if afterFailure.SupersededAt != nil {
		t.Fatal("failed rotation burned the source session")
	}

	const attempts = 16
	var winners atomic.Int32
	var wg sync.WaitGroup
	errs := make(chan error, attempts)
	for i := 0; i < attempts; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			next := base
			next.ID = fmt.Sprintf("sess_next_%02d", i)
			next.TokenHash = account.HashToken(next.ID)
			next.RotatedFrom = base.ID
			err := repo.Rotate(context.Background(), base.ID, next)
			switch {
			case err == nil:
				winners.Add(1)
			case errors.Is(err, ErrSessionRotationLost):
			default:
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if got := winners.Load(); got != 1 {
		t.Fatalf("rotation winners = %d, want exactly 1", got)
	}
	count, err := repo.col().CountDocuments(context.Background(), bson.M{"rotatedFrom": base.ID})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("replacement branches = %d, want 1", count)
	}
}

func TestConsumePasswordResetHasOneWinnerAndRevokesAllSessions(t *testing.T) {
	store := sessionTestStore(t)
	ctx := context.Background()
	accounts := NewAccountRepo(store)
	sessions := NewSessionRepo(store)
	tokens := NewOneTimeTokenRepo(store)

	oldPassword := "old password for reset testing"
	oldHash, err := account.HashPassword(oldPassword)
	if err != nil {
		t.Fatal(err)
	}
	a := account.Account{ID: "acc_reset_race", Email: "reset-race@example.com", EmailVerified: true,
		PasswordHash: oldHash, Role: account.RoleDeveloper}
	if err := accounts.Create(ctx, a); err != nil {
		t.Fatal(err)
	}
	a.SessionEpoch = 1
	const priorSessions = 4
	for i := 0; i < priorSessions; i++ {
		issued, err := account.Issue(a, account.StageAuthenticated, time.Now().UTC(), fmt.Sprintf("agent-%d", i), "127.0.0.1")
		if err != nil {
			t.Fatal(err)
		}
		if err := sessions.Create(ctx, issued.Session); err != nil {
			t.Fatal(err)
		}
	}

	issuedReset, err := account.IssueToken(a.ID, account.PurposePasswordReset, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	created, err := tokens.CreatePasswordReset(ctx, issuedReset.Token, 15*time.Minute)
	if err != nil || !created {
		t.Fatalf("create reset token: created=%v err=%v", created, err)
	}
	storedReset, err := tokens.ByValue(ctx, issuedReset.Value)
	if err != nil {
		t.Fatal(err)
	}

	const attempts = 16
	passwords := make([]string, attempts)
	hashes := make([]string, attempts)
	for i := range passwords {
		passwords[i] = fmt.Sprintf("winner candidate password number %02d", i)
		hashes[i], err = account.HashPassword(passwords[i])
		if err != nil {
			t.Fatal(err)
		}
	}
	type result struct {
		index int
		err   error
	}
	results := make(chan result, attempts)
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- result{index: i, err: tokens.ConsumePasswordReset(ctx, storedReset.ID, a.ID, hashes[i])}
		}()
	}
	wg.Wait()
	close(results)
	winner := -1
	for result := range results {
		if result.err == nil {
			if winner != -1 {
				t.Fatalf("multiple reset winners: %d and %d", winner, result.index)
			}
			winner = result.index
			continue
		}
		if !errors.Is(result.err, account.ErrTokenAlreadyUsed) {
			t.Errorf("loser %d returned unexpected error: %v", result.index, result.err)
		}
	}
	if winner == -1 {
		t.Fatal("password reset race had no winner")
	}

	finalAccount, err := accounts.ByID(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if finalAccount.SessionEpoch != 2 {
		t.Fatalf("session epoch = %d, want exactly one bump to 2", finalAccount.SessionEpoch)
	}
	if err := account.VerifyPassword(passwords[winner], finalAccount.PasswordHash); err != nil {
		t.Fatalf("winning password was not persisted: %v", err)
	}
	if err := account.VerifyPassword(oldPassword, finalAccount.PasswordHash); err == nil {
		t.Fatal("old password remained valid after reset")
	}
	for i, password := range passwords {
		if i == winner {
			continue
		}
		if err := account.VerifyPassword(password, finalAccount.PasswordHash); err == nil {
			t.Fatalf("losing password %d altered final credential", i)
		}
	}
	active, err := sessions.ListActive(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 0 {
		t.Fatalf("active prior sessions = %d, want 0", len(active))
	}
	revoked, err := sessions.col().CountDocuments(ctx, bson.M{"accountId": a.ID, "revokedAt": bson.M{"$exists": true}})
	if err != nil {
		t.Fatal(err)
	}
	if revoked != priorSessions {
		t.Fatalf("revoked session rows = %d, want %d", revoked, priorSessions)
	}
	consumed, err := tokens.ByValue(ctx, issuedReset.Value)
	if err != nil {
		t.Fatal(err)
	}
	if consumed.UsedAt == nil {
		t.Fatal("winning reset did not persist token consumption")
	}
}
