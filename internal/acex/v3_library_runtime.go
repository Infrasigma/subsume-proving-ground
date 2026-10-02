package acex

import "errors"

func (r *CognitiveRuntime) PromoteV3Library(candidate V3Library, beforeCost, afterCost int, hidden []V3HiddenCase, maxSize, beam int) error {
	if r == nil {
		return errors.New("nil cognitive runtime")
	}
	if candidate.Digest() == r.V3Library.Digest() {
		return errors.New("candidate library is unchanged")
	}
	if afterCost >= beforeCost {
		return errors.New("candidate library has no visible resource improvement")
	}
	ratios, _, _, err := V3VerifyHiddenSuccessors(candidate, NewV3Library(), hidden, maxSize, beam)
	if err != nil {
		return err
	}
	_ = ratios
	receipt := MakeImprovementReceipt(r.Version, r.Version+1, "v3-typed-library:"+candidate.Digest(), beforeCost, afterCost, true)
	if err := r.ImprovementLedger.Admit(receipt); err != nil {
		return err
	}
	r.V3LibraryHistory = append(r.V3LibraryHistory, cloneV3Library(r.V3Library))
	r.V3Library = cloneV3Library(candidate)
	r.V3Library.Version = r.V3Library.Version + 1
	r.Version++
	r.Self.Capabilities["typed-library-invention"] = 1
	r.Self.Capabilities["recursive-library-composition"] = 1
	return nil
}

func (r *CognitiveRuntime) RollbackV3Library() error {
	if r == nil || len(r.V3LibraryHistory) == 0 {
		return errors.New("no V3 library rollback available")
	}
	r.V3Library = cloneV3Library(r.V3LibraryHistory[len(r.V3LibraryHistory)-1])
	r.V3LibraryHistory = r.V3LibraryHistory[:len(r.V3LibraryHistory)-1]
	if r.Version > 0 {
		r.Version--
	}
	return nil
}
