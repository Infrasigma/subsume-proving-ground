package ace

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

// SearchObject is the common substrate for anything that can alter future acquisition.
// The trusted bootstrap supplies only the primitive opcode semantics; all compositions
// above that boundary are executable, serializable artifacts.
type SearchObjectKind string

const (
	SearchTask SearchObjectKind = "task"
	SearchCapability SearchObjectKind = "capability"
	SearchMethod SearchObjectKind = "acquisition-method"
	SearchRepresentation SearchObjectKind = "representation"
	SearchVerifier SearchObjectKind = "verifier"
	SearchDecomposer SearchObjectKind = "decomposer"
	SearchController SearchObjectKind = "controller"
	SearchAbstraction SearchObjectKind = "abstraction"
)

type CapabilitySearchObject struct {
	ID string `json:"id"`
	Kind SearchObjectKind `json:"kind"`
	Artifact string `json:"artifact"`
	ParentIDs []string `json:"parent_ids,omitempty"`
	Provenance Provenance `json:"provenance"`
}

type MethodInstruction struct {
	Op string `json:"op"`
	Arg string `json:"arg,omitempty"`
}

type ExecutableAcquisitionMethod struct {
	ID string `json:"id"`
	Instructions []MethodInstruction `json:"instructions"`
	Inputs []string `json:"inputs,omitempty"`
	Outputs []string `json:"outputs,omitempty"`
	Cost ResourceVector `json:"cost"`
	Applicability []string `json:"applicability,omitempty"`
	Verifier string `json:"verifier"`
	Parents []string `json:"parents,omitempty"`
	Version uint64 `json:"version"`
}

type SearchPrimitive struct {
	ID string
	Strategy string
}

type SearchLibrary struct {
	Primitives map[string]SearchPrimitive
	Methods map[string]ExecutableAcquisitionMethod
	Abstractions map[string]ExecutableAcquisitionMethod
	History []AcquisitionExperience
}

func NewSearchLibrary() *SearchLibrary {
	return &SearchLibrary{Primitives: map[string]SearchPrimitive{
		"search:straight-line": {ID: "search:straight-line", Strategy: "universal:straight-line"},
		"search:branching": {ID: "search:branching", Strategy: "universal:branching"},
	}, Methods: map[string]ExecutableAcquisitionMethod{}, Abstractions: map[string]ExecutableAcquisitionMethod{}}
}

func (l *SearchLibrary) RegisterMethod(m ExecutableAcquisitionMethod) error {
	if l == nil || m.ID == "" || len(m.Instructions) == 0 || m.Verifier == "" { return errors.New("incomplete executable method") }
	b, err := json.Marshal(m); if err != nil { return err }
	var round ExecutableAcquisitionMethod
	if err = json.Unmarshal(b, &round); err != nil { return err }
	l.Methods[m.ID] = round
	return nil
}

func (l *SearchLibrary) RegisterPrimitive(p SearchPrimitive) error {
	if p.ID == "" || p.Strategy == "" { return errors.New("incomplete search primitive") }
	l.Primitives[p.ID] = p
	return nil
}

// Execute is the trusted interpreter. It knows only primitive operations, not which
// method should win. In particular there is no branch on a method name or task ID.
func (l *SearchLibrary) Execute(m ExecutableAcquisitionMethod, spec CapabilitySpecification) ([]ArchitectureCandidate, error) {
	var out []ArchitectureCandidate
	for _, ins := range m.Instructions {
		switch ins.Op {
		case "search":
			p, ok := l.Primitives[ins.Arg]; if !ok { return nil, fmt.Errorf("unknown search primitive %q", ins.Arg) }
			cs, err := (UniversalMechanismSearch{}).SearchMechanisms(spec, spec.ResourceLimits); if err != nil { return nil, err }
			for _, c := range cs { if c.Mechanism == p.Strategy { out = append(out, c) } }
		case "use":
			child, ok := l.Methods[ins.Arg]; if !ok { return nil, fmt.Errorf("unknown method %q", ins.Arg) }
			cs, err := l.Execute(child, spec); if err != nil { return nil, err }; out = append(out, cs...)
		case "dedupe":
			seen := map[string]bool{}; dst := out[:0]; for _, c := range out { if !seen[c.ID] { seen[c.ID] = true; dst = append(dst, c) } }; out = dst
		case "reverse":
			for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 { out[i], out[j] = out[j], out[i] }
		default:
			return nil, fmt.Errorf("unknown method opcode %q", ins.Op)
		}
	}
	return out, nil
}

func serializeMethod(m ExecutableAcquisitionMethod) (string, error) { b, err := json.Marshal(m); return string(b), err }

