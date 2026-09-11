package ace

import (
	"errors"
	"sort"
)

// AbstractionContract describes the reusable interface of an acquired
// computational organization. It is deliberately substrate-level rather than
// task-specific: the current abstraction family transforms architecture-
// candidate streams into architecture-candidate streams.
type AbstractionContract struct {
	Inputs         []string
	Outputs        []string
	Preconditions  []string
	Postconditions []string
}

type AbstractionEvidence struct {
	TaskStructure  string
	Verified       bool
	HeldOut        bool
	TransferScore  float64
	DiscoveryCost  ResourceVector
	ObservedGain   float64
}

type AcquiredAbstraction struct {
	ID           string
	Name         string
	Procedure    AcquisitionProcedure
	Contract    AbstractionContract
	Dependencies []string
	Evidence    []AbstractionEvidence
	CostHistory []ResourceVector
	Verification VerificationResult
	Provenance  Provenance
}

type AbstractionLibrary struct {
	Version       uint64
	Abstractions  []AcquiredAbstraction
}

func (l *AbstractionLibrary) Find(id string) (AcquiredAbstraction, bool) {
	for _, a := range l.Abstractions {
		if a.ID == id {
			return a, true
		}
	}
	return AcquiredAbstraction{}, false
}

func (l *AbstractionLibrary) Install(a AcquiredAbstraction) error {
	if a.ID == "" || a.Name == "" {
		return errors.New("incomplete acquired abstraction")
	}
	if len(a.Procedure.Steps) < 2 {
		return errors.New("acquired abstraction must compress a non-trivial composition")
	}
	if _, err := a.Procedure.Marshal(); err != nil {
		return err
	}
	if !a.Verification.Independent || a.Verification.Status != "verified" {
		return errors.New("acquired abstraction lacks independent verification")
	}
	if len(a.Evidence) == 0 {
		return errors.New("acquired abstraction lacks evidence")
	}
	if _, ok := l.Find(a.ID); ok {
		return nil
	}
	l.Abstractions = append(l.Abstractions, a)
	l.Version++
	return nil
}

func (l AbstractionLibrary) IDs() []string {
	out := make([]string, 0, len(l.Abstractions))
	for _, a := range l.Abstractions {
		out = append(out, a.ID)
	}
	sort.Strings(out)
	return out
}

// AbstractionObservation is the minimum evidence needed to turn a successful
// executable composition into a reusable library object. The library is not a
// cache: its later effect is measured by re-entering the executable procedure
// search space through call steps.
type AbstractionObservation struct {
	TaskStructure string
	Procedure     AcquisitionProcedure
	Verified      bool
	HeldOut       bool
	TransferScore float64
	DiscoveryCost ResourceVector
	ObservedGain  float64
}

