package acex

import "testing"

type runtimeWorld struct {
	state NumericState
}

func (w *runtimeWorld) Observe() NumericState {
	return copyState(w.state)
}

func (w *runtimeWorld) Act(action string) (NumericState, error) {
	x := w.state["x"]
	switch action {
	case "inc":
		if x == 3 {
			x = 5
		} else {
			x++
		}
	default:
		return nil, nil
	}
	w.state["x"] = x
	return copyState(w.state), nil
}

func TestV2IntegratedCognitiveRuntime(t *testing.T) {
	r := NewCognitiveRuntime()

	// Learn a reusable action model from experience.
	for _, tr := range []Transition{
		{Before:NumericState{"x":0},Action:"inc",After:NumericState{"x":1}},
		{Before:NumericState{"x":1},Action:"inc",After:NumericState{"x":2}},
		{Before:NumericState{"x":2},Action:"inc",After:NumericState{"x":3}},
	} {
		if err := r.LearnTransition(tr.Before, tr.Action, tr.After); err != nil {
			t.Fatal(err)
		}
	}

	plan, err := r.Plan(NumericState{"x":0}, []string{"inc"}, func(s NumericState) bool { return s["x"] >= 3 })
	if err != nil {
		t.Fatal(err)
	}
	world := &runtimeWorld{state:NumericState{"x":0}}
	end, err := r.ExecutePlan(world, NumericState{"x":0}, plan)
	if err != nil {
		t.Fatal(err)
	}
	if end["x"] != 3 {
		t.Fatalf("integrated plan failed: %+v", end)
	}

	// The environment now violates the learned predictive rule. The runtime
	// must record the contradiction and lower trust rather than silently
	// declaring success.
	end, err = r.ExecutePlan(world, NumericState{"x":3}, PlanResult{
		Actions: []string{"inc"},
		State: NumericState{"x":4},
		Confidence:1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if end["x"] != 5 {
		t.Fatalf("unexpected world transition: %+v", end)
	}
	if len(r.Self.Failures) == 0 {
		t.Fatal("prediction mismatch was not recorded")
	}
	if r.Self.KnownActions["inc"] >= 1 {
		t.Fatalf("model confidence failed to fall after contradiction: %+v", r.Self.KnownActions)
	}

	traces := []Trace{
		{Steps:[]TraceStep{{Op:"observe"},{Op:"sort"},{Op:"dedupe"},{Op:"commit"}}},
		{Steps:[]TraceStep{{Op:"inspect"},{Op:"sort"},{Op:"dedupe"},{Op:"save"}}},
		{Steps:[]TraceStep{{Op:"parse"},{Op:"sort"},{Op:"dedupe"},{Op:"emit"}}},
	}
	concept := Concept{ID:"runtime-concept",Accuracy:1,Features:[]string{"sort","dedupe"},Complexity:2}
	if err := r.ConsolidateTrace(traces, concept); err != nil {
		t.Fatal(err)
	}
	if len(r.Memory.Items) == 0 || len(r.Mechanisms) == 0 {
		t.Fatalf("integrated consolidation failed: memory=%+v mechanisms=%+v", r.Memory.Items, r.Mechanisms)
	}
	if r.Diagnose() == "stable" {
		t.Fatal("runtime failed to expose learned failure state")
	}
}
