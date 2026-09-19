package ace

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

type AcquisitionProcedure struct {
	Version int             `json:"version"`
	Steps   []ProcedureStep `json:"steps"`
}

type ProcedureStep struct {
	Op  string `json:"op"`
	Arg int    `json:"arg,omitempty"`
	Ref string `json:"ref,omitempty"`
}

func (p AcquisitionProcedure) Marshal() (string, error) {
	if p.Version != 1 || len(p.Steps) == 0 {
		return "", errors.New("invalid acquisition procedure")
	}
	for _, s := range p.Steps {
		if s.Op == "call" && s.Ref == "" {
			return "", errors.New("call step requires abstraction reference")
		}
	}
	b, e := json.Marshal(p)
	return string(b), e
}

func decodeAcquisitionProcedure(s string) (AcquisitionProcedure, error) {
	var p AcquisitionProcedure
	if e := json.Unmarshal([]byte(s), &p); e != nil {
		return p, e
	}
	if p.Version != 1 || len(p.Steps) == 0 {
		return p, errors.New("invalid acquisition procedure")
	}
	for _, step := range p.Steps {
		if step.Op == "call" && step.Ref == "" {
			return p, errors.New("call step requires abstraction reference")
		}
	}
	return p, nil
}

func EnumerateAcquisitionProcedures(maxSteps int) []AcquisitionProcedure {
	return enumerateAcquisitionProcedures(maxSteps, nil)
}

func EnumerateAcquisitionProceduresWithLibrary(maxSteps int, lib *AbstractionLibrary) []AcquisitionProcedure {
	procedures := enumerateAcquisitionProcedures(maxSteps, lib)
	atoms := enumerateProcedureAtoms(lib)
	ops := make([]string, 0, len(atoms))
	for _, a := range atoms {
		if a.Op == "call" {
			ops = append(ops, a.Op+":"+a.Ref)
		} else if a.Op == "take" || a.Op == "rotate" {
			ops = append(ops, fmt.Sprintf("%s:%d", a.Op, a.Arg))
		} else {
			ops = append(ops, a.Op)
		}
	}
	libraryCount := 0
	if lib != nil {
		libraryCount = len(lib.Abstractions)
	}
	fmt.Printf("ACQ_GENERATOR_RAW max_steps=%d atom_count=%d generated=%d library_count=%d atom_ops=%v\n", maxSteps, len(atoms), len(procedures), libraryCount, ops)
	return procedures
}

func enumerateAcquisitionProcedures(maxSteps int, lib *AbstractionLibrary) []AcquisitionProcedure {
	if maxSteps < 1 {
		return nil
	}
	atoms := enumerateProcedureAtoms(lib)
	out := make([]AcquisitionProcedure, 0, 64)
	var rec func([]ProcedureStep, int)
	rec = func(prefix []ProcedureStep, depth int) {
		if depth == 0 {
			out = append(out, AcquisitionProcedure{Version: 1, Steps: append([]ProcedureStep(nil), prefix...)})
			return
		}
		for _, a := range atoms {
			next := append(append([]ProcedureStep(nil), prefix...), a)
			rec(next, depth-1)
		}
	}
	for d := 1; d <= maxSteps; d++ {
		rec(nil, d)
	}
	return out
}

func executeSearchProcedure(p AcquisitionProcedure, cs []ArchitectureCandidate) ([]ArchitectureCandidate, error) {
	return executeSearchProcedureWithLibrary(p, cs, nil)
}

const maxAcquisitionProcedureExecutionSteps = 256

func executeSearchProcedureWithLibrary(p AcquisitionProcedure, cs []ArchitectureCandidate, lib *AbstractionLibrary) ([]ArchitectureCandidate, error) {
	return executeSearchProcedureWithLibraryState(p, cs, lib, map[string]bool{}, new(int))
}

func executeSearchProcedureWithLibraryState(p AcquisitionProcedure, cs []ArchitectureCandidate, lib *AbstractionLibrary, callStack map[string]bool, steps *int) ([]ArchitectureCandidate, error) {
	cur := append([]ArchitectureCandidate(nil), cs...)
	for _, s := range p.Steps {
		*steps++
		if *steps > maxAcquisitionProcedureExecutionSteps {
			return nil, errors.New("acquisition procedure execution step limit exceeded")
		}
		switch s.Op {
		case "identity":
		case "reverse":
			for i, j := 0, len(cur)-1; i < j; i, j = i+1, j-1 {
				cur[i], cur[j] = cur[j], cur[i]
			}
		case "dedupe":
			seen := map[string]bool{}
			next := make([]ArchitectureCandidate, 0, len(cur))
			for _, c := range cur {
				if !seen[c.Mechanism] {
					seen[c.Mechanism] = true
					next = append(next, c)
				}
			}
			cur = next
		case "sort-cost":
			sort.SliceStable(cur, func(i, j int) bool {
				return cur[i].Resources.Compute+cur[i].Resources.ExperimentBudget < cur[j].Resources.Compute+cur[j].Resources.ExperimentBudget
			})
		case "take":
			if s.Arg < 1 || s.Arg > len(cur) {
				return nil, fmt.Errorf("take argument %d outside stream", s.Arg)
			}
			cur = append([]ArchitectureCandidate(nil), cur[:s.Arg]...)
		case "rotate":
			if len(cur) == 0 {
				continue
			}
			n := s.Arg % len(cur)
			if n < 0 {
				n += len(cur)
			}
			cur = append(append([]ArchitectureCandidate(nil), cur[n:]...), cur[:n]...)
		case "call":
			if lib == nil {
				return nil, errors.New("call step requires acquired abstraction library")
			}
			if callStack[s.Ref] {
				return nil, fmt.Errorf("cyclic acquired abstraction call %q", s.Ref)
			}
			a, ok := lib.Find(s.Ref)
			if !ok {
				return nil, fmt.Errorf("unknown acquired abstraction %q", s.Ref)
			}
			trusted := ""; if lib.TrustedSigners != nil { trusted = lib.TrustedSigners[a.KMSSignature.SignerID] }
			if err := a.VerifyAdmission(trusted); err != nil {
				return nil, fmt.Errorf("cryptographic abstraction admission rejected for %q: %w", s.Ref, err)
			}
			callStack[s.Ref] = true
			var err error
			cur, err = executeSearchProcedureWithLibraryState(a.Procedure, cur, lib, callStack, steps)
			delete(callStack, s.Ref)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unknown acquisition procedure op %q", s.Op)
		}
	}
	return cur, nil
}

func procedureSignature(p AcquisitionProcedure) string {
	b, _ := json.Marshal(p)
	return Hash(string(b))
}
