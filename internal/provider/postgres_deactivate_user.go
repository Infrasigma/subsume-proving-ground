package provider

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type PostgresDeactivateUserHandler struct{}

func (PostgresDeactivateUserHandler) Name() string { return "deactivate_user" }
func (PostgresDeactivateUserHandler) Resource() string { return "users" }
func (PostgresDeactivateUserHandler) MaxScope() int64 { return 1 }

func (PostgresDeactivateUserHandler) ValidateContract(contract ActionContract) error {
	if contract.Resource.Type != "users" || contract.Operation != "deactivate_user" { return fmt.Errorf("invalid deactivate_user contract binding") }
	userID, ok := integerValue(contract.Arguments["user_id"])
	if !ok || userID <= 0 { return fmt.Errorf("user_id must be a positive integer") }
	if contract.Resource.ID != fmt.Sprintf("%d", userID) { return fmt.Errorf("user_id %d does not match contract resource id %q", userID, contract.Resource.ID) }
	if contract.ExpectedEffect.Resource != "users" || contract.ExpectedEffect.ID != contract.Resource.ID { return fmt.Errorf("expected effect identity must match users resource") }
	return nil
}

func (PostgresDeactivateUserHandler) Execute(ctx context.Context, tx pgx.Tx, args map[string]any) (MutationResult, error) {
	userID, ok := integerValue(args["user_id"]); if !ok || userID <= 0 { return MutationResult{}, fmt.Errorf("user_id must be a positive integer") }
	expectedVersion, ok := integerValue(args["expected_version"]); if !ok || expectedVersion < 0 { return MutationResult{}, fmt.Errorf("expected_version must be a non-negative integer") }
	const query = `UPDATE users
SET active = false,
    version = version + 1
WHERE id = $1
  AND version = $2
  AND active = true
RETURNING id, version, active`
	rows, err := tx.Query(ctx, query, userID, expectedVersion); if err != nil { return MutationResult{}, fmt.Errorf("execute deactivate_user mutation: %w", err) }
	defer rows.Close()
	result := MutationResult{}
	for rows.Next() {
		var id, version int64; var active bool
		if err := rows.Scan(&id, &version, &active); err != nil { return MutationResult{}, fmt.Errorf("scan deactivate_user RETURNING row: %w", err) }
		result.ReturnedData = append(result.ReturnedData, map[string]any{"id": id, "version": version, "active": active})
	}
	if err := rows.Err(); err != nil { return MutationResult{}, fmt.Errorf("read deactivate_user RETURNING rows: %w", err) }
	result.RowsAffected = rows.CommandTag().RowsAffected()
	return result, nil
}
