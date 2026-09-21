package acex

import "testing"

func TestV2PredictiveModelRevisionAndPlanning(t *testing.T) {
	m := NewPredictiveModel()

	episodes := []Transition{
		{Before:NumericState{"x":0},Action:"inc",After:NumericState{"x":1}},
		{Before:NumericState{"x":1},Action:"inc",After:NumericState{"x":2}},
		{Before:NumericState{"x":2},Action:"inc",After:NumericState{"x":3}},
	}
	for _, tr := range episodes {
		if err := m.Observe(tr); err != nil {
			t.Fatal(err)
		}
	}

	p := m.Predict(NumericState{"x":9}, "inc")
	if !p.Known || p.State["x"] != 10 || p.Confidence <= 0.5 {
		t.Fatalf("bad learned prediction: %+v", p)
	}

	plan, err := PlanWithForesight(
		m,
		NumericState{"x":0},
		[]string{"inc"},
		func(s NumericState) bool { return s["x"] >= 3 },
		4,
		0.5,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Actions) != 3 || plan.State["x"] != 3 || plan.Confidence < 0.5 {
		t.Fatalf("bad plan: %+v", plan)
	}

	// Surprising transition must reduce trust instead of silently accepting
	// the old world model.
	ok := m.VerifyAndRevise(Transition{
		Before:NumericState{"x":3},
		Action:"inc",
		After:NumericState{"x":8},
	})
	if ok {
		t.Fatal("surprising transition incorrectly accepted")
	}
	p = m.Predict(NumericState{"x":3}, "inc")
	if p.Confidence >= 1.0 {
		t.Fatalf("stale model retained full confidence after contradiction: %+v", p)
	}

	// Low-confidence foresight is an abstention path; planner refuses to
	// build a high-confidence plan from it.
	m.Actions["unsafe"] = &ActionModel{Count:0, Delta:map[string]int{"x":100}}
	if _, err := PlanWithForesight(
		m,
		NumericState{"x":0},
		[]string{"unsafe"},
		func(s NumericState) bool { return s["x"] >= 100 },
		1,
		0.5,
	); err == nil {
		t.Fatal("planner trusted an unverified prediction")
	}
}
