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
	ColOrganizations  = "organizations"
	ColApplications   = "applications"
	ColAPIKeys        = "api_keys"
	ColOrgInvitations = "organization_invitations"
)

type organizationDoc struct {
	ID        string                  `bson:"_id"`
	Name      string                  `bson:"name"`
	OwnerID   string                  `bson:"ownerId"`
	Members   []organizationMemberDoc `bson:"members"`
	CreatedAt time.Time               `bson:"createdAt"`
}

type organizationMemberDoc struct {
	AccountID string    `bson:"accountId"`
	Email     string    `bson:"email"`
	Role      string    `bson:"role"`
	JoinedAt  time.Time `bson:"joinedAt"`
}

type applicationDoc struct {
	ID             string    `bson:"_id"`
	OrganizationID string    `bson:"organizationId"`
	Name           string    `bson:"name"`
	Description    string    `bson:"description,omitempty"`
	Environments   []string  `bson:"environments"`
	Domains        []string  `bson:"domains,omitempty"`
	CallbackURL    string    `bson:"callbackUrl,omitempty"`
	Plan           string    `bson:"plan"`
	CreatedAt      time.Time `bson:"createdAt"`
}

type organizationInvitationDoc struct {
	ID             string     `bson:"_id"`
	OrganizationID string     `bson:"organizationId"`
	Email          string     `bson:"email"`
	Role           string     `bson:"role"`
	TokenHash      string     `bson:"tokenHash"`
	Status         string     `bson:"status"`
	InvitedBy      string     `bson:"invitedBy"`
	CreatedAt      time.Time  `bson:"createdAt"`
	ExpiresAt      time.Time  `bson:"expiresAt"`
	AcceptedAt     *time.Time `bson:"acceptedAt,omitempty"`
}

type OrganizationRepo struct{ col *mongo.Collection }

func NewOrganizationRepo(s *Store) *OrganizationRepo {
	return &OrganizationRepo{col: s.db.Collection(ColOrganizations)}
}

func (r *OrganizationRepo) Create(ctx context.Context, org identity.Organization) error {
	if err := org.Validate(); err != nil {
		return err
	}
	members := make([]organizationMemberDoc, 0, len(org.Members))
	for _, m := range org.Members {
		members = append(members, organizationMemberDoc{AccountID: m.AccountID, Email: m.Email, Role: string(m.Role), JoinedAt: m.JoinedAt})
	}
	_, err := r.col.InsertOne(ctx, organizationDoc{ID: org.ID, Name: org.Name, OwnerID: org.OwnerID, Members: members, CreatedAt: org.CreatedAt})
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
	return organizationFromDoc(d), nil
}

func (r *OrganizationRepo) ByIDForAccount(ctx context.Context, id, accountID string) (*identity.Organization, error) {
	var d organizationDoc
	if err := r.col.FindOne(ctx, bson.M{"_id": id, "members.accountId": accountID}).Decode(&d); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, identity.ErrNotFound
		}
		return nil, err
	}
	return organizationFromDoc(d), nil
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
		out = append(out, *organizationFromDoc(d))
	}
	return out, nil
}

func (r *OrganizationRepo) ListByAccount(ctx context.Context, accountID string) ([]identity.Organization, error) {
	cur, err := r.col.Find(ctx, bson.M{"members.accountId": accountID}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}))
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
		out = append(out, *organizationFromDoc(d))
	}
	return out, nil
}

func organizationFromDoc(d organizationDoc) *identity.Organization {
	members := make([]identity.OrganizationMember, 0, len(d.Members))
	for _, m := range d.Members {
		members = append(members, identity.OrganizationMember{AccountID: m.AccountID, Email: m.Email, Role: identity.OrganizationRole(m.Role), JoinedAt: m.JoinedAt})
	}
	return &identity.Organization{ID: d.ID, Name: d.Name, OwnerID: d.OwnerID, Members: members, CreatedAt: d.CreatedAt}
}

func (r *OrganizationRepo) AddMember(ctx context.Context, orgID string, member identity.OrganizationMember) error {
	res, err := r.col.UpdateOne(ctx, bson.M{"_id": orgID, "members.accountId": bson.M{"$ne": member.AccountID}}, bson.M{"$push": bson.M{"members": organizationMemberDoc{AccountID: member.AccountID, Email: member.Email, Role: string(member.Role), JoinedAt: member.JoinedAt}}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return identity.ErrNotFound
	}
	return nil
}

func (r *OrganizationRepo) TransferOwnership(ctx context.Context, orgID, currentOwnerID, nextOwnerID string, at time.Time) error {
	res, err := r.col.UpdateOne(ctx, bson.M{"_id": orgID, "ownerId": currentOwnerID, "members.accountId": nextOwnerID}, bson.M{"$set": bson.M{"ownerId": nextOwnerID, "members.$[next].role": string(identity.OrganizationOwner), "members.$[old].role": string(identity.OrganizationAdmin)}}, options.UpdateOne().SetArrayFilters([]any{bson.M{"next.accountId": nextOwnerID}, bson.M{"old.accountId": currentOwnerID}}))
	_ = at
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return identity.ErrNotFound
	}
	return nil
}

type ApplicationRepo struct{ col *mongo.Collection }

func NewApplicationRepo(s *Store) *ApplicationRepo {
	return &ApplicationRepo{col: s.db.Collection(ColApplications)}
}

