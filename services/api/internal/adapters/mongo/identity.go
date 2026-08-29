package mongo

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/identity"
)

const (
	ColOrganizations = "organizations"
	ColApplications  = "applications"
	ColAPIKeys       = "api_keys"
)

type organizationDoc struct {
	ID        string    `bson:"_id"`
	Name      string    `bson:"name"`
	OwnerID   string    `bson:"ownerId"`
	CreatedAt time.Time `bson:"createdAt"`
}

type applicationDoc struct {
	ID             string    `bson:"_id"`
	OrganizationID string    `bson:"organizationId"`
	Name           string    `bson:"name"`
	Description    string    `bson:"description,omitempty"`
	CreatedAt      time.Time `bson:"createdAt"`
}

type OrganizationRepo struct{ col *mongo.Collection }

func NewOrganizationRepo(s *Store) *OrganizationRepo {
	return &OrganizationRepo{col: s.db.Collection(ColOrganizations)}
}

func (r *OrganizationRepo) Create(ctx context.Context, org identity.Organization) error {
	if err := org.Validate(); err != nil {
		return err
	}
	_, err := r.col.InsertOne(ctx, organizationDoc{ID: org.ID, Name: org.Name, OwnerID: org.OwnerID, CreatedAt: org.CreatedAt})
	return err
}

func (r *OrganizationRepo) ByIDForOwner(ctx context.Context, id, ownerID string) (*identity.Organization, error) {
	var d organizationDoc
	if err := r.col.FindOne(ctx, bson.M{"_id": id, "ownerId": ownerID}).Decode(&d); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, identity.ErrNotFound
		}
		return nil, err
	}
	return &identity.Organization{ID: d.ID, Name: d.Name, OwnerID: d.OwnerID, CreatedAt: d.CreatedAt}, nil
}

func (r *OrganizationRepo) ListByOwner(ctx context.Context, ownerID string) ([]identity.Organization, error) {
	cur, err := r.col.Find(ctx, bson.M{"ownerId": ownerID}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var docs []organizationDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]identity.Organization, 0, len(docs))
	for _, d := range docs {
		out = append(out, identity.Organization{ID: d.ID, Name: d.Name, OwnerID: d.OwnerID, CreatedAt: d.CreatedAt})
	}
	return out, nil
}

type ApplicationRepo struct{ col *mongo.Collection }

func NewApplicationRepo(s *Store) *ApplicationRepo {
	return &ApplicationRepo{col: s.db.Collection(ColApplications)}
}

func (r *ApplicationRepo) Create(ctx context.Context, app identity.Application) error {
	if err := app.Validate(); err != nil {
		return err
	}
	_, err := r.col.InsertOne(ctx, applicationDoc{ID: app.ID, OrganizationID: app.OrganizationID, Name: app.Name, Description: app.Description, CreatedAt: app.CreatedAt})
	return err
}

func (r *ApplicationRepo) ByID(ctx context.Context, id, orgID string) (*identity.Application, error) {
	var d applicationDoc
	if err := r.col.FindOne(ctx, bson.M{"_id": id, "organizationId": orgID}).Decode(&d); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, identity.ErrNotFound
		}
		return nil, err
	}
	return &identity.Application{ID: d.ID, OrganizationID: d.OrganizationID, Name: d.Name, Description: d.Description, CreatedAt: d.CreatedAt}, nil
}

func (r *ApplicationRepo) ListByOrganization(ctx context.Context, orgID string) ([]identity.Application, error) {
	cur, err := r.col.Find(ctx, bson.M{"organizationId": orgID}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var docs []applicationDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]identity.Application, 0, len(docs))
	for _, d := range docs {
		out = append(out, identity.Application{ID: d.ID, OrganizationID: d.OrganizationID, Name: d.Name, Description: d.Description, CreatedAt: d.CreatedAt})
	}
	return out, nil
}

// The stored key never contains the secret — only the public prefix and an
// argon2id digest (Spec §12.2).
type apiKeyDoc struct {
	ID             string     `bson:"_id"`
	ApplicationID  string     `bson:"applicationId"`
	OrganizationID string     `bson:"organizationId"`
	Name           string     `bson:"name"`
	Class          string     `bson:"class"`
	Environment    string     `bson:"environment"`
	Prefix         string     `bson:"prefix"`
	SecretHash     string     `bson:"secretHash"`
	Scopes         []string   `bson:"scopes"`
	AllowedOrigins []string   `bson:"allowedOrigins,omitempty"`
	AllowedIPs     []string   `bson:"allowedIps,omitempty"`
	CreatedAt      time.Time  `bson:"createdAt"`
	ExpiresAt      *time.Time `bson:"expiresAt,omitempty"`
	LastUsedAt     *time.Time `bson:"lastUsedAt,omitempty"`
	RevokedAt      *time.Time `bson:"revokedAt,omitempty"`
	Elevated       bool       `bson:"elevated,omitempty"`
	ElevatedReason string     `bson:"elevatedReason,omitempty"`
}

