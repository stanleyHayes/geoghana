package mongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/audit"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/identity"
)

type allowanceDoc struct {
	BurstUnits      int     `bson:"burstUnits"`
	RefillPerSecond float64 `bson:"refillPerSecond"`
	WindowSeconds   int64   `bson:"windowSeconds"`
}

// AppendPolicyAudited commits the immutable policy revision and its audit
// evidence together. A policy without evidence, or evidence for a policy that
// did not commit, must never be observable.
func (r *FairUseRepo) AppendPolicyAudited(ctx context.Context, p identity.FairUsePolicy, evidence audit.Entry) error {
	if err := p.Validate(); err != nil {
		return err
	}
	return r.appendAudited(ctx, func(tx context.Context) error {
		if _, err := r.policies.InsertOne(tx, toPolicyDoc(p)); err != nil {
			return fmt.Errorf("append fair-use policy: %w", err)
		}
		return nil
	}, evidence)
}

func (r *FairUseRepo) AppendOverrideAudited(ctx context.Context, o identity.FairUseOverride, evidence audit.Entry) error {
	if err := o.Validate(); err != nil {
		return err
	}
	return r.appendAudited(ctx, func(tx context.Context) error {
		if _, err := r.overrides.InsertOne(tx, toOverrideDoc(o)); err != nil {
			return fmt.Errorf("append fair-use override: %w", err)
		}
		return nil
	}, evidence)
}

func (r *FairUseRepo) appendAudited(ctx context.Context, insert func(context.Context) error, evidence audit.Entry) error {
	auditAppendMu.Lock()
	defer auditAppendMu.Unlock()
	session, err := r.s.client.StartSession()
	if err != nil {
		return fmt.Errorf("start fair-use transaction: %w", err)
	}
	defer session.EndSession(ctx)
	_, err = session.WithTransaction(ctx, func(tx context.Context) (any, error) {
		if err := insert(tx); err != nil {
			return nil, err
		}
		if _, err := NewAuditRepo(r.s).append(tx, evidence); err != nil {
			return nil, err
		}
		return nil, nil
	})
	return err
}

type fairUsePolicyDoc struct {
	ID            string         `bson:"_id"`
	PolicyID      string         `bson:"policyId"`
	Revision      int64          `bson:"revision"`
	Anonymous     allowanceDoc   `bson:"anonymous"`
	Authenticated allowanceDoc   `bson:"authenticated"`
	Sandbox       allowanceDoc   `bson:"sandbox"`
	CostCeilings  map[string]int `bson:"costCeilings"`
	Reason        string         `bson:"reason"`
	ActorID       string         `bson:"actorId"`
	CreatedAt     time.Time      `bson:"createdAt"`
	EffectiveFrom time.Time      `bson:"effectiveFrom"`
}

type fairUseOverrideDoc struct {
	ID            string         `bson:"_id"`
	ApplicationID string         `bson:"applicationId"`
	Allowance     allowanceDoc   `bson:"allowance"`
	CostCeilings  map[string]int `bson:"costCeilings"`
	Enabled       bool           `bson:"enabled"`
	Reason        string         `bson:"reason"`
	ActorID       string         `bson:"actorId"`
	CreatedAt     time.Time      `bson:"createdAt"`
	ExpiresAt     time.Time      `bson:"expiresAt"`
	SupersedesID  string         `bson:"supersedesId,omitempty"`
}

// FairUseRepo only appends policy decisions. There are deliberately no update
// or delete methods: replacements and revocations are new immutable records.
type FairUseRepo struct {
	s                   *Store
	policies, overrides *mongo.Collection
}

func NewFairUseRepo(s *Store) *FairUseRepo {
	return &FairUseRepo{s: s, policies: s.db.Collection(ColFairUsePolicies), overrides: s.db.Collection(ColFairUseOverrides)}
}

func (r *FairUseRepo) AppendPolicy(ctx context.Context, p identity.FairUsePolicy) error {
	if err := p.Validate(); err != nil {
		return err
	}
	_, err := r.policies.InsertOne(ctx, toPolicyDoc(p))
	if err != nil {
		return fmt.Errorf("append fair-use policy: %w", err)
	}
	return nil
}

func (r *FairUseRepo) AppendOverride(ctx context.Context, o identity.FairUseOverride) error {
	if err := o.Validate(); err != nil {
		return err
	}
	_, err := r.overrides.InsertOne(ctx, toOverrideDoc(o))
	if err != nil {
		return fmt.Errorf("append fair-use override: %w", err)
	}
	return nil
}

