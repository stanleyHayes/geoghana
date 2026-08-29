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

var (
	ErrSessionNotFound = errors.New("session not found")
	// ErrSessionReplayed means a superseded token was presented. That is not
	// an expiry — it is evidence the token chain leaked, so the caller kills
	// every session for the account rather than merely rejecting this one.
	ErrSessionReplayed = errors.New("a superseded session token was presented")
)

type sessionDoc struct {
	ID          string     `bson:"_id"`
	AccountID   string     `bson:"accountId"`
	Role        string     `bson:"role"`
	Stage       string     `bson:"stage"`
	TokenHash   string     `bson:"tokenHash"`
	Epoch       int        `bson:"epoch"`
	IssuedAt    time.Time  `bson:"issuedAt"`
	ExpiresAt   time.Time  `bson:"expiresAt"`
	LastUsedAt  time.Time  `bson:"lastUsedAt"`
	RotatedFrom string     `bson:"rotatedFrom,omitempty"`
	RevokedAt   *time.Time `bson:"revokedAt,omitempty"`
	UserAgent   string     `bson:"userAgent,omitempty"`
	IP          string     `bson:"ip,omitempty"`
	// SupersededAt marks a session that has been rotated away from. The row is
	// KEPT rather than deleted so presenting its token is recognisable as a
	// replay; deleting it would make theft look like an ordinary expiry.
	SupersededAt *time.Time `bson:"supersededAt,omitempty"`
}

func (d sessionDoc) toDomain() account.Session {
	return account.Session{
		ID: d.ID, AccountID: d.AccountID, Role: account.Role(d.Role),
		Stage: account.Stage(d.Stage), TokenHash: d.TokenHash, Epoch: d.Epoch,
		IssuedAt: d.IssuedAt, ExpiresAt: d.ExpiresAt, LastUsedAt: d.LastUsedAt,
		RotatedFrom: d.RotatedFrom, RevokedAt: d.RevokedAt,
		UserAgent: d.UserAgent, IP: d.IP,
	}
}

func fromSession(s account.Session) sessionDoc {
	return sessionDoc{
		ID: s.ID, AccountID: s.AccountID, Role: string(s.Role), Stage: string(s.Stage),
		TokenHash: s.TokenHash, Epoch: s.Epoch, IssuedAt: s.IssuedAt,
		ExpiresAt: s.ExpiresAt, LastUsedAt: s.LastUsedAt, RotatedFrom: s.RotatedFrom,
		RevokedAt: s.RevokedAt, UserAgent: s.UserAgent, IP: s.IP,
	}
}

type SessionRepo struct{ s *Store }

func NewSessionRepo(s *Store) *SessionRepo { return &SessionRepo{s: s} }

func (r *SessionRepo) col() *mongo.Collection { return r.s.db.Collection(ColSessions) }

func (r *SessionRepo) Create(ctx context.Context, s account.Session) error {
	if _, err := r.col().InsertOne(ctx, fromSession(s)); err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// ByToken resolves a presented token.
//
// Lookup is by the token's HASH, so the raw token is never in a query, a log
// or an index. A superseded row is reported distinctly, because presenting an
// already-rotated token means the chain leaked.
func (r *SessionRepo) ByToken(ctx context.Context, token string) (account.Session, error) {
	var d sessionDoc
	err := r.col().FindOne(ctx, bson.M{"tokenHash": account.HashToken(token)}).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return account.Session{}, ErrSessionNotFound
	}
	if err != nil {
		return account.Session{}, fmt.Errorf("find session: %w", err)
	}
	if d.SupersededAt != nil {
		return d.toDomain(), ErrSessionReplayed
	}
	return d.toDomain(), nil
}

// Rotate supersedes the old row and inserts the replacement in one logical
// step. The old row stays so a replay of its token is still detectable.
func (r *SessionRepo) Rotate(ctx context.Context, oldID string, next account.Session) error {
	now := time.Now().UTC()
	if _, err := r.col().UpdateOne(ctx, bson.M{"_id": oldID},
		bson.M{"$set": bson.M{"supersededAt": now}}); err != nil {
		return fmt.Errorf("supersede session: %w", err)
	}
	return r.Create(ctx, next)
}