func abstractionDependencies(p AcquisitionProcedure) []string {
	seen := map[string]bool{}
	for _, s := range p.Steps {
		if s.Op == "call" && s.Ref != "" {
			seen[s.Ref] = true
		}
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// DiscoverReusableAbstraction is intentionally evidence-driven. It accepts no
// developer-provided abstraction name or target algorithm. A composition is
// eligible only when independently verified evidence exists on at least two
// distinct task structures and the composition is genuinely non-trivial.
func DiscoverReusableAbstraction(observations []AbstractionObservation, minDistinctStructures int) (AcquiredAbstraction, error) {
	if minDistinctStructures < 2 {
		minDistinctStructures = 2
	}
	type bucket struct {
		procedure       AcquisitionProcedure
		observations    []AbstractionObservation
		structures      map[string]bool
	}
	buckets := map[string]*bucket{}
	for _, o := range observations {
		if !o.Verified || !o.HeldOut || len(o.Procedure.Steps) < 2 {
			continue
		}
		sig := procedureSignature(o.Procedure)
		b := buckets[sig]
		if b == nil {
			b = &bucket{procedure: o.Procedure, structures: map[string]bool{}}
			buckets[sig] = b
		}
		b.observations = append(b.observations, o)
		if o.TaskStructure != "" {
			b.structures[o.TaskStructure] = true
		}
	}
	var best *bucket
	bestScore := -1.0
	for _, b := range buckets {
		if len(b.structures) < minDistinctStructures {
			continue
		}
		score := 0.0
		for _, o := range b.observations {
			score += o.ObservedGain / (1 + o.DiscoveryCost.Compute + o.DiscoveryCost.Memory + o.DiscoveryCost.TimeMS + o.DiscoveryCost.ExperimentBudget)
		}
		if best == nil || score > bestScore || (score == bestScore && procedureSignature(b.procedure) < procedureSignature(best.procedure)) {
			best = b
			bestScore = score
		}
	}
	if best == nil {
		return AcquiredAbstraction{}, errors.New("no reusable abstraction has cross-structure evidence")
	}
	first := best.observations[0]
	for _, o := range best.observations[1:] {
		if o.DiscoveryCost.Compute < first.DiscoveryCost.Compute {
			first = o
		}
	}
	verification := VerificationResult{
		Status:     "verified",
		Independent: true,
		Expected:   []string{"repeated cross-structure behavioural validity", "composable executable semantics"},
		Observed:   []string{"independent held-out evidence", "distinct task structures"},
		Provenance: Prov("abstraction-verifier", procedureSignature(best.procedure), "cross-structure-evidence", best.observations),
	}
	a := AcquiredAbstraction{
		ID:        Hash([]any{"acquired-abstraction", procedureSignature(best.procedure)}),
		Name:      "acquired-abstraction:" + procedureSignature(best.procedure),
		Procedure: best.procedure,
		Contract: AbstractionContract{
			Inputs:         []string{"architecture-candidate-stream"},
			Outputs:        []string{"architecture-candidate-stream"},
			Preconditions:  []string{"bounded candidate stream", "all referenced abstractions installed"},
			Postconditions: []string{"deterministic executable transformation"},
		},
		Dependencies: abstractionDependencies(best.procedure),
		Verification: verification,
		Provenance:  Prov("abstraction-acquisition", first.TaskStructure, "cross-structure-composition", best.procedure),
	}
	for _, o := range best.observations {
		a.Evidence = append(a.Evidence, AbstractionEvidence{
			TaskStructure: o.TaskStructure,
			Verified:      o.Verified,
			HeldOut:       o.HeldOut,
			TransferScore: o.TransferScore,
			DiscoveryCost: o.DiscoveryCost,
			ObservedGain:  o.ObservedGain,
		})
		a.CostHistory = append(a.CostHistory, o.DiscoveryCost)
	}
	return a, nil
}

func enumerateProcedureAtoms(lib *AbstractionLibrary) []ProcedureStep {
	atoms := []ProcedureStep{
		{Op: "identity"},
		{Op: "reverse"},
		{Op: "dedupe"},
		{Op: "sort-cost"},
		{Op: "take", Arg: 1},
		{Op: "rotate", Arg: 1},
	}
	if lib != nil {
		ids := lib.IDs()
		for _, id := range ids {
			atoms = append(atoms, ProcedureStep{Op: "call", Ref: id})
		}
	}
	return atoms
}

// ProcedureLibrarySearchCost returns the size of the executable procedure
// language explored by a bounded enumerator. It is a bookkeeping primitive for
// discovery-inclusive cost, not a performance target.
func ProcedureLibrarySearchCost(maxSteps int, lib *AbstractionLibrary) int {
	if maxSteps < 1 {
		return 0
	}
	n := len(enumerateProcedureAtoms(lib))
	total := 0
	power := 1
	for depth := 1; depth <= maxSteps; depth++ {
		power *= n
		total += power
	}
	return total
}

// ExecuteAcquiredAbstraction reuses the same interpreter as ordinary
// procedures. There is no second semantics for a library object.
func ExecuteAcquiredAbstraction(a AcquiredAbstraction, cs []ArchitectureCandidate, lib *AbstractionLibrary) ([]ArchitectureCandidate, error) {
	if lib == nil {
		return nil, errors.New("abstraction execution requires a library")
	}
	return executeSearchProcedureWithLibrary(a.Procedure, cs, lib)
}