func (r *FairUseRepo) CurrentPolicy(ctx context.Context, at time.Time) (*identity.FairUsePolicy, error) {
	var d fairUsePolicyDoc
	err := r.policies.FindOne(ctx, bson.M{"policyId": identity.DefaultFairUsePolicyID, "effectiveFrom": bson.M{"$lte": at}}, options.FindOne().SetSort(bson.D{{Key: "effectiveFrom", Value: -1}, {Key: "revision", Value: -1}})).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read current fair-use policy: %w", err)
	}
	p := fromPolicyDoc(d)
	if err := p.Validate(); err != nil {
		return nil, fmt.Errorf("decode current fair-use policy: %w", err)
	}
	return &p, nil
}

func (r *FairUseRepo) CurrentOverride(ctx context.Context, appID string, at time.Time) (*identity.FairUseOverride, error) {
	var d fairUseOverrideDoc
	err := r.overrides.FindOne(ctx, bson.M{"applicationId": appID, "createdAt": bson.M{"$lte": at}}, options.FindOne().SetSort(bson.D{{Key: "createdAt", Value: -1}, {Key: "_id", Value: -1}})).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read current fair-use override: %w", err)
	}
	o := fromOverrideDoc(d)
	if err := o.Validate(); err != nil {
		return nil, fmt.Errorf("decode current fair-use override: %w", err)
	}
	return &o, nil
}

func allowanceToDoc(a identity.Allowance) allowanceDoc {
	return allowanceDoc{a.BurstUnits, a.RefillPerSecond, int64(a.Window.Seconds())}
}
func allowanceFromDoc(a allowanceDoc) identity.Allowance {
	return identity.Allowance{BurstUnits: a.BurstUnits, RefillPerSecond: a.RefillPerSecond, Window: time.Duration(a.WindowSeconds) * time.Second}
}
func ceilingsToDoc(in map[identity.CostClass]int) map[string]int {
	out := map[string]int{}
	for k, v := range in {
		out[string(k)] = v
	}
	return out
}
func ceilingsFromDoc(in map[string]int) map[identity.CostClass]int {
	out := map[identity.CostClass]int{}
	for k, v := range in {
		out[identity.CostClass(k)] = v
	}
	return out
}
func toPolicyDoc(p identity.FairUsePolicy) fairUsePolicyDoc {
	return fairUsePolicyDoc{ID: fmt.Sprintf("%s:%020d", p.ID, p.Revision), PolicyID: p.ID, Revision: p.Revision, Anonymous: allowanceToDoc(p.Anonymous), Authenticated: allowanceToDoc(p.Authenticated), Sandbox: allowanceToDoc(p.Sandbox), CostCeilings: ceilingsToDoc(p.CostCeilings), Reason: p.Reason, ActorID: p.ActorID, CreatedAt: p.CreatedAt, EffectiveFrom: p.EffectiveFrom}
}
func fromPolicyDoc(d fairUsePolicyDoc) identity.FairUsePolicy {
	return identity.FairUsePolicy{ID: d.PolicyID, Revision: d.Revision, Anonymous: allowanceFromDoc(d.Anonymous), Authenticated: allowanceFromDoc(d.Authenticated), Sandbox: allowanceFromDoc(d.Sandbox), CostCeilings: ceilingsFromDoc(d.CostCeilings), Reason: d.Reason, ActorID: d.ActorID, CreatedAt: d.CreatedAt, EffectiveFrom: d.EffectiveFrom}
}
func toOverrideDoc(o identity.FairUseOverride) fairUseOverrideDoc {
	return fairUseOverrideDoc{ID: o.ID, ApplicationID: o.ApplicationID, Allowance: allowanceToDoc(o.Allowance), CostCeilings: ceilingsToDoc(o.CostCeilings), Enabled: o.Enabled, Reason: o.Reason, ActorID: o.ActorID, CreatedAt: o.CreatedAt, ExpiresAt: o.ExpiresAt, SupersedesID: o.SupersedesID}
}
func fromOverrideDoc(d fairUseOverrideDoc) identity.FairUseOverride {
	return identity.FairUseOverride{ID: d.ID, ApplicationID: d.ApplicationID, Allowance: allowanceFromDoc(d.Allowance), CostCeilings: ceilingsFromDoc(d.CostCeilings), Enabled: d.Enabled, Reason: d.Reason, ActorID: d.ActorID, CreatedAt: d.CreatedAt, ExpiresAt: d.ExpiresAt, SupersedesID: d.SupersedesID}
}
