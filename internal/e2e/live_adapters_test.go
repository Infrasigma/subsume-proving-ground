package e2e

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/Infrasigma/subsume-proving-ground/internal/c14n"
	"github.com/Infrasigma/subsume-proving-ground/internal/evidence"
	"github.com/Infrasigma/subsume-proving-ground/internal/protocol"
	"github.com/Infrasigma/subsume-proving-ground/internal/provider"
	"github.com/jackc/pgx/v5/pgxpool"
)

// liveVerifier is the concrete verifier used by the live-fire harness. It
// performs the same envelope parsing, canonicalization, signature validation,
// and contract validation required at the production Broker boundary.
type liveVerifier struct{ publicKey ed25519.PublicKey }

func (v liveVerifier) Verify(raw []byte) (provider.ActionContract, error) {
	var env protocol.Envelope
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&env); err != nil {
		return provider.ActionContract{}, fmt.Errorf("decode envelope: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return provider.ActionContract{}, errors.New("envelope contains trailing JSON")
	}
	if env.Type != "ActionContract" {
		return provider.ActionContract{}, fmt.Errorf("unexpected envelope type %q", env.Type)
	}
	value, err := protocol.PayloadValue(env)
	if err != nil {
		return provider.ActionContract{}, err
	}
	canonical, err := c14n.Canonicalize(value)
	if err != nil {
		return provider.ActionContract{}, fmt.Errorf("canonicalize contract: %w", err)
	}
	domain, err := protocol.DomainForType(env.Type)
	if err != nil {
		return provider.ActionContract{}, err
	}
	if err := protocol.Verify(hex.EncodeToString(v.publicKey), domain, env, canonical); err != nil {
		return provider.ActionContract{}, err
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return provider.ActionContract{}, err
	}
	var contract provider.ActionContract
	pd := json.NewDecoder(bytes.NewReader(payload))
	pd.UseNumber()
	pd.DisallowUnknownFields()
	if err := pd.Decode(&contract); err != nil {
		return provider.ActionContract{}, fmt.Errorf("decode action contract: %w", err)
	}
	if err := contract.ValidateForMutation(); err != nil {
		return provider.ActionContract{}, err
	}
	return contract, nil
}

type liveProviderResult struct {
	executionID string
	mutation    provider.MutationResult
}

// liveProvider delegates directly to the real PostgreSQL provider. It is a
// thin test adapter, not a fake provider or mocked execution implementation.
type liveProvider struct{ postgres *provider.PostgresProvider }

func (p liveProvider) Execute(ctx context.Context, contract provider.ActionContract, capability protocol.Envelope) (any, error) {
	value, err := protocol.PayloadValue(capability)
	if err != nil {
		return nil, fmt.Errorf("decode capability: %w", err)
	}
	m, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("capability payload is not an object")
	}
	executionID, ok := m["execution_id"].(string)
	if !ok || executionID == "" {
		return nil, errors.New("capability execution_id missing")
	}
	result, err := p.postgres.Execute(ctx, contract)
	if err != nil {
		return nil, err
	}
	return liveProviderResult{executionID: executionID, mutation: result}, nil
}

// liveEvidence independently observes PostgreSQL after the real provider
// transaction and compares the observed effect against the signed contract.
type liveEvidence struct{ pool *pgxpool.Pool }

func (e liveEvidence) Verify(contract provider.ActionContract, providerResult any) (any, error) {
	live, ok := providerResult.(liveProviderResult)
	if !ok {
		return nil, errors.New("unexpected provider result type")
	}
	userID, err := strconv.ParseInt(contract.Resource.ID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("resource id: %w", err)
	}
	var id, version int64
	var active bool
	if err := e.pool.QueryRow(context.Background(), `SELECT id, version, active FROM users WHERE id=$1`, userID).Scan(&id, &version, &active); err != nil {
		return nil, fmt.Errorf("observe committed postgres state: %w", err)
	}
	observed := map[string]any{"id": id, "version": version, "active": active}
	expectedHash, err := canonicalHash(contract.ExpectedEffect.Fields)
	if err != nil {
		return nil, fmt.Errorf("canonicalize expected effect: %w", err)
	}
	observedHash, err := canonicalHash(observed)
	if err != nil {
		return nil, fmt.Errorf("canonicalize observed effect: %w", err)
	}
	if expectedHash != observedHash {
		return nil, provider.ErrEffectMismatch
	}
	return evidence.NewDeactivateUser(live.executionID, live.mutation.RowsAffected, observed)
}

func canonicalHash(v any) ([32]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return [32]byte{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return [32]byte{}, err
	}
	canonical, err := c14n.Canonicalize(value)
	if err != nil {
		return [32]byte{}, err
	}
	return protocol.PayloadHash(canonical), nil
}
