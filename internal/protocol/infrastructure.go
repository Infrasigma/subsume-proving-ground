package protocol

import (
	"fmt"
	"time"
)

type InfrastructureContract struct {
	ContractVersion   string                 `json:"contract_version"`
	ContractID        string                 `json:"contract_id"`
	Operation         string                 `json:"operation"`
	Provider          string                 `json:"provider"`
	ResourceType      string                 `json:"resource_type"`
	MaxInstances      int                    `json:"max_instances"`
	MaxRuntimeSeconds int64                  `json:"max_runtime_seconds"`
	MaxCostCPUTimeMS  int64                  `json:"max_cost_cpu_time_ms"`
	MaxMemoryBytes    int64                  `json:"max_memory_bytes"`
	ExpectedEffect    InfrastructureEffect   `json:"expected_effect"`
	PolicyHash        string                 `json:"policy_hash"`
	IssuedAt          time.Time              `json:"issued_at"`
	ExpiresAt         time.Time              `json:"expires_at"`
	Nonce             string                 `json:"nonce"`
}

type InfrastructureEffect struct {
	ResourceType string         `json:"resource_type"`
	ResourceID   string         `json:"resource_id"`
	State        string         `json:"state"`
	Fields       map[string]any `json:"fields,omitempty"`
}

func (c InfrastructureContract) Validate() error {
	if c.ContractVersion == "" || c.ContractID == "" || c.Provider == "" || c.ResourceType == "" {
		return fmt.Errorf("infrastructure contract identity is incomplete")
	}
	if c.Operation != "provision" && c.Operation != "reclaim" {
		return fmt.Errorf("unsupported infrastructure operation %q", c.Operation)
	}
	if c.MaxInstances < 1 || c.MaxInstances > 64 {
		return fmt.Errorf("max_instances must be in 1..64")
	}
	if c.MaxRuntimeSeconds < 1 || c.MaxRuntimeSeconds > 86400 {
		return fmt.Errorf("max_runtime_seconds must be in 1..86400")
	}
	if c.MaxCostCPUTimeMS < 1 || c.MaxMemoryBytes < 1 {
		return fmt.Errorf("infrastructure cost bounds must be positive")
	}
	if c.ExpectedEffect.ResourceType == "" || c.ExpectedEffect.ResourceID == "" || c.ExpectedEffect.State == "" {
		return fmt.Errorf("expected infrastructure effect is incomplete")
	}
	if c.PolicyHash == "" || c.Nonce == "" {
		return fmt.Errorf("infrastructure policy binding is incomplete")
	}
	if c.IssuedAt.IsZero() || c.ExpiresAt.IsZero() || !c.ExpiresAt.After(c.IssuedAt) {
		return fmt.Errorf("infrastructure contract validity window is invalid")
	}
	if c.ExpectedEffect.ResourceType != c.ResourceType {
		return fmt.Errorf("expected resource type %q does not match %q", c.ExpectedEffect.ResourceType, c.ResourceType)
	}
	return nil
}

func (c InfrastructureContract) ReclamationContract() (InfrastructureContract, error) {
	if err := c.Validate(); err != nil {
		return InfrastructureContract{}, err
	}
	if c.Operation != "provision" {
		return InfrastructureContract{}, fmt.Errorf("only provision contracts can derive reclamation contracts")
	}
	r := c
	r.ContractID = c.ContractID + "-reclaim"
	r.Operation = "reclaim"
	r.ExpectedEffect = InfrastructureEffect{
		ResourceType: c.ResourceType,
		ResourceID:   c.ExpectedEffect.ResourceID,
		State:        "absent",
		Fields: map[string]any{
			"parent_contract_id": c.ContractID,
		},
	}
	r.Nonce = c.Nonce + "-reclaim"
	return r, nil
}
