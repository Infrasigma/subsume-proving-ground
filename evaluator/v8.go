package evaluator

// ConsequenceClass identifies one of the eight independently addressable
// consequence bits in the V8 multi-hot target.
type ConsequenceClass uint8

const (
	ClassAgentMove ConsequenceClass = iota
	ClassContactElastic
	ClassContactInelastic
	ClassPropertyMutation
	ClassCreationDeletion
	ClassDelayedTrigger
	ClassTerminalGoal
	ClassNull
)

const ConsequenceClassCount = 8

// TransitionEvidence stores independently observed transition mechanics.
// Rules are accumulated without short-circuiting so one transition may carry
// several consequence classes at once.
type TransitionEvidence struct {
	AgentMoved       bool
	ElasticContact   bool
	InelasticContact bool
	PropertyMutation bool
	CreationDeletion bool
	DelayedTrigger   bool
	Terminal         bool
}

// ProjectMultiHot projects every independently observed mechanic into the
// fixed eight-bit V8 target. Agent movement is deliberately independent of
// contact: a transition may contain AGENT_MOVE and CONTACT_* simultaneously.
func (e TransitionEvidence) ProjectMultiHot() uint8 {
	var mask uint8
	if e.AgentMoved {
		mask |= uint8(1) << ClassAgentMove
	}
	if e.ElasticContact {
		mask |= uint8(1) << ClassContactElastic
	}
	if e.InelasticContact {
		mask |= uint8(1) << ClassContactInelastic
	}
	if e.PropertyMutation {
		mask |= uint8(1) << ClassPropertyMutation
	}
	if e.CreationDeletion {
		mask |= uint8(1) << ClassCreationDeletion
	}
	if e.DelayedTrigger {
		mask |= uint8(1) << ClassDelayedTrigger
	}
	if e.Terminal {
		mask |= uint8(1) << ClassTerminalGoal
	}
	if mask == 0 {
		mask |= uint8(1) << ClassNull
	}
	return mask
}

// Record retains the complete projected target for callers that already have
// an externally computed bit/class. It does not clear previously observed
// evidence.
type MaskedEvidence struct {
	Mask uint8
}

func (e *MaskedEvidence) Record(class ConsequenceClass) {
	if class >= ConsequenceClassCount {
		return
	}
	e.Mask |= uint8(1) << class
}

func (e MaskedEvidence) Has(class ConsequenceClass) bool {
	if class >= ConsequenceClassCount {
		return false
	}
	return e.Mask&(uint8(1)<<class) != 0
}

func (e MaskedEvidence) ActiveCount() int {
	m := e.Mask
	n := 0
	for m != 0 {
		m &= m - 1
		n++
	}
	return n
}
