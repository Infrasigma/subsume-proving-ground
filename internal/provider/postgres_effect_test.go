package provider

import "testing"

func TestVerifyExpectedEffectIsMapOrderIndependent(t *testing.T) {
	contract := ActionContract{
		ContractVersion: "1.0", ActionID: "act-1", ExecutionClass: "MUTATION", Provider: "postgresql",
		Resource: ResourceRef{Type: "users", ID: "1842"}, Operation: "deactivate_user",
		MutationScope: MutationScope{MaxAffectedObjects: 1},
		ExpectedEffect: ExpectedEffect{Resource: "users", ID: "1842", Fields: map[string]any{"id": int64(1842), "version": int64(43), "active": false}},
	}
	result := MutationResult{RowsAffected: 1, ReturnedData: []map[string]any{{"active": false, "id": int64(1842), "version": int64(43)}}}
	if err := verifyExpectedEffect(contract, result); err != nil { t.Fatalf("map-order-independent effect should verify: %v", err) }
}

func TestVerifyExpectedEffectRejectsExtraReturnedField(t *testing.T) {
	contract := ActionContract{
		ContractVersion: "1.0", ActionID: "act-1", ExecutionClass: "MUTATION", Provider: "postgresql",
		Resource: ResourceRef{Type: "users", ID: "1842"}, Operation: "deactivate_user",
		MutationScope: MutationScope{MaxAffectedObjects: 1},
		ExpectedEffect: ExpectedEffect{Resource: "users", ID: "1842", Fields: map[string]any{"id": int64(1842), "version": int64(43), "active": false}},
	}
	result := MutationResult{RowsAffected: 1, ReturnedData: []map[string]any{{"id": int64(1842), "version": int64(43), "active": false, "unexpected": "x"}}}
	if err := verifyExpectedEffect(contract, result); err == nil { t.Fatal("expected extra returned field to fail exact equivalence") }
}

func TestDeactivateUserContractBindsArgumentToResource(t *testing.T) {
	h := PostgresDeactivateUserHandler{}
	contract := ActionContract{
		ContractVersion: "1.0", ActionID: "act-1", ExecutionClass: "MUTATION", Provider: "postgresql",
		Resource: ResourceRef{Type: "users", ID: "1842"}, Operation: "deactivate_user",
		Arguments: map[string]any{"user_id": int64(1843), "expected_version": int64(42)},
		MutationScope: MutationScope{MaxAffectedObjects: 1},
		ExpectedEffect: ExpectedEffect{Resource: "users", ID: "1842", Fields: map[string]any{"id": int64(1842), "version": int64(43), "active": false}},
	}
	if err := h.ValidateContract(contract); err == nil { t.Fatal("expected mismatched user_id/resource id to fail") }
}

func TestExecuteRejectsScopeBeforeOpeningPool(t *testing.T) {
	registry, err := NewOperationRegistry(PostgresDeactivateUserHandler{})
	if err != nil { t.Fatal(err) }
	p := &PostgresProvider{pool: nil, registry: registry}
	contract := ActionContract{
		ContractVersion: "1.0", ActionID: "act-1", ExecutionClass: "MUTATION", Provider: "postgresql",
		Resource: ResourceRef{Type: "users", ID: "1842"}, Operation: "deactivate_user",
		Arguments: map[string]any{"user_id": int64(1842), "expected_version": int64(42)},
		MutationScope: MutationScope{MaxAffectedObjects: 2},
		ExpectedEffect: ExpectedEffect{Resource: "users", ID: "1842", Fields: map[string]any{"id": int64(1842), "version": int64(43), "active": false}},
	}
	if err := func() error { _, err := p.Execute(t.Context(), contract); return err }(); err == nil || err.Error() == "" { t.Fatal("expected scope rejection") }
}
