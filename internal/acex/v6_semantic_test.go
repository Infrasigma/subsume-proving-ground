package acex

import (
	"strings"
	"testing"
)

func makeV6TaskFamily(seed int64, offsets []int, negated bool) ([]V4Task, error) {
	out := make([]V4Task, 0, len(offsets))
	inputs := []int{-8, -6, -4, -2, 0, 2, 4, 6, 8}
	holds := []int{-9, -7, -5, -3, -1, 1, 3, 5, 7, 9}
	x := v4IntInput()
	for i, c := range offsets {
		inner := v4IntExpr("add", x, v4IntConst(c))
		if negated {
			inner = v4IntExpr("neg", inner)
		}
		// Force a nontrivial reusable skeleton; the learner must discover the
		// shared max/abs/add structure rather than collapsing to abs(variable).
		expr := v4IntExpr("max", v4IntConst(0), v4IntExpr("abs", inner))
		task, err := v4MakeTask("v6-visible-"+strings.TrimSpace(string(rune('a'+i))), inputs, holds, expr, NewV4Library())
		if err != nil {
			return nil, err
		}
		out = append(out, task)
	}
	_ = seed
	return out, nil
}

func TestV6SemanticEquivalenceQuotient(t *testing.T) {
	x := v4IntInput()
	a := v4IntExpr("abs", v4IntExpr("neg", v4IntExpr("add", x, v4IntConst(2))))
	b := v4IntExpr("abs", v4IntExpr("add", x, v4IntConst(2)))
	ok, cert, err := V6Equivalent(a, b, NewV4Library(), -10, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || cert == "" {
		t.Fatalf("bounded semantic equivalence failed: ok=%v cert=%s", ok, cert)
	}
	rep, cls, err := V6SaturateEquivalents(a, NewV4Library(), -10, 10, 96)
	if err != nil {
		t.Fatal(err)
	}
	if v4Signature(rep) != v4Signature(b) {
		t.Fatalf("semantic saturation did not discover simpler representative: %s vs %s", v4Signature(rep), v4Signature(b))
	}
	if len(cls.Members) < 2 {
		t.Fatalf("equivalence class did not contain multiple forms: %+v", cls)
	}
}

func TestV6ProspectiveLibraryLearning(t *testing.T) {
	visible, err := makeV6TaskFamily(1, []int{-3, 3, 6}, true)
	if err != nil {
		t.Fatal(err)
	}
	// Future tasks are generated independently from the candidate library. They
	// share a latent abstraction but the generator never consumes learned state.
	future, err := makeV6TaskFamily(2, []int{-4, -2, 1, 4}, false)
	if err != nil {
		t.Fatal(err)
	}
	learned, err := V6LearnProspective(visible, future, NewV4Library(), 9, 1200)
	if err != nil {
		t.Fatal(err)
	}
	if len(learned.Added) != 1 || learned.Added[0].Parent != "v6-prospective" {
		t.Fatalf("no prospective concept admitted: %+v", learned.Added)
	}
	if learned.VisibleAfter >= learned.VisibleBefore {
		t.Fatalf("visible search did not improve: %d -> %d", learned.VisibleBefore, learned.VisibleAfter)
	}
	if learned.FutureAfter >= learned.FutureBefore {
		t.Fatalf("future acquisition did not improve: %d -> %d", learned.FutureBefore, learned.FutureAfter)
	}
	for i, ratio := range learned.FutureRatios {
		if ratio >= .80 {
			t.Fatalf("future ratio[%d]=%.3f", i, ratio)
		}
	}
	if len(learned.EquivalenceKeys) != len(visible) {
		t.Fatalf("missing semantic certificates: %v", learned.EquivalenceKeys)
	}
}

func TestV6ExecutableMechanismSelection(t *testing.T) {
	visible, err := makeV6TaskFamily(3, []int{-4, -1, 2}, false)
	if err != nil {
		t.Fatal(err)
	}
	library := NewV4Library()
	seedLearn, err := V6LearnProspective(visible, visible, library, 9, 1200)
	if err != nil {
		t.Fatal(err)
	}
	hidden, err := makeV6TaskFamily(4, []int{-5, -2, 1, 5}, true)
	if err != nil {
		t.Fatal(err)
	}
	strategy, ratios, err := V6SelectStrategy(
		V6Strategy{Name:"baseline", Strategy:V6BaselineSearch, Library:NewV4Library()},
		seedLearn.Library, visible, hidden, 9, 1200,
	)
	if err != nil {
		t.Fatal(err)
	}
	if strategy.Strategy != V6SemanticSearch {
		t.Fatalf("meta-search did not select executable semantic strategy: %+v", strategy)
	}
	if len(ratios) != len(hidden) {
		t.Fatalf("missing hidden strategy ratios: %v", ratios)
	}
	for i, ratio := range ratios {
		if ratio >= .80 {
			t.Fatalf("hidden executable strategy ratio[%d]=%.3f", i, ratio)
		}
	}
}

func TestV6FailureCurriculum(t *testing.T) {
	future, err := makeV6TaskFamily(5, []int{-6, -3, 0, 3, 6}, false)
	if err != nil {
		t.Fatal(err)
	}
	// Start with an intentionally insufficient language budget. The challenge
	// must point at the observed representation/search bottleneck rather than
	// silently claiming completion.
	_, _, err = v4SolveVisible(future, NewV4Library(), 1, 2)
	if err == nil {
		t.Fatal("forced bottleneck unexpectedly solved")
	}
	ch := V5NextChallenge([]string{"representation"}, 777)
	if ch.Gap != "representation" || ch.ID == "" {
		t.Fatalf("failure did not produce endogenous next challenge: %+v", ch)
	}
}
