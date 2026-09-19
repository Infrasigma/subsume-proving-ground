package ace

import "errors"

// EvaluateLayerECandidate runs the same Layer E predicates in order while
// emitting one forensic event at the first rejection. It is deliberately
// side-effect free with respect to capability installation.
func (a AutonomousAcquirer) EvaluateLayerECandidate(spec CapabilitySpecification, tests []ProgramTestCase, c ArchitectureCandidate) (ModificationProposal, VerificationResult, error) {
	sandbox := ExecutableSandbox{Cases: map[string][]ProgramTestCase{c.ID: tests}}
	p, err := a.Builder.Build(c, spec)
	if err != nil {
		a.Diagnostics.Record("E", c.ID, "architecture-candidate", c, DiagnosticFormalValidity, "builder.Build(candidate) succeeds", false, err.Error())
		return ModificationProposal{}, VerificationResult{}, err
	}
	rr, err := sandbox.Validate(p)
	if err != nil || !rr.Passed {
		detail := "sandbox validation rejected candidate"
		if err != nil { detail = err.Error() }
		a.Diagnostics.Record("E", c.ID, "architecture-candidate", c, DiagnosticEnvironmental, "sandbox validation passes", false, detail)
		if err == nil { err = errors.New("sandbox rejected candidate") }
		return ModificationProposal{}, VerificationResult{}, err
	}
	v, err := independentAffineVerify(spec, tests, p.Artifact)
	if err != nil || v.Status != "verified" {
		detail := "independent affine verifier rejected candidate"
		if err != nil { detail = err.Error() }
		a.Diagnostics.Record("E", c.ID, "architecture-candidate", c, DiagnosticVerification, "independentAffineVerify(candidate) == verified", false, detail)
		if err == nil { err = errors.New("independent verifier rejected candidate") }
		return ModificationProposal{}, v, err
	}
	a.Diagnostics.Record("E", c.ID, "architecture-candidate", c, DiagnosticAccepted, "builder + sandbox + independent verifier", true, "candidate survived Layer E")
	return p, v, nil
}