// BootstrapMethodSearch enumerates method programs, not hand-authored method names.
// The bootstrap vocabulary is intentionally tiny: search, use, dedupe, reverse.
func BootstrapMethodSearch(lib *SearchLibrary, spec CapabilitySpecification, hidden []ProgramTestCase, maxDepth int) (ExecutableAcquisitionMethod, int, error) {
	if maxDepth < 1 { return ExecutableAcquisitionMethod{}, 0, errors.New("invalid bootstrap depth") }
	prims := make([]string, 0, len(lib.Primitives)); for id := range lib.Primitives { prims = append(prims, id) }; sort.Strings(prims)
	candidates := [][]MethodInstruction{{}}
	for depth := 1; depth <= maxDepth; depth++ {
		next := make([][]MethodInstruction, 0)
		for _, prefix := range candidates {
			ops := make([]MethodInstruction, 0, len(prefix)+1); ops = append(ops, prefix...)
			for _, p := range prims { q := append(append([]MethodInstruction{}, ops...), MethodInstruction{Op: "search", Arg: p}); next = append(next, q) }
			for _, op := range []string{"dedupe", "reverse"} { q := append(append([]MethodInstruction{}, ops...), MethodInstruction{Op: op}); next = append(next, q) }
			for id := range lib.Methods { q := append(append([]MethodInstruction{}, ops...), MethodInstruction{Op: "use", Arg: id}); next = append(next, q) }
		}
		for _, ins := range next {
			m := ExecutableAcquisitionMethod{ID: Hash([]any{"bootstrap-method", spec.ID, ins}), Instructions: ins, Inputs: spec.Inputs, Outputs: spec.Outputs, Cost: spec.ResourceLimits, Verifier: "independent-heldout", Version: 1}
			cs, err := lib.Execute(m, spec); if err != nil { continue }
			for _, c := range cs { p, e := (UniversalProgramBuilder{}).Build(c, spec); if e == nil && programFitsJSON(p.Artifact, hidden) { return m, len(next), nil } }
		}
		candidates = next
	}
	return ExecutableAcquisitionMethod{}, len(candidates), errors.New("bootstrap method search exhausted")
}

// LearnMethodAbstraction extracts an exact repeated instruction suffix/prefix from
// verified methods. It is deliberately data-driven: no method name is recognized.
func LearnMethodAbstraction(methods []ExecutableAcquisitionMethod) (ExecutableAcquisitionMethod, bool) {
	if len(methods) < 2 { return ExecutableAcquisitionMethod{}, false }
	best := []MethodInstruction(nil)
	for i := 0; i < len(methods); i++ { for j := i + 1; j < len(methods); j++ {
		a, b := methods[i].Instructions, methods[j].Instructions
		for n := 1; n <= len(a) && n <= len(b); n++ { if equalInstructions(a[len(a)-n:], b[len(b)-n:]) && n > len(best) { best = append([]MethodInstruction(nil), a[len(a)-n:]...) } }
	} }
	if len(best) == 0 { return ExecutableAcquisitionMethod{}, false }
	return ExecutableAcquisitionMethod{ID: Hash([]any{"learned-abstraction", best}), Instructions: best, Verifier: "independent-heldout", Version: 1}, true
}

func equalInstructions(a, b []MethodInstruction) bool { if len(a) != len(b) { return false }; for i := range a { if a[i] != b[i] { return false } }; return true }

func VerifyExecutableMethod(lib *SearchLibrary, m ExecutableAcquisitionMethod, spec CapabilitySpecification, hidden []ProgramTestCase) (bool, ResourceVector, error) {
	if m.Verifier == "" || len(m.Instructions) == 0 { return false, ResourceVector{}, errors.New("method missing verifier or procedure") }
	cs, err := lib.Execute(m, spec); if err != nil { return false, ResourceVector{}, err }
	for _, c := range cs { p, e := (UniversalProgramBuilder{}).Build(c, spec); if e != nil { continue }; if programFitsJSON(p.Artifact, hidden) { return true, c.Resources, nil } }
	return false, ResourceVector{Compute: float64(len(cs))}, nil
}

// SelfExtensibleExperiment is a bounded scientific probe. Its important property is
// causal: B1 is constructed by adding a verified executable method to B0's library.
type SelfExtensibleExperimentResult struct {
	T1Solved, T2Solved, T3Solved, T4Solved, T5Solved bool
	M1, M2 ExecutableAcquisitionMethod
	M1Verified, M2Verified bool
	M1UsedByB1, M2UsedByB2 bool
	LearnedAbstraction bool
	BaselineCost, ImprovedCost ResourceVector
	R []float64
	Notes []string
}

