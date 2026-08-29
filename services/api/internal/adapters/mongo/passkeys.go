package mongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/account"
)

var ErrChallengeNotFound = errors.New("challenge not found or expired")

// ChallengeTTL bounds how long a ceremony may take. Long enough for someone to
// find their phone, short enough that a captured challenge is not useful later.
const ChallengeTTL = 5 * time.Minute

type challengeDoc struct {
	ID        string    `bson:"_id"`
	AccountID string    `bson:"accountId,omitempty"`
	Purpose   string    `bson:"purpose"`
	Session   []byte    `bson:"session"`
	ExpiresAt time.Time `bson:"expiresAt"`
}

// ChallengeRepo stores in-flight WebAuthn ceremonies.
//
// The challenge lives SERVER-SIDE and is looked up by an opaque id handed to
// the client. Round-tripping it through the browser would let a caller choose
// its own challenge, which defeats the entire protocol.
type ChallengeRepo struct{ s *Store }

func NewChallengeRepo(s *Store) *ChallengeRepo { return &ChallengeRepo{s: s} }

func (r *ChallengeRepo) col() *mongo.Collection { return r.s.db.Collection(ColWebAuthnChal) }

func (r *ChallengeRepo) Put(ctx context.Context, accountID, purpose string, session []byte) (string, error) {
	id, err := account.NewID("chal")
	if err != nil {
		return "", err
	}
	_, err = r.col().InsertOne(ctx, challengeDoc{
		ID: id, AccountID: accountID, Purpose: purpose, Session: session,
		ExpiresAt: time.Now().UTC().Add(ChallengeTTL),
	})
	if err != nil {
		return "", fmt.Errorf("store challenge: %w", err)
	}
	return id, nil
}

// Take fetches a challenge and deletes it in one operation.
//
// FindOneAndDelete rather than find-then-delete: a challenge is single use,
// and two concurrent completions must not both succeed.
func (r *ChallengeRepo) Take(ctx context.Context, id, purpose string) (accountID string, session []byte, err error) {
	var d challengeDoc
	if err := r.col().FindOneAndDelete(ctx,
		bson.M{"_id": id, "purpose": purpose}).Decode(&d); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return "", nil, ErrChallengeNotFound
		}
		return "", nil, fmt.Errorf("take challenge: %w", err)
	}
	// The TTL sweep is periodic, so an expired row can still be present.
	if time.Now().UTC().After(d.ExpiresAt) {
		return "", nil, ErrChallengeNotFound
	}
	return d.AccountID, d.Session, nil
}

// AddPasskey appends a credential to an account.
func (r *AccountRepo) AddPasskey(ctx context.Context, accountID string, p account.Passkey) error {
	res, err := r.col().UpdateOne(ctx, bson.M{"_id": accountID}, bson.M{
		"$push": bson.M{"passkeys": passkeyDoc{
			ID: p.ID, Name: p.Name, Credential: p.Credential,
			SignCount: p.SignCount, BackedUp: p.BackedUp, AddedAt: p.AddedAt,
		}},
		"$set": bson.M{"updatedAt": time.Now().UTC()},
	})
	if err != nil {
		return fmt.Errorf("add passkey: %w", err)
	}
	if res.MatchedCount == 0 {
		return ErrAccountNotFound
	}
	return nil
}

// RemovePasskey deletes a credential.
func (r *AccountRepo) RemovePasskey(ctx context.Context, accountID, passkeyID string) error {
	res, err := r.col().UpdateOne(ctx, bson.M{"_id": accountID}, bson.M{
		"$pull": bson.M{"passkeys": bson.M{"id": passkeyID}},
		"$set":  bson.M{"updatedAt": time.Now().UTC()},
	})
	if err != nil {
		return fmt.Errorf("remove passkey: %w", err)
	}
	if res.MatchedCount == 0 {
		return ErrAccountNotFound
	}
	return nil
}

// UpdatePasskeyUse records the new sign count and last-used time.
func (r *AccountRepo) UpdatePasskeyUse(ctx context.Context, accountID, passkeyID string, signCount uint32) error {
	_, err := r.col().UpdateOne(ctx,
		bson.M{"_id": accountID, "passkeys.id": passkeyID},
		bson.M{"$set": bson.M{
			"passkeys.$.signCount": signCount,
			"passkeys.$.lastUsed":  time.Now().UTC(),
			"updatedAt":            time.Now().UTC(),
		}})
	if err != nil {
		return fmt.Errorf("update passkey use: %w", err)
	}
	return nil
}
