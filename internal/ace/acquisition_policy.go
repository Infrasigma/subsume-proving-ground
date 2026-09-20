package ace

import (
	"errors"
	"fmt"
	"sort"
)

type RepresentationGeneratorPrimitive struct {
	ID            string
	Block         RepresentationBlock
	SemanticDelta []string
	BaseWeight    float64
}

type GeneratorPolicyBinding struct {
	FailureTopology string
	PrimitiveID     string
	SearchWeight    float64
}

type GeneratorSpaceMutation struct {
	PrimitiveID     string
	FailureTopology string
	SemanticDelta   []string
	SearchWeight    float64
	Evidence        []string
}

type AcquisitionPolicy struct {
	Primitives []RepresentationGeneratorPrimitive
	Bindings   []GeneratorPolicyBinding
	Mutations  []GeneratorSpaceMutation
}

func FailureTopologySignature(t FailureTelemetry) string {
	return Hash([]any{
		"failure-topology-v1",
		t.CurrentRepresentation,
		t.SearchExhausted,
		t.AllCandidateFamiliesExhausted,
		t.SearchOrdersTested,
		t.CandidateReachedVerifier,
		t.IndependentVerifierRejected,
	})
}

// MutateGeneratorSpace admits a representation primitive only as a complete
// semantic artifact. The mutation is deterministic, idempotent, and never
// installs an unnamed or empty block.
func (p *AcquisitionPolicy) MutateGeneratorSpace(newPrimitive RepresentationBlock, semanticDelta []string) error {
	if p == nil {
		return errors.New("acquisition policy unavailable")
	}
	if newPrimitive.ID == "" || newPrimitive.Name == "" || newPrimitive.Op == "" || newPrimitive.Input == "" {
		return errors.New("generator primitive is incomplete")
	}
	if len(semanticDelta) == 0 {
		return errors.New("generator primitive requires semantic delta evidence")
	}
	for _, existing := range p.Primitives {
		if existing.ID == newPrimitive.ID {
			return nil
		}
	}
	p.Primitives = append(p.Primitives, RepresentationGeneratorPrimitive{
		ID:            newPrimitive.ID,
		Block:         newPrimitive,
		SemanticDelta: append([]string(nil), semanticDelta...),
		BaseWeight:    1.0,
	})
	sort.SliceStable(p.Primitives, func(i, j int) bool {
		return p.Primitives[i].ID < p.Primitives[j].ID
	})
	return nil
}

