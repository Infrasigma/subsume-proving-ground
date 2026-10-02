package acex

import "errors"

func (r *CognitiveRuntime) PromoteV4Library(candidate V4Library, beforeCost, afterCost int, hidden []V4HiddenCase, maxSize, beam int) error {
	if r == nil {
		return errors.New("nil cognitive runtime")
	}
	if candidate.Digest() == r.V4Library.Digest() {
		return errors.New("candidate V4 library is unchanged")
	}
	if afterCost >= beforeCost {
		return errors.New("candidate V4 library has no visible improvement")
	}
	if _, _, _, err := V4VerifyHiddenSuccessors(candidate, NewV4Library(), hidden, maxSize, beam); err != nil {
		return err
	}
	receipt := MakeImprovementReceipt(r.Version, r.Version+1, "v4-equivalence-library:"+candidate.Digest(), beforeCost, afterCost, true)
	if err := r.ImprovementLedger.Admit(receipt); err != nil {
		return err
	}
	r.V4LibraryHistory = append(r.V4LibraryHistory, cloneV4Library(r.V4Library))
	r.V4Library = cloneV4Library(candidate)
	r.V4Library.Version++
	r.Version++
	r.Self.Capabilities["parameterized-concept-induction"] = 1
	r.Self.Capabilities["equivalence-aware-library-growth"] = 1
	return nil
}

func (r *CognitiveRuntime) RollbackV4Library() error {
	if r == nil || len(r.V4LibraryHistory) == 0 {
		return errors.New("no V4 library rollback available")
	}
	r.V4Library = cloneV4Library(r.V4LibraryHistory[len(r.V4LibraryHistory)-1])
	r.V4LibraryHistory = r.V4LibraryHistory[:len(r.V4LibraryHistory)-1]
	if r.Version > 0 {
		r.Version--
	}
	return nil
}