func (r *ApplicationRepo) Create(ctx context.Context, app identity.Application) error {
	if err := app.Validate(); err != nil {
		return err
	}
	envs := make([]string, 0, len(app.Environments))
	for _, e := range app.Environments {
		envs = append(envs, string(e))
	}
	_, err := r.col.InsertOne(ctx, applicationDoc{ID: app.ID, OrganizationID: app.OrganizationID, Name: app.Name, Description: app.Description, Environments: envs, Domains: app.Domains, CallbackURL: app.CallbackURL, Plan: app.Plan, CreatedAt: app.CreatedAt})
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
	return applicationFromDoc(d), nil
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
		out = append(out, *applicationFromDoc(d))
	}
	return out, nil
}

func applicationFromDoc(d applicationDoc) *identity.Application {
	envs := make([]identity.Environment, 0, len(d.Environments))
	for _, e := range d.Environments {
		envs = append(envs, identity.Environment(e))
	}
	return &identity.Application{ID: d.ID, OrganizationID: d.OrganizationID, Name: d.Name, Description: d.Description, Environments: envs, Domains: d.Domains, CallbackURL: d.CallbackURL, Plan: d.Plan, CreatedAt: d.CreatedAt}
}

type OrganizationInvitationRepo struct{ col *mongo.Collection }

func NewOrganizationInvitationRepo(s *Store) *OrganizationInvitationRepo {
	return &OrganizationInvitationRepo{col: s.db.Collection(ColOrgInvitations)}
}
func invitationFromDoc(d organizationInvitationDoc) identity.OrganizationInvitation {
	return identity.OrganizationInvitation{ID: d.ID, OrganizationID: d.OrganizationID, Email: d.Email, Role: identity.OrganizationRole(d.Role), TokenHash: d.TokenHash, Status: identity.InvitationStatus(d.Status), InvitedBy: d.InvitedBy, CreatedAt: d.CreatedAt, ExpiresAt: d.ExpiresAt, AcceptedAt: d.AcceptedAt}
}
func (r *OrganizationInvitationRepo) Create(ctx context.Context, v identity.OrganizationInvitation) error {
	_, err := r.col.InsertOne(ctx, organizationInvitationDoc{ID: v.ID, OrganizationID: v.OrganizationID, Email: v.Email, Role: string(v.Role), TokenHash: v.TokenHash, Status: string(v.Status), InvitedBy: v.InvitedBy, CreatedAt: v.CreatedAt, ExpiresAt: v.ExpiresAt})
	return err
}
func (r *OrganizationInvitationRepo) ListPending(ctx context.Context, orgID string) ([]identity.OrganizationInvitation, error) {
	cur, err := r.col.Find(ctx, bson.M{"organizationId": orgID, "status": string(identity.InvitationPending)}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var docs []organizationInvitationDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]identity.OrganizationInvitation, 0, len(docs))
	for _, d := range docs {
		out = append(out, invitationFromDoc(d))
	}
	return out, nil
}
func (r *OrganizationInvitationRepo) ByTokenHash(ctx context.Context, hash string) (*identity.OrganizationInvitation, error) {
	var d organizationInvitationDoc
	if err := r.col.FindOne(ctx, bson.M{"tokenHash": hash, "status": string(identity.InvitationPending)}).Decode(&d); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, identity.ErrNotFound
		}
		return nil, err
	}
	v := invitationFromDoc(d)
	return &v, nil
}
func (r *OrganizationInvitationRepo) Accept(ctx context.Context, id string, at time.Time) error {
	res, err := r.col.UpdateOne(ctx, bson.M{"_id": id, "status": string(identity.InvitationPending)}, bson.M{"$set": bson.M{"status": string(identity.InvitationAccepted), "acceptedAt": at}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return identity.ErrNotFound
	}
	return nil
}
func (r *OrganizationInvitationRepo) Revoke(ctx context.Context, id, orgID string) error {
	res, err := r.col.UpdateOne(ctx, bson.M{"_id": id, "organizationId": orgID, "status": string(identity.InvitationPending)}, bson.M{"$set": bson.M{"status": string(identity.InvitationRevoked)}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return identity.ErrNotFound
	}
	return nil
}

// The stored key never contains the secret — only the public prefix and an
// argon2id digest (Spec §12.2).
type apiKeyDoc struct {
	ID              string     `bson:"_id"`
	ApplicationID   string     `bson:"applicationId"`
	OrganizationID  string     `bson:"organizationId"`
	Name            string     `bson:"name"`
	Class           string     `bson:"class"`
	Environment     string     `bson:"environment"`
	Prefix          string     `bson:"prefix"`
	SecretHash      string     `bson:"secretHash"`
	Scopes          []string   `bson:"scopes"`
	AllowedOrigins  []string   `bson:"allowedOrigins,omitempty"`
	AllowedIPs      []string   `bson:"allowedIps,omitempty"`
	CreatedAt       time.Time  `bson:"createdAt"`
	ExpiresAt       *time.Time `bson:"expiresAt,omitempty"`
	LastUsedAt      *time.Time `bson:"lastUsedAt,omitempty"`
	RevokedAt       *time.Time `bson:"revokedAt,omitempty"`
	RevokedReason   string     `bson:"revokedReason,omitempty"`
	SuspendedAt     *time.Time `bson:"suspendedAt,omitempty"`
	SuspendedReason string     `bson:"suspendedReason,omitempty"`
	Elevated        bool       `bson:"elevated,omitempty"`
	ElevatedReason  string     `bson:"elevatedReason,omitempty"`
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
		RevokedAt: k.RevokedAt, RevokedReason: k.RevokedReason,
		SuspendedAt: k.SuspendedAt, SuspendedReason: k.SuspendedReason,
		Elevated: k.Elevated, ElevatedReason: k.ElevatedReason,
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
		RevokedAt: d.RevokedAt, RevokedReason: d.RevokedReason,
		SuspendedAt: d.SuspendedAt, SuspendedReason: d.SuspendedReason,
		Elevated: d.Elevated, ElevatedReason: d.ElevatedReason,
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
