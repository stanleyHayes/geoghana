package mongo

import (
	"context"
	"encoding/base64"
	"errors"
	"regexp"
	"strings"
	"time"

	app "github.com/ghanageo/ghanageo/services/api/internal/app/adminidentity"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/account"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/audit"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/identity"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/usage"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// AdminIdentityRepo is deliberately separate from tenant repositories. Its
// methods are global and must only be reached through adminidentity.Service.
type AdminIdentityRepo struct{ s *Store }

func NewAdminIdentityRepo(s *Store) *AdminIdentityRepo { return &AdminIdentityRepo{s: s} }

func decodeAdminCursor(v string) (string, error) {
	if v == "" {
		return "", nil
	}
	b, err := base64.RawURLEncoding.DecodeString(v)
	if err != nil || len(b) == 0 || len(b) > 128 {
		return "", identity.ErrInvalidCursor
	}
	return string(b), nil
}
func encodeAdminCursor(v string) string { return base64.RawURLEncoding.EncodeToString([]byte(v)) }
func searchRegex(v string) bson.Regex {
	return bson.Regex{Pattern: regexp.QuoteMeta(strings.TrimSpace(v)), Options: "i"}
}

func adminPageQuery(f identity.AdminListFilter, q bson.M) (bson.M, int, error) {
	f = f.Normalize()
	cursor, err := decodeAdminCursor(f.Cursor)
	if err != nil {
		return nil, 0, err
	}
	if cursor != "" {
		q["_id"] = bson.M{"$gt": cursor}
	}
	return q, f.Limit, nil
}

func (r *AdminIdentityRepo) Organizations(ctx context.Context, f identity.AdminListFilter) (identity.AdminPage[identity.Organization], error) {
	q := bson.M{}
	if f.Query != "" {
		rx := searchRegex(f.Query)
		q["$or"] = bson.A{bson.M{"name": rx}, bson.M{"members.email": rx}}
	}
	countQ := bson.M{}
	for k, v := range q {
		countQ[k] = v
	}
	q, limit, err := adminPageQuery(f, q)
	if err != nil {
		return identity.AdminPage[identity.Organization]{}, err
	}
	total, err := r.s.db.Collection(ColOrganizations).CountDocuments(ctx, countQ)
	if err != nil {
		return identity.AdminPage[identity.Organization]{}, err
	}
	cur, err := r.s.db.Collection(ColOrganizations).Find(ctx, q, options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}).SetLimit(int64(limit+1)))
	if err != nil {
		return identity.AdminPage[identity.Organization]{}, err
	}
	defer cur.Close(ctx)
	var docs []organizationDoc
	if err := cur.All(ctx, &docs); err != nil {
		return identity.AdminPage[identity.Organization]{}, err
	}
	p := identity.AdminPage[identity.Organization]{Data: make([]identity.Organization, 0, min(limit, len(docs))), Total: total}
	for i, d := range docs {
		if i == limit {
			p.NextCursor = encodeAdminCursor(docs[limit-1].ID)
			break
		}
		p.Data = append(p.Data, *organizationFromDoc(d))
	}
	return p, nil
}

func (r *AdminIdentityRepo) Accounts(ctx context.Context, f identity.AdminListFilter) (identity.AdminPage[identity.DeveloperAccount], error) {
	q := bson.M{}
	if f.Query != "" {
		rx := searchRegex(f.Query)
		q["$or"] = bson.A{bson.M{"email": rx}, bson.M{"_id": rx}}
	}
	countQ := bson.M{}
	for k, v := range q {
		countQ[k] = v
	}
	q, limit, err := adminPageQuery(f, q)
	if err != nil {
		return identity.AdminPage[identity.DeveloperAccount]{}, err
	}
	col := r.s.db.Collection(ColAccounts)
	total, err := col.CountDocuments(ctx, countQ)
	if err != nil {
		return identity.AdminPage[identity.DeveloperAccount]{}, err
	}
	cur, err := col.Find(ctx, q, options.Find().SetProjection(bson.M{"passwordHash": 0, "totpSecret": 0, "recoveryCodes": 0, "passkeys": 0, "sessionEpoch": 0}).SetSort(bson.D{{Key: "_id", Value: 1}}).SetLimit(int64(limit+1)))
	if err != nil {
		return identity.AdminPage[identity.DeveloperAccount]{}, err
	}
	defer cur.Close(ctx)
	var docs []accountDoc
	if err := cur.All(ctx, &docs); err != nil {
		return identity.AdminPage[identity.DeveloperAccount]{}, err
	}
	p := identity.AdminPage[identity.DeveloperAccount]{Data: make([]identity.DeveloperAccount, 0, min(limit, len(docs))), Total: total}
	for i, d := range docs {
		if i == limit {
			p.NextCursor = encodeAdminCursor(docs[limit-1].ID)
			break
		}
		p.Data = append(p.Data, identity.DeveloperAccount{ID: d.ID, Email: d.Email, EmailVerified: d.EmailVerified, Role: account.Role(d.Role), Disabled: d.Disabled, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt})
	}
	return p, nil
}

