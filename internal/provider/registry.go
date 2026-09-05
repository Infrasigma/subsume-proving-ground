package provider

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type OperationHandler interface {
	Name() string
	Resource() string
	MaxScope() int64
	Execute(ctx context.Context, tx pgx.Tx, args map[string]any) (MutationResult, error)
}

// ContractValidator is an optional operation-specific semantic validation hook.
// It lets a registered handler bind argument identity to the ActionContract
// resource without exposing SQL or executable syntax to the caller.
type ContractValidator interface { ValidateContract(contract ActionContract) error }

type MutationResult struct { RowsAffected int64; ReturnedData []map[string]any }

type OperationRegistry struct { handlers map[string]OperationHandler }

func NewOperationRegistry(handlers ...OperationHandler) (*OperationRegistry, error) {
	r := &OperationRegistry{handlers: make(map[string]OperationHandler, len(handlers))}
	for _, h := range handlers {
		if h == nil { return nil, fmt.Errorf("nil operation handler") }
		if h.Name() == "" || h.Resource() == "" { return nil, fmt.Errorf("operation handler must declare name and resource") }
		if h.MaxScope() < 1 { return nil, fmt.Errorf("operation %s has invalid maximum scope", h.Name()) }
		key := operationKey(h.Resource(), h.Name())
		if _, exists := r.handlers[key]; exists { return nil, fmt.Errorf("duplicate operation handler %q", key) }
		r.handlers[key] = h
	}
	return r, nil
}

func (r *OperationRegistry) Lookup(resource, operation string) (OperationHandler, error) {
	if r == nil { return nil, fmt.Errorf("operation registry is nil") }
	h, ok := r.handlers[operationKey(resource, operation)]
	if !ok { return nil, fmt.Errorf("operation not registered: %s:%s", resource, operation) }
	return h, nil
}

func operationKey(resource, operation string) string { return resource + ":" + operation }
