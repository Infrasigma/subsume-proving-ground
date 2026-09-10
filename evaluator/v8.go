package evaluator

// ConsequenceClass identifies one of the eight independently addressable
// consequence bits in the V8 multi-hot target.
type ConsequenceClass uint8

const ConsequenceClassCount = 8

// TransitionEvidence stores the complete multi-hot consequence target.
// Bit k is set iff consequence class k is active.
type TransitionEvidence struct {
	Mask uint8
}

func (e *TransitionEvidence) Record(class ConsequenceClass) {
	if class >= ConsequenceClassCount {
		return
	}
	e.Mask |= uint8(1) << class
}

func (e TransitionEvidence) Has(class ConsequenceClass) bool {
	if class >= ConsequenceClassCount {
		return false
	}
	return e.Mask&(uint8(1)<<class) != 0
}

func (e TransitionEvidence) ActiveCount() int {
	m := e.Mask
	n := 0
	for m != 0 {
		m &= m - 1
		n++
	}
	return n
}
