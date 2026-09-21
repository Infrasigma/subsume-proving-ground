package acex

import (
	"strings"
	"testing"
)

func TestV4TypedParameterizedConcepts(t *testing.T) {
	seeds := []int64{610117, 830921}
	for _, seed := range seeds {
		intTasks, err := V4MakeFirstIntTasks(seed)
		if err != nil { t.Fatal(err) }
		boolTasks, err := V4MakeFirstBoolTasks(seed)
		if err != nil { t.Fatal(err) }

		lib := NewV4Library()
		intLearn, err := V4LearnConcept(intTasks, lib, 9, 1200)
		if err != nil { t.Fatalf("seed=%d int concept: %v", seed, err) }
		lib = intLearn.Library
		if len(intLearn.Added) != 1 || len(intLearn.Added[0].Params) < 1 {
			t.Fatalf("seed=%d missing parameterized int concept: %+v", seed, intLearn.Added)
		}

		boolLearn, err := V4LearnConcept(boolTasks, lib, 9, 1200)
		if err != nil { t.Fatalf("seed=%d bool concept: %v", seed, err) }
		lib = boolLearn.Library
		if len(boolLearn.Added) != 1 || len(boolLearn.Added[0].Params) < 1 {
			t.Fatalf("seed=%d missing parameterized bool concept", seed)
		}

		second, err := V4MakeSecondIntTasks(intLearn.Added[0])
		if err != nil { t.Fatal(err) }
		secondLearn, err := V4LearnConcept(second, lib, 9, 1200)
		if err != nil { t.Fatalf("seed=%d recursive int concept: %v", seed, err) }
		if len(secondLearn.Added) != 1 || !strings.Contains(v4Signature(secondLearn.Added[0].Body), "call:"+intLearn.Added[0].Name) {
			t.Fatalf("seed=%d recursive concept did not reference first concept: %+v", seed, secondLearn.Added)
		}
		lib = secondLearn.Library

		hidden, err := V4MakeHiddenSuccessors(seed, secondLearn.Added[0], boolLearn.Added[0], lib)
		if err != nil { t.Fatal(err) }
		ratios, before, after, err := V4VerifyHiddenSuccessors(lib, NewV4Library(), hidden, 9, 1200)
		if err != nil { t.Fatalf("seed=%d hidden verification: %v", seed, err) }
		if len(ratios) != 6 || after >= before {
			t.Fatalf("seed=%d hidden costs not reduced: %d -> %d ratios=%v", seed, before, after, ratios)
		}
		t.Logf("V4 seed=%d int=%s bool=%s recursive=%s visible=%d->%d hidden=%d->%d ratios=%v digest=%s",
			seed, intLearn.Added[0].Name, boolLearn.Added[0].Name, secondLearn.Added[0].Name,
			intLearn.BeforeCost+boolLearn.BeforeCost+secondLearn.BeforeCost,
			intLearn.AfterCost+boolLearn.AfterCost+secondLearn.AfterCost,
			before, after, ratios, lib.Digest())
	}
}