func toKeyDoc(k identity.APIKey) apiKeyDoc {
	scopes := make([]string, 0, len(k.Scopes))
	for _, s := range k.Scopes {
		scopes = append(scopes, string(s))
	}
	return apiKeyDoc{
		ID: k.ID, ApplicationID: k.ApplicationID, OrganizationID: k.OrganizationID,
		Name: k.Name, Class: string(k.Class), Environment: string(k.Environment),
		Prefix: k.Prefix, SecretHash: k.SecretHash, Scopes: scopes,
		AllowedOrigins: k.AllowedOrigins, AllowedIPs: k.AllowedIPs,
		CreatedAt: k.CreatedAt, ExpiresAt: k.ExpiresAt, LastUsedAt: k.LastUsedAt,
		RevokedAt: k.RevokedAt, Elevated: k.Elevated, ElevatedReason: k.ElevatedReason,
	}
}

func fromKeyDoc(d apiKeyDoc) identity.APIKey {
	scopes := make([]identity.Scope, 0, len(d.Scopes))
	for _, s := range d.Scopes {
		scopes = append(scopes, identity.Scope(s))
	}
	return identity.APIKey{
		ID: d.ID, ApplicationID: d.ApplicationID, OrganizationID: d.OrganizationID,
		Name: d.Name, Class: identity.KeyClass(d.Class),
		Environment: identity.Environment(d.Environment),
		Prefix:      d.Prefix, SecretHash: d.SecretHash, Scopes: scopes,
		AllowedOrigins: d.AllowedOrigins, AllowedIPs: d.AllowedIPs,
		CreatedAt: d.CreatedAt, ExpiresAt: d.ExpiresAt, LastUsedAt: d.LastUsedAt,
		RevokedAt: d.RevokedAt, Elevated: d.Elevated, ElevatedReason: d.ElevatedReason,
	}
}

type KeyRepo struct{ col *mongo.Collection }

func NewKeyRepo(s *Store) *KeyRepo { return &KeyRepo{col: s.db.Collection(ColAPIKeys)} }

// ByPrefix resolves a public prefix. Returns (nil, nil) for an unknown prefix
// so the caller can return the same error as a wrong secret, preventing
// enumeration of which prefixes exist.
func (r *KeyRepo) ByPrefix(ctx context.Context, prefix string) (*identity.APIKey, error) {
	var d apiKeyDoc
	if err := r.col.FindOne(ctx, bson.M{"prefix": prefix}).Decode(&d); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	k := fromKeyDoc(d)
	return &k, nil
}

// MarkUsed records last-used metadata. Best effort: never fail a request for it.
func (r *KeyRepo) MarkUsed(ctx context.Context, prefix string, at time.Time) error {
	_, err := r.col.UpdateOne(ctx, bson.M{"prefix": prefix}, bson.M{"$set": bson.M{"lastUsedAt": at}})
	return err
}

func (r *KeyRepo) Create(ctx context.Context, k identity.APIKey) error {
	if err := k.Validate(); err != nil {
		return err
	}
	_, err := r.col.InsertOne(ctx, toKeyDoc(k))
	return err
}

// Revoke takes effect immediately — the admin emergency control of Spec §13.
func (r *KeyRepo) Revoke(ctx context.Context, prefix string, at time.Time) error {
	res, err := r.col.UpdateOne(ctx, bson.M{"prefix": prefix}, bson.M{"$set": bson.M{"revokedAt": at}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return identity.ErrNotFound
	}
	return nil
}

func (r *KeyRepo) ListByOrganization(ctx context.Context, orgID string) ([]identity.APIKey, error) {
	cur, err := r.col.Find(ctx, bson.M{"organizationId": orgID},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var docs []apiKeyDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]identity.APIKey, 0, len(docs))
	for _, d := range docs {
		out = append(out, fromKeyDoc(d))
	}
	return out, nil
}

func (r *KeyRepo) ByIDForApplication(ctx context.Context, id, appID string) (*identity.APIKey, error) {
	var d apiKeyDoc
	if err := r.col.FindOne(ctx, bson.M{"_id": id, "applicationId": appID}).Decode(&d); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, identity.ErrNotFound
		}
		return nil, err
	}
	k := fromKeyDoc(d)
	return &k, nil
}

func (r *KeyRepo) ListByApplication(ctx context.Context, appID string) ([]identity.APIKey, error) {
	cur, err := r.col.Find(ctx, bson.M{"applicationId": appID}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var docs []apiKeyDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]identity.APIKey, 0, len(docs))
	for _, d := range docs {
		out = append(out, fromKeyDoc(d))
	}
	return out, nil
}