func (r *SessionRepo) Revoke(ctx context.Context, id string) error {
	now := time.Now().UTC()
	res, err := r.col().UpdateOne(ctx, bson.M{"_id": id},
		bson.M{"$set": bson.M{"revokedAt": now}})
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	if res.MatchedCount == 0 {
		return ErrSessionNotFound
	}
	return nil
}

// RevokeAllForAccount is belt to the account epoch's braces. The epoch alone
// already invalidates every session; marking the rows too means a listing of
// active sessions immediately reflects the truth.
func (r *SessionRepo) RevokeAllForAccount(ctx context.Context, accountID string) (int64, error) {
	now := time.Now().UTC()
	res, err := r.col().UpdateMany(ctx,
		bson.M{"accountId": accountID, "revokedAt": bson.M{"$exists": false}},
		bson.M{"$set": bson.M{"revokedAt": now}})
	if err != nil {
		return 0, fmt.Errorf("revoke sessions: %w", err)
	}
	return res.ModifiedCount, nil
}

// ListActive powers the "active sessions" screen a user can revoke from.
func (r *SessionRepo) ListActive(ctx context.Context, accountID string) ([]account.Session, error) {
	cur, err := r.col().Find(ctx, bson.M{
		"accountId":    accountID,
		"revokedAt":    bson.M{"$exists": false},
		"supersededAt": bson.M{"$exists": false},
		"expiresAt":    bson.M{"$gt": time.Now().UTC()},
	}, options.Find().SetSort(bson.D{{Key: "lastUsedAt", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()

	var out []account.Session
	for cur.Next(ctx) {
		var d sessionDoc
		if err := cur.Decode(&d); err != nil {
			return nil, fmt.Errorf("decode session: %w", err)
		}
		out = append(out, d.toDomain())
	}
	return out, cur.Err()
}

// OneTimeTokenRepo stores verification, reset and recovery tokens.
type OneTimeTokenRepo struct{ s *Store }

func NewOneTimeTokenRepo(s *Store) *OneTimeTokenRepo { return &OneTimeTokenRepo{s: s} }

func (r *OneTimeTokenRepo) col() *mongo.Collection { return r.s.db.Collection(ColOneTimeTokens) }

type ottDoc struct {
	ID        string     `bson:"_id"`
	AccountID string     `bson:"accountId"`
	Purpose   string     `bson:"purpose"`
	Hash      string     `bson:"hash"`
	ExpiresAt time.Time  `bson:"expiresAt"`
	UsedAt    *time.Time `bson:"usedAt,omitempty"`
	CreatedAt time.Time  `bson:"createdAt"`
}

func (r *OneTimeTokenRepo) Create(ctx context.Context, t account.OneTimeToken) error {
	_, err := r.col().InsertOne(ctx, ottDoc{
		ID: t.ID, AccountID: t.AccountID, Purpose: string(t.Purpose),
		Hash: t.Hash, ExpiresAt: t.ExpiresAt, CreatedAt: t.CreatedAt,
	})
	if err != nil {
		return fmt.Errorf("create token: %w", err)
	}
	return nil
}

func (r *OneTimeTokenRepo) ByValue(ctx context.Context, value string) (account.OneTimeToken, error) {
	var d ottDoc
	err := r.col().FindOne(ctx, bson.M{"hash": account.HashToken(value)}).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return account.OneTimeToken{}, ErrSessionNotFound
	}
	if err != nil {
		return account.OneTimeToken{}, fmt.Errorf("find token: %w", err)
	}
	return account.OneTimeToken{
		ID: d.ID, AccountID: d.AccountID, Purpose: account.Purpose(d.Purpose),
		Hash: d.Hash, ExpiresAt: d.ExpiresAt, UsedAt: d.UsedAt, CreatedAt: d.CreatedAt,
	}, nil
}

// MarkUsed consumes a token. The update is CONDITIONAL on it being unused, so
// two simultaneous redemptions cannot both succeed — checking then writing
// would leave exactly that race open.
func (r *OneTimeTokenRepo) MarkUsed(ctx context.Context, id string) error {
	now := time.Now().UTC()
	res, err := r.col().UpdateOne(ctx,
		bson.M{"_id": id, "usedAt": bson.M{"$exists": false}},
		bson.M{"$set": bson.M{"usedAt": now}})
	if err != nil {
		return fmt.Errorf("consume token: %w", err)
	}
	if res.MatchedCount == 0 {
		return account.ErrTokenAlreadyUsed
	}
	return nil
}
