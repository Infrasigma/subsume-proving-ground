package acex

import "testing"

func TestV8StructuralRoleLearnerRejectsConflictedRole(t *testing.T) {
	learner := NewV8StructuralRoleLearner()
	state, actions, correct := roleTransferState("conflict", true)
	learner.Observe(state, correct, 1, false)
	learner.Observe(state, actions[0], -1, false)
	if _, ok := learner.Select(state, actions); ok {
		t.Fatal("conflicted structural role was selected")
	}
}
