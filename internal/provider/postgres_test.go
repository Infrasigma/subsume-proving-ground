package provider

import (
	"testing"
)

func TestOperationRegistryRejectsDuplicateSemanticOperation(t *testing.T) {
	_, err := NewOperationRegistry(PostgresDeactivateUserHandler{}, PostgresDeactivateUserHandler{})
	if err == nil {
		t.Fatal("expected duplicate operation handler to be rejected")
	}
}

func TestOperationRegistryLookup(t *testing.T) {
	registry, err := NewOperationRegistry(PostgresDeactivateUserHandler{})
	if err != nil {
		t.Fatal(err)
	}
	h, err := registry.Lookup("users", "deactivate_user")
	if err != nil {
		t.Fatal(err)
	}
	if h.MaxScope() != 1 {
		t.Fatalf("handler max scope = %d, want 1", h.MaxScope())
	}
	if _, err := registry.Lookup("users", "delete_user"); err == nil {
		t.Fatal("expected unregistered operation to fail")
	}
}

func TestVerifyExpectedEffect(t *testing.T) {
	contract := ActionContract{
		ContractVersion: "1.0",
		ActionID: "act-1",
		ExecutionClass: "MUTATION",
		Provider: "postgresql",
		Resource: ResourceRef{Type: "users", ID: "1842"},
		Operation: "deactivate_user",
		MutationScope: MutationScope{MaxAffectedObjects: 1},
		ExpectedEffect: ExpectedEffect{
			Resource: "users",
			ID: "1842",
			Fields: map[string]any{"id": int64(1842), "version": int64(43), "active": false},
		},
	}
	result := MutationResult{
		RowsAffected: 1,
		ReturnedData: []map[string]any{{"id": int64(1842), "version": int64(43), "active": false}},
	}
	if err := verifyExpectedEffect(contract, result); err != nil {
		t.Fatalf("expected effect should verify: %v", err)
	}
}

func TestVerifyExpectedEffectRejectsMismatch(t *testing.T) {
	contract := ActionContract{
		ContractVersion: "1.0",
		ActionID: "act-1",
		ExecutionClass: "MUTATION",
		Provider: "postgresql",
		Resource: ResourceRef{Type: "users", ID: "1842"},
		Operation: "deactivate_user",
		MutationScope: MutationScope{MaxAffectedObjects: 1},
		ExpectedEffect: ExpectedEffect{
			Resource: "users",
			ID: "1842",
			Fields: map[string]any{"active": false, "version": int64(43)},
		},
	}
	result := MutationResult{
		RowsAffected: 1,
		ReturnedData: []map[string]any{{"id": int64(1842), "version": int64(44), "active": false}},
	}
	if err := verifyExpectedEffect(contract, result); err == nil {
		t.Fatal("expected effect mismatch to fail closed")
	}
}

func TestContractCannotWidenHandlerScope(t *testing.T) {
	registry, err := NewOperationRegistry(PostgresDeactivateUserHandler{})
	if err != nil {
		t.Fatal(err)
	}
	provider := &PostgresProvider{registry: registry}
	contract := ActionContract{
		ContractVersion: "1.0",
		ActionID: "act-1",
		ExecutionClass: "MUTATION",
		Provider: "postgresql",
		Resource: ResourceRef{Type: "users", ID: "1842"},
		Operation: "deactivate_user",
		MutationScope: MutationScope{MaxAffectedObjects: 2},
		ExpectedEffect: ExpectedEffect{Resource: "users", ID: "1842"},
	}
	if _, err := provider.lookupAndValidateHandler(contract); err == nil {
		t.Fatal("expected contract scope wider than handler scope to fail")
	}
}