func (p *AcquisitionPolicy) BindPrimitiveToFailureTopology(signature, primitiveID string, discountedWeight float64) error {
	if p == nil {
		return errors.New("acquisition policy unavailable")
	}
	if signature == "" || primitiveID == "" {
		return errors.New("failure topology and primitive ID are required")
	}
	if discountedWeight <= 0 || discountedWeight >= 1 {
		return errors.New("discounted search weight must be in (0,1)")
	}
	found := false
	for _, primitive := range p.Primitives {
		if primitive.ID == primitiveID {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("cannot bind unknown primitive %q", primitiveID)
	}
	for i := range p.Bindings {
		if p.Bindings[i].FailureTopology == signature && p.Bindings[i].PrimitiveID == primitiveID {
			p.Bindings[i].SearchWeight = discountedWeight
			return nil
		}
	}
	p.Bindings = append(p.Bindings, GeneratorPolicyBinding{
		FailureTopology: signature,
		PrimitiveID:     primitiveID,
		SearchWeight:    discountedWeight,
	})
	return nil
}

func (p AcquisitionPolicy) Weight(signature string, primitiveID string) float64 {
	for _, binding := range p.Bindings {
		if binding.FailureTopology == signature && binding.PrimitiveID == primitiveID {
			return binding.SearchWeight
		}
	}
	return 1.0
}

// ActivePrimitiveForFailure returns the lowest-cost generator primitive for a
// known failure topology. At most one primitive is activated per acquisition
// attempt to avoid combinatorial feature expansion.
func (p AcquisitionPolicy) ActivePrimitiveForFailure(signature string) (RepresentationGeneratorPrimitive, bool) {
	var best RepresentationGeneratorPrimitive
	bestWeight := 0.0
	found := false
	for _, primitive := range p.Primitives {
		weight := p.Weight(signature, primitive.ID)
		if !found || weight < bestWeight || (weight == bestWeight && primitive.ID < best.ID) {
			best = primitive
			bestWeight = weight
			found = true
		}
	}
	if !found {
		return RepresentationGeneratorPrimitive{}, false
	}
	return best, true
}

func PrepareCapabilityWithAcquisitionPolicy(
	spec CapabilitySpecification,
	hidden []ProgramTestCase,
	failure FailureTelemetry,
	policy AcquisitionPolicy,
) (CapabilitySpecification, []ProgramTestCase, RepresentationGeneratorPrimitive, error) {
	signature := FailureTopologySignature(failure)
	primitive, ok := policy.ActivePrimitiveForFailure(signature)
	if !ok {
		return spec, hidden, RepresentationGeneratorPrimitive{}, errors.New("no learned representation primitive for failure topology")
	}
	enriched, enrichedHidden, err := augmentWithRepresentationBlock(spec, hidden, primitive.Block)
	if err != nil {
		return CapabilitySpecification{}, nil, RepresentationGeneratorPrimitive{}, err
	}
	enriched.Provenance = Prov(
		"acquisition-policy",
		spec.ID,
		"topology-conditioned-generator-expansion",
		map[string]any{"failure_topology": signature, "primitive": primitive.ID, "weight": policy.Weight(signature, primitive.ID)},
	)
	return enriched, enrichedHidden, primitive, nil
}

// PromoteCounterfactualRepresentation admits the unique independently
// verified representation survivor. It refuses promotion when the experiment
// was inconclusive or when the runner did not return the exact semantic block.
func PromoteCounterfactualRepresentation(
	policy *AcquisitionPolicy,
	failure FailureTelemetry,
	result CounterfactualDiagnosis,
) (GeneratorSpaceMutation, error) {
	if policy == nil {
		return GeneratorSpaceMutation{}, errors.New("acquisition policy unavailable")
	}
	if !result.Discriminated || result.Diagnosis.Class != BottleneckRepresentation {
		return GeneratorSpaceMutation{}, errors.New("representation promotion requires a uniquely discriminated representation diagnosis")
	}

	var winner *CounterfactualTrial
	for i := range result.Trials {
		trial := &result.Trials[i]
		if trial.Hypothesis != HypothesisRepresentation {
			continue
		}
		if trial.Result.Solved && trial.Result.IndependentlyVerified && trial.Result.PromotedRepresentation != nil {
			if winner != nil {
				return GeneratorSpaceMutation{}, errors.New("multiple independently verified representation winners")
			}
			winner = trial
		}
	}
	if winner == nil {
		return GeneratorSpaceMutation{}, errors.New("representation diagnosis lacks a promotable verified primitive")
	}

	block := *winner.Result.PromotedRepresentation
	signature := FailureTopologySignature(failure)
	semanticDelta := append([]string(nil), winner.Result.Evidence...)
	if err := policy.MutateGeneratorSpace(block, semanticDelta); err != nil {
		return GeneratorSpaceMutation{}, err
	}
	const discountedWeight = 0.5
	if err := policy.BindPrimitiveToFailureTopology(signature, block.ID, discountedWeight); err != nil {
		return GeneratorSpaceMutation{}, err
	}

	mutation := GeneratorSpaceMutation{
		PrimitiveID:     block.ID,
		FailureTopology: signature,
		SemanticDelta:   semanticDelta,
		SearchWeight:    discountedWeight,
		Evidence: []string{
			"unique counterfactual representation survivor",
			"hidden cases independently verified",
			"primitive admitted to persistent generator space",
			"topology-conditioned search weight installed",
		},
	}
	policy.Mutations = append(policy.Mutations, mutation)
	return mutation, nil
}