func TestV4RuntimePromotionRollback(t *testing.T) {
	intTasks, err := V4MakeFirstIntTasks(101)
	if err != nil { t.Fatal(err) }
	lib := NewV4Library()
	intLearn, err := V4LearnConcept(intTasks, lib, 9, 1200)
	if err != nil { t.Fatal(err) }
	second, err := V4MakeSecondIntTasks(intLearn.Added[0])
	if err != nil { t.Fatal(err) }
	secondLearn, err := V4LearnConcept(second, intLearn.Library, 9, 1200)
	if err != nil { t.Fatal(err) }
	boolTasks, err := V4MakeFirstBoolTasks(101)
	if err != nil { t.Fatal(err) }
	boolLearn, err := V4LearnConcept(boolTasks, secondLearn.Library, 9, 1200)
	if err != nil { t.Fatal(err) }
	final := boolLearn.Library
	hidden, err := V4MakeHiddenSuccessors(101, secondLearn.Added[0], boolLearn.Added[0], final)
	if err != nil { t.Fatal(err) }

	r := NewCognitiveRuntime()
	before := r.V4Library.Digest()
	totalBefore := intLearn.BeforeCost + secondLearn.BeforeCost + boolLearn.BeforeCost
	totalAfter := intLearn.AfterCost + secondLearn.AfterCost + boolLearn.AfterCost
	if err := r.PromoteV4Library(final, totalBefore, totalAfter, hidden, 9, 1200); err != nil {
		t.Fatal(err)
	}
	if r.V4Library.Digest() != final.Digest() || r.Version != 1 {
		t.Fatalf("promotion failed: digest=%s version=%d", r.V4Library.Digest(), r.Version)
	}
	if receipt, ok := r.ImprovementLedger.Latest(); !ok || !receipt.HiddenPassed || receipt.AfterCost >= receipt.BeforeCost {
		t.Fatalf("missing proof-carrying receipt: %+v", receipt)
	}
	if err := r.RollbackV4Library(); err != nil { t.Fatal(err) }
	if r.V4Library.Digest() != before || r.Version != 0 {
		t.Fatalf("rollback failed: digest=%s version=%d", r.V4Library.Digest(), r.Version)
	}
}

func TestV4ArchitectureTerminal(t *testing.T) {
	if testing.Short() { t.Skip("V4 terminal disabled in short mode") }
	seeds := []int64{610117, 830921}
	for _, seed := range seeds {
		intTasks, err := V4MakeFirstIntTasks(seed)
		if err != nil { t.Fatal(err) }
		boolTasks, err := V4MakeFirstBoolTasks(seed)
		if err != nil { t.Fatal(err) }
		lib := NewV4Library()

		intLearn, err := V4LearnConcept(intTasks, lib, 9, 1200)
		if err != nil { t.Fatalf("seed=%d int learning: %v", seed, err) }
		lib = intLearn.Library

		boolLearn, err := V4LearnConcept(boolTasks, lib, 9, 1200)
		if err != nil { t.Fatalf("seed=%d bool learning: %v", seed, err) }
		lib = boolLearn.Library

		second, err := V4MakeSecondIntTasks(intLearn.Added[0])
		if err != nil { t.Fatal(err) }
		secondLearn, err := V4LearnConcept(second, lib, 9, 1200)
		if err != nil { t.Fatalf("seed=%d recursive learning: %v", seed, err) }
		lib = secondLearn.Library

		hidden, err := V4MakeHiddenSuccessors(seed, secondLearn.Added[0], boolLearn.Added[0], lib)
		if err != nil { t.Fatal(err) }
		ratios, before, after, err := V4VerifyHiddenSuccessors(lib, NewV4Library(), hidden, 9, 1200)
		if err != nil { t.Fatalf("seed=%d hidden verification: %v ratios=%v before=%d after=%d", seed, err, ratios, before, after) }
		if len(ratios) != 6 { t.Fatalf("seed=%d hidden case count=%d", seed, len(ratios)) }

		reloaded := cloneV4Library(lib)
		if reloaded.Digest() != lib.Digest() { t.Fatalf("seed=%d rehydration digest mismatch", seed) }

		rt := NewCognitiveRuntime()
		if err := rt.PromoteV4Library(lib,
			intLearn.BeforeCost+boolLearn.BeforeCost+secondLearn.BeforeCost,
			intLearn.AfterCost+boolLearn.AfterCost+secondLearn.AfterCost,
			hidden, 9, 1200); err != nil {
			t.Fatalf("seed=%d promotion: %v", seed, err)
		}
		if err := rt.RollbackV4Library(); err != nil { t.Fatalf("seed=%d rollback: %v", seed, err) }

		t.Logf("ACEX V4 TERMINAL seed=%d ratios=%v visible=%d->%d hidden=%d->%d digest=%s",
			seed, ratios,
			intLearn.BeforeCost+boolLearn.BeforeCost+secondLearn.BeforeCost,
			intLearn.AfterCost+boolLearn.AfterCost+secondLearn.AfterCost,
			before, after, lib.Digest())
	}
}