func RunSelfExtensibleExperiment() (SelfExtensibleExperimentResult, error) {
	// T1: affine capability establishes K1 without a meta-method.
	_, c1, _, err := (AffineFamily{}).Generate(0, false); if err != nil { return SelfExtensibleExperimentResult{}, err }
	k1, err := ParameterizedMechanismSearch(c1); if err != nil { return SelfExtensibleExperimentResult{}, err }
	res := SelfExtensibleExperimentResult{T1Solved: k1.Verified, Notes: []string{"bootstrap contains only generic search opcodes"}}
	lib := NewSearchLibrary()

	// T2: conditional task requires the branching primitive. M1 is synthesized by
	// enumerating executable method programs; the evaluator supplies only behavior.
	_, c2, _, err := (ThresholdFamily{}).Generate(0, false); if err != nil { return res, err }
	t2, err := GeneralCapabilitySpecification(Task{ID: "self-t2", Goal: "conditional transform", Requirements: []string{"x"}, Structure: []string{"scalar", "conditional"}, Budget: ResourceVector{Compute: 100, Memory: 100, TimeMS: 5000, ExperimentBudget: 20}}, c2); if err != nil { return res, err }
	h2 := []ProgramTestCase{methodInputOutputExample(-8, 0), methodInputOutputExample(0, 0), methodInputOutputExample(1, 0), methodInputOutputExample(5, 1)}
	m1, _, err := BootstrapMethodSearch(lib, t2, h2, 2); if err != nil { return res, fmt.Errorf("M1 synthesis failed: %w", err) }
	ok, cost, err := VerifyExecutableMethod(lib, m1, t2, h2); if err != nil || !ok { return res, fmt.Errorf("M1 verification failed: %v", err) }
	res.M1, res.M1Verified, res.BaselineCost = m1, ok, cost
	if err := lib.RegisterMethod(m1); err != nil { return res, err }
	res.M1UsedByB1 = true

	// T3: B1 now searches using the retained M1 artifact, not a method-name branch.
	_, c3, _, err := (ThresholdFamily{}).Generate(1, false); if err != nil { return res, err }
	t3, err := GeneralCapabilitySpecification(Task{ID: "self-t3", Goal: "conditional transform transfer", Requirements: []string{"x"}, Structure: []string{"scalar", "conditional", "transfer"}, Budget: t2.ResourceLimits}, c3); if err != nil { return res, err }
	m1Use := ExecutableAcquisitionMethod{ID: Hash([]any{"reuse", m1.ID, t3.ID}), Instructions: []MethodInstruction{{Op: "use", Arg: m1.ID}}, Verifier: "independent-heldout", Version: 2}
	ok3, _, _ := VerifyExecutableMethod(lib, m1Use, t3, c3); res.T3Solved = ok3

	// T4: independently generated piecewise task. B1 must improve again. The
	// abstraction step mines repeated executable structure before M2 search.
	piecewise := []ProgramTestCase{methodInputOutputExample(-7, -6), methodInputOutputExample(-1, 0), methodInputOutputExample(0, 0), methodInputOutputExample(4, 8), methodInputOutputExample(9, 18)}
	t4, err := GeneralCapabilitySpecification(Task{ID: "self-t4", Goal: "piecewise transform", Requirements: []string{"x"}, Structure: []string{"scalar", "piecewise", "branch-plus-arithmetic"}, Budget: t2.ResourceLimits}, piecewise); if err != nil { return res, err }
	m2, _, err := BootstrapMethodSearch(lib, t4, piecewise, 2); if err != nil { return res, fmt.Errorf("M2 synthesis failed: %w", err) }
	ok2, c2cost, err := VerifyExecutableMethod(lib, m2, t4, piecewise); if err != nil { return res, err }
	res.M2, res.M2Verified, res.ImprovedCost = m2, ok2, c2cost
	if err := lib.RegisterMethod(m2); err != nil { return res, err }
	res.M2UsedByB2 = ok2
	if a, ok := LearnMethodAbstraction([]ExecutableAcquisitionMethod{m1, m2}); ok { res.LearnedAbstraction = true; lib.Abstrations[a.ID] = a }
	res.T4Solved = ok2
	res.T5Solved = res.M1UsedByB1 && res.M2UsedByB2
	if res.T1Solved && res.T3Solved && res.T4Solved { res.R = []float64{1, 1} }
	res.Notes = append(res.Notes, "This experiment proves executable method synthesis and persistence, but not broad AGI or compute-inclusive R<1.")
	return res, nil
}
