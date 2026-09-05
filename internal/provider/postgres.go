package provider

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrScopeExceeded = errors.New("contract mutation scope exceeded")
	ErrEffectMismatch = errors.New("returned effect does not satisfy action contract")
	ErrCommitIndeterminate = errors.New("postgres commit outcome is indeterminate")
)

type PostgresProvider struct {
	pool *pgxpool.Pool
	registry *OperationRegistry
}

func NewPostgresProvider(pool *pgxpool.Pool, registry *OperationRegistry) (*PostgresProvider, error) {
	if pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}
	if registry == nil {
		return nil, fmt.Errorf("operation registry is required")
	}
	return &PostgresProvider{pool: pool, registry: registry}, nil
}

// Execute owns the PostgreSQL transaction boundary. A handler can only execute
// a pre-registered semantic operation. The contract can narrow the handler's
// scope but can never widen it.
func (p *PostgresProvider) Execute(ctx context.Context, contract ActionContract) (MutationResult, error) {
	if err := contract.ValidateForMutation(); err != nil {
		return MutationResult{}, err
	}

	handler, err := p.registry.Lookup(contract.Resource.Type, contract.Operation)
	if err != nil {
		return MutationResult{}, err
	}
	if contract.Provider != "postgresql" {
		return MutationResult{}, fmt.Errorf("unsupported provider %q", contract.Provider)
	}
	if contract.MutationScope.MaxAffectedObjects > handler.MaxScope() {
		return MutationResult{}, fmt.Errorf("%w: contract allows %d, handler maximum is %d", ErrScopeExceeded, contract.MutationScope.MaxAffectedObjects, handler.MaxScope())
	}

	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return MutationResult{}, fmt.Errorf("begin postgres transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	result, err := handler.Execute(ctx, tx, contract.Arguments)
	if err != nil {
		return MutationResult{}, fmt.Errorf("execute %s:%s: %w", contract.Resource.Type, contract.Operation, err)
	}

	if result.RowsAffected > handler.MaxScope() || result.RowsAffected > contract.MutationScope.MaxAffectedObjects {
		return MutationResult{}, fmt.Errorf("%w: rows_affected=%d contract_max=%d handler_max=%d", ErrScopeExceeded, result.RowsAffected, contract.MutationScope.MaxAffectedObjects, handler.MaxScope())
	}

	if err := verifyExpectedEffect(contract, result); err != nil {
		return MutationResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		// At this point rollback cannot establish whether PostgreSQL committed.
		// The caller must enter AACR's INDETERMINATE/reconciliation path and use
		// the durable provider idempotency identity to determine the outcome.
		return MutationResult{}, fmt.Errorf("%w: %v", ErrCommitIndeterminate, err)
	}
	return result, nil
}

func verifyExpectedEffect(contract ActionContract, result MutationResult) error {
	if result.RowsAffected == 0 {
		return fmt.Errorf("%w: mutation affected zero rows", ErrEffectMismatch)
	}
	if len(result.ReturnedData) != int(result.RowsAffected) {
		return fmt.Errorf("%w: RETURNING rows=%d does not match rows_affected=%d", ErrEffectMismatch, len(result.ReturnedData), result.RowsAffected)
	}
	if len(result.ReturnedData) != 1 {
		return fmt.Errorf("%w: deactivate_user reference path requires exactly one returned row", ErrEffectMismatch)
	}
	row := result.ReturnedData[0]
	if contract.ExpectedEffect.Resource != contract.Resource.Type || contract.ExpectedEffect.ID != contract.Resource.ID {
		return fmt.Errorf("%w: expected effect resource identity does not match contract", ErrEffectMismatch)
	}

	for field, expected := range contract.ExpectedEffect.Fields {
		actual, ok := row[field]
		if !ok {
			return fmt.Errorf("%w: returned field %q is missing", ErrEffectMismatch, field)
		}
		if !semanticValueEqual(expected, actual) {
			return fmt.Errorf("%w: field %q expected %v, got %v", ErrEffectMismatch, field, expected, actual)
		}
	}
	return nil
}

func semanticValueEqual(expected, actual any) bool {
	if e, ok := integerValue(expected); ok {
		a, ok := integerValue(actual)
		return ok && e == a
	}
	return fmt.Sprint(expected) == fmt.Sprint(actual)
}

func integerValue(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int8:
		return int64(n), true
	case int16:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case uint:
		if uint64(n) > uint64(^uint64(0)>>1) { return 0, false }
		return int64(n), true
	case uint8:
		return int64(n), true
	case uint16:
		return int64(n), true
	case uint32:
		return int64(n), true
	case uint64:
		if n > uint64(^uint64(0)>>1) { return 0, false }
		return int64(n), true
	case float64:
		if n != float64(int64(n)) { return 0, false }
		return int64(n), true
	case string:
		i, err := strconv.ParseInt(n, 10, 64)
		return i, err == nil
	default:
		return 0, false
	}
}
