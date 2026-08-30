package mongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/account"
)

var ErrAccountNotFound = errors.New("account not found")

type accountDoc struct {
	ID            string       `bson:"_id"`
	Email         string       `bson:"email"`
	EmailVerified bool         `bson:"emailVerified"`
	PasswordHash  string       `bson:"passwordHash,omitempty"`
	Role          string       `bson:"role"`
	Disabled      bool         `bson:"disabled"`
	TOTPSecret    string       `bson:"totpSecret,omitempty"`
	TOTPLastStep  int64        `bson:"totpLastStep,omitempty"`
	RecoveryCodes []string     `bson:"recoveryCodes,omitempty"`
	Passkeys      []passkeyDoc `bson:"passkeys,omitempty"`
	SessionEpoch  int          `bson:"sessionEpoch"`
	CreatedAt     time.Time    `bson:"createdAt"`
	UpdatedAt     time.Time    `bson:"updatedAt"`
}

type passkeyDoc struct {
	ID   string `bson:"id"`
	Name string `bson:"name"`
	// The library's full credential record. Storing the whole thing means a
	// library upgrade that starts checking another attribute still has it.
	Credential []byte    `bson:"credential"`
	SignCount  uint32    `bson:"signCount"`
	BackedUp   bool      `bson:"backedUp,omitempty"`
	AddedAt    time.Time `bson:"addedAt"`
	LastUsed   time.Time `bson:"lastUsed,omitempty"`
}

func (d accountDoc) toDomain() account.Account {
	keys := make([]account.Passkey, 0, len(d.Passkeys))
	for _, p := range d.Passkeys {
		keys = append(keys, account.Passkey{
			ID: p.ID, Name: p.Name, Credential: p.Credential,
			SignCount: p.SignCount, BackedUp: p.BackedUp,
			AddedAt: p.AddedAt, LastUsed: p.LastUsed,
		})
	}
	return account.Account{
		ID: d.ID, Email: d.Email, EmailVerified: d.EmailVerified,
		PasswordHash: d.PasswordHash, Role: account.Role(d.Role),
		Disabled: d.Disabled, TOTPSecret: d.TOTPSecret, Passkeys: keys,
		SessionEpoch: d.SessionEpoch, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

// AccountRepo stores accounts.
type AccountRepo struct{ s *Store }

func NewAccountRepo(s *Store) *AccountRepo { return &AccountRepo{s: s} }

func (r *AccountRepo) col() *mongo.Collection { return r.s.db.Collection(ColAccounts) }

func (r *AccountRepo) Create(ctx context.Context, a account.Account) error {
	now := time.Now().UTC()
	d := accountDoc{
		ID: a.ID, Email: account.NormalizeEmail(a.Email), EmailVerified: a.EmailVerified,
		PasswordHash: a.PasswordHash, Role: string(a.Role), Disabled: a.Disabled,
		SessionEpoch: 1, CreatedAt: now, UpdatedAt: now,
	}
	if _, err := r.col().InsertOne(ctx, d); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			// The unique index on email is what makes registration
			// race-safe; two simultaneous sign-ups cannot both win.
			return fmt.Errorf("an account with that email already exists")
		}
		return fmt.Errorf("create account: %w", err)
	}
	return nil
}

func (r *AccountRepo) ByEmail(ctx context.Context, email string) (*account.Account, error) {
	var d accountDoc
	err := r.col().FindOne(ctx, bson.M{"email": account.NormalizeEmail(email)}).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrAccountNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find account: %w", err)
	}
	a := d.toDomain()
	return &a, nil
}

func (r *AccountRepo) ByID(ctx context.Context, id string) (*account.Account, error) {
	var d accountDoc
	err := r.col().FindOne(ctx, bson.M{"_id": id}).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrAccountNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find account: %w", err)
	}
	a := d.toDomain()
	return &a, nil
}

func (r *AccountRepo) MarkEmailVerified(ctx context.Context, id string) error {
	return r.set(ctx, id, bson.M{"emailVerified": true})
}

func (r *AccountRepo) SetPasswordHash(ctx context.Context, id, hash string) error {
	return r.set(ctx, id, bson.M{"passwordHash": hash})
}

// SetTOTP enrols an authenticator and its recovery codes together. They are
// written in one update so an account can never end up with a second factor
// but no way to recover it.
func (r *AccountRepo) SetTOTP(ctx context.Context, id, secret string, recoveryHashes []string) error {
	return r.set(ctx, id, bson.M{
		"totpSecret": secret, "recoveryCodes": recoveryHashes, "totpLastStep": int64(0),
	})
}

// RecordTOTPStep stores the highest consumed time step, which is what makes a
// replay detectable.
func (r *AccountRepo) RecordTOTPStep(ctx context.Context, id string, step uint64) error {
	return r.set(ctx, id, bson.M{"totpLastStep": int64(step)})
}

func (r *AccountRepo) TOTPLastStep(ctx context.Context, id string) (uint64, error) {
	var d accountDoc
	if err := r.col().FindOne(ctx, bson.M{"_id": id},
		options.FindOne().SetProjection(bson.M{"totpLastStep": 1})).Decode(&d); err != nil {
		return 0, fmt.Errorf("read totp step: %w", err)
	}
	if d.TOTPLastStep < 0 {
		return 0, nil
	}
	return uint64(d.TOTPLastStep), nil
}

func (r *AccountRepo) RecoveryCodes(ctx context.Context, id string) ([]string, error) {
	var d accountDoc
	if err := r.col().FindOne(ctx, bson.M{"_id": id},
		options.FindOne().SetProjection(bson.M{"recoveryCodes": 1})).Decode(&d); err != nil {
		return nil, fmt.Errorf("read recovery codes: %w", err)
	}
	return d.RecoveryCodes, nil
}

func (r *AccountRepo) SetRecoveryCodes(ctx context.Context, id string, hashes []string) error {
	return r.set(ctx, id, bson.M{"recoveryCodes": hashes})
}

// ResetMFA is reserved for explicit operator recovery. It clears every TOTP
// credential together so an account cannot retain a secret without its
// matching recovery state. Callers must revoke sessions separately.
func (r *AccountRepo) ResetMFA(ctx context.Context, id string) error {
	return r.set(ctx, id, bson.M{
		"totpSecret": "", "totpLastStep": int64(0), "recoveryCodes": []string{},
	})
}

// BumpSessionEpoch is "sign out everywhere": one write, every session dies.
func (r *AccountRepo) BumpSessionEpoch(ctx context.Context, id string) error {
	_, err := r.col().UpdateOne(ctx, bson.M{"_id": id}, bson.M{
		"$inc": bson.M{"sessionEpoch": 1},
		"$set": bson.M{"updatedAt": time.Now().UTC()},
	})
	if err != nil {
		return fmt.Errorf("revoke sessions: %w", err)
	}
	return nil
}

func (r *AccountRepo) SetRole(ctx context.Context, id string, role account.Role) error {
	return r.set(ctx, id, bson.M{"role": string(role)})
}

func (r *AccountRepo) SetDisabled(ctx context.Context, id string, disabled bool) error {
	return r.set(ctx, id, bson.M{"disabled": disabled})
}

func (r *AccountRepo) set(ctx context.Context, id string, fields bson.M) error {
	fields["updatedAt"] = time.Now().UTC()
	res, err := r.col().UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": fields})
	if err != nil {
		return fmt.Errorf("update account: %w", err)
	}
	if res.MatchedCount == 0 {
		return ErrAccountNotFound
	}
	return nil
}