func (r *AdminIdentityRepo) Applications(ctx context.Context, f identity.AdminListFilter) (identity.AdminPage[identity.Application], error) {
	q := bson.M{}
	if f.OrganizationID != "" {
		q["organizationId"] = f.OrganizationID
	}
	if f.Query != "" {
		rx := searchRegex(f.Query)
		q["$or"] = bson.A{bson.M{"name": rx}, bson.M{"description": rx}, bson.M{"_id": rx}}
	}
	countQ := bson.M{}
	for k, v := range q {
		countQ[k] = v
	}
	q, limit, err := adminPageQuery(f, q)
	if err != nil {
		return identity.AdminPage[identity.Application]{}, err
	}
	col := r.s.db.Collection(ColApplications)
	total, err := col.CountDocuments(ctx, countQ)
	if err != nil {
		return identity.AdminPage[identity.Application]{}, err
	}
	cur, err := col.Find(ctx, q, options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}).SetLimit(int64(limit+1)))
	if err != nil {
		return identity.AdminPage[identity.Application]{}, err
	}
	defer cur.Close(ctx)
	var docs []applicationDoc
	if err := cur.All(ctx, &docs); err != nil {
		return identity.AdminPage[identity.Application]{}, err
	}
	p := identity.AdminPage[identity.Application]{Data: make([]identity.Application, 0, min(limit, len(docs))), Total: total}
	for i, d := range docs {
		if i == limit {
			p.NextCursor = encodeAdminCursor(docs[limit-1].ID)
			break
		}
		p.Data = append(p.Data, *applicationFromDoc(d))
	}
	return p, nil
}

func (r *AdminIdentityRepo) Keys(ctx context.Context, f identity.AdminListFilter) (identity.AdminPage[identity.AdminKey], error) {
	q := bson.M{}
	if f.OrganizationID != "" {
		q["organizationId"] = f.OrganizationID
	}
	if f.ApplicationID != "" {
		q["applicationId"] = f.ApplicationID
	}
	if f.Query != "" {
		rx := searchRegex(f.Query)
		q["$or"] = bson.A{bson.M{"name": rx}, bson.M{"prefix": rx}, bson.M{"_id": rx}}
	}
	switch f.State {
	case "active":
		q["revokedAt"] = bson.M{"$exists": false}
		q["suspendedAt"] = bson.M{"$exists": false}
	case "revoked":
		q["revokedAt"] = bson.M{"$exists": true}
	case "suspended":
		q["suspendedAt"] = bson.M{"$exists": true}
	}
	countQ := bson.M{}
	for k, v := range q {
		countQ[k] = v
	}
	q, limit, err := adminPageQuery(f, q)
	if err != nil {
		return identity.AdminPage[identity.AdminKey]{}, err
	}
	col := r.s.db.Collection(ColAPIKeys)
	total, err := col.CountDocuments(ctx, countQ)
	if err != nil {
		return identity.AdminPage[identity.AdminKey]{}, err
	}
	cur, err := col.Find(ctx, q, options.Find().SetProjection(bson.M{"secretHash": 0}).SetSort(bson.D{{Key: "_id", Value: 1}}).SetLimit(int64(limit+1)))
	if err != nil {
		return identity.AdminPage[identity.AdminKey]{}, err
	}
	defer cur.Close(ctx)
	var docs []apiKeyDoc
	if err := cur.All(ctx, &docs); err != nil {
		return identity.AdminPage[identity.AdminKey]{}, err
	}
	p := identity.AdminPage[identity.AdminKey]{Data: make([]identity.AdminKey, 0, min(limit, len(docs))), Total: total}
	for i, d := range docs {
		if i == limit {
			p.NextCursor = encodeAdminCursor(docs[limit-1].ID)
			break
		}
		p.Data = append(p.Data, identity.RedactKey(fromKeyDoc(d)))
	}
	return p, nil
}

func (r *AdminIdentityRepo) UsageSummary(ctx context.Context, orgID, appID string, since time.Time) (usage.Summary, error) {
	return NewUsageRepo(r.s).Summary(ctx, orgID, appID, since)
}
func (r *AdminIdentityRepo) RequestLog(ctx context.Context, orgID, appID string, before *time.Time, limit int) (usage.Page, error) {
	return NewUsageRepo(r.s).List(ctx, orgID, appID, before, limit)
}

func (r *AdminIdentityRepo) SetKeyState(ctx context.Context, keyID string, m app.KeyMutation, evidence audit.Entry) error {
	auditAppendMu.Lock()
	defer auditAppendMu.Unlock()
	session, err := r.s.client.StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)
	_, err = session.WithTransaction(ctx, func(tx context.Context) (any, error) {
		filter := bson.M{"_id": keyID, "revokedAt": bson.M{"$exists": false}}
		set := bson.M{}
		switch m.State {
		case "suspended":
			filter["suspendedAt"] = bson.M{"$exists": false}
			set["suspendedAt"] = m.At
			set["suspendedReason"] = m.Reason
		case "revoked":
			set["revokedAt"] = m.At
			set["revokedReason"] = m.Reason
		default:
			return nil, identity.ErrInvalidKeyTransition
		}
		res, updateErr := r.s.db.Collection(ColAPIKeys).UpdateOne(tx, filter, bson.M{"$set": set})
		if updateErr != nil {
			return nil, updateErr
		}
		if res.MatchedCount == 0 {
			var d apiKeyDoc
			findErr := r.s.db.Collection(ColAPIKeys).FindOne(tx, bson.M{"_id": keyID}).Decode(&d)
			if errors.Is(findErr, mongo.ErrNoDocuments) {
				return nil, identity.ErrNotFound
			}
			if findErr != nil {
				return nil, findErr
			}
			if (m.State == "suspended" && d.SuspendedAt != nil) || (m.State == "revoked" && d.RevokedAt != nil) {
				return nil, nil
			}
			return nil, identity.ErrInvalidKeyTransition
		}
		_, appendErr := NewAuditRepo(r.s).append(tx, evidence)
		return nil, appendErr
	})
	return err
}
