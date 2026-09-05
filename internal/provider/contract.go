package provider

import "github.com/Infrasigma/subsume-proving-ground/internal/protocol"

// Provider keeps these aliases for source compatibility while the canonical
// semantic contract types live in the protocol leaf package. This prevents
// protocol from depending on operational provider code.
type ActionContract = protocol.ActionContract
type Actor = protocol.Actor
type ResourceRef = protocol.ResourceRef
type ExpectedEffect = protocol.ExpectedEffect
type MutationScope = protocol.MutationScope
type ReadScope = protocol.ReadScope
type DataEgressScope = protocol.DataEgressScope
type PolicyReference = protocol.PolicyReference
