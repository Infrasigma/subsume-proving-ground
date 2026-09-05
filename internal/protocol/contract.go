package protocol

import "fmt"

// ActionContract is the provider-neutral semantic intent presented for
// authorization and execution. It contains no SQL syntax, database
// identifiers, credentials, or transaction instructions.
type ActionContract struct {
	ContractVersion string `json:"contract_version"`
	ActionID string `json:"action_id"`
	ExecutionClass string `json:"execution_class"`
	Actor Actor `json:"actor"`
	Provider string `json:"provider"`
	Resource ResourceRef `json:"resource"`
	Operation string `json:"operation"`
	Arguments map[string]any `json:"arguments"`
	Precondition map[string]any `json:"precondition"`
	ExpectedEffect ExpectedEffect `json:"expected_effect"`
	MutationScope MutationScope `json:"mutation_scope"`
	ReadScope ReadScope `json:"read_scope"`
	DataEgressScope DataEgressScope `json:"data_egress_scope"`
	RecoveryMode string `json:"recovery_mode"`
	PolicyReference PolicyReference `json:"policy_reference"`
	AssuranceRequirement string `json:"assurance_requirement"`
	IssuedAt string `json:"issued_at"`
	NotBefore string `json:"not_before"`
	ExpiresAt string `json:"expires_at"`
	Nonce string `json:"nonce"`
}

type Actor struct {
	ID string `json:"id"`
	WorkloadIdentity string `json:"workload_identity"`
}

type ResourceRef struct {
	Type string `json:"type"`
	ID string `json:"id"`
}

type ExpectedEffect struct {
	Resource string `json:"resource"`
	ID string `json:"id"`
	Fields map[string]any `json:"fields"`
}

type MutationScope struct {
	MaxAffectedObjects int64 `json:"max_affected_objects"`
}

type ReadScope struct {
	MaxRecords int64 `json:"max_records"`
	MaxBytes int64 `json:"max_bytes"`
}

type DataEgressScope struct {
	Allowed bool `json:"allowed"`
}

type PolicyReference struct {
	PolicyID string `json:"policy_id"`
	Version string `json:"version"`
	Hash string `json:"hash"`
}

func (c ActionContract) ValidateForMutation() error {
	if c.ContractVersion == "" || c.ActionID == "" || c.Provider == "" {
		return fmt.Errorf("invalid action contract: missing required identity fields")
	}
	if c.ExecutionClass != "MUTATION" {
		return fmt.Errorf("invalid execution_class %q", c.ExecutionClass)
	}
	if c.Resource.Type == "" || c.Operation == "" {
		return fmt.Errorf("invalid action contract: resource type and operation are required")
	}
	if c.MutationScope.MaxAffectedObjects < 1 {
		return fmt.Errorf("mutation_scope.max_affected_objects must be positive")
	}
	if c.ExpectedEffect.Resource == "" || c.ExpectedEffect.ID == "" {
		return fmt.Errorf("expected_effect resource and id are required")
	}
	return nil
}
