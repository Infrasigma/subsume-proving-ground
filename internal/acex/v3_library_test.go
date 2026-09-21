package acex

import (
	"strings"
	"testing"
)

func TestV3TypedLibraryGrowthCore(t *testing.T) {
	if !v3KillMode { t.Skip("V3 route is preserved as a scientific KILL; run with -tags v3kill") }
	lib := NewV3Library()
	first, err := V3MakeGenerationOneTasks(lib, 101)
	if err != nil { t.Fatal(err) }
	secondSeed := NewV3Library()
	m1lib, _, _, _, err := V3LearnGeneration(first, secondSeed, 7, 1200)
	if err != nil { t.Fatal(err) }

	var firstName string
	for name := range m1lib.Macros { firstName = name; break }
	if firstName == "" { t.Fatal("generation 1 invented no macro") }

	second, err := V3MakeGenerationTwoTasks(m1lib, firstName)
	if err != nil { t.Fatal(err) }

	result, err := V3LearnTwoGenerations(first, second, lib, 7, 1200)
	if err != nil { t.Fatal(err) }
	if len(result.Added) != 2 {
		t.Fatalf("expected two learned abstractions, got %d: %+v", len(result.Added), result.Added)
	}
	if !strings.Contains(result.Added[1].Name, "v3skill-") {
		t.Fatalf("unexpected second-generation macro name: %+v", result.Added[1])
	}
	if !v3ContainsMacro(result.Added[1].Body, result.Added[0].Name) {
		t.Fatalf("second abstraction does not reference first abstraction: first=%+v second=%+v", result.Added[0], result.Added[1])
	}
	if result.AfterCost >= result.BeforeCost {
		t.Fatalf("library did not compound resource-normalized cost: before=%d after=%d", result.BeforeCost, result.AfterCost)
	}

	rehydrated := cloneV3Library(result.Library)
	if rehydrated.Digest() != result.Library.Digest() {
		t.Fatalf("rehydration changed library digest: before=%s after=%s", result.Library.Digest(), rehydrated.Digest())
	}
	if len(rehydrated.Macros) != len(result.Library.Macros) {
		t.Fatalf("rehydration changed macro count: %d vs %d", len(rehydrated.Macros), len(result.Library.Macros))
	}
}

func TestV3RuntimePromotionAndRollback(t *testing.T) {
	if !v3KillMode { t.Skip("V3 route is preserved as a scientific KILL; run with -tags v3kill") }
	r := NewCognitiveRuntime()
	base := NewV3Library()
	first, err := V3MakeGenerationOneTasks(base, 202)
	if err != nil { t.Fatal(err) }
	tmp, _, _, _, err := V3LearnGeneration(first, base, 7, 1200)
	if err != nil { t.Fatal(err) }
	var m1 string
	for name := range tmp.Macros { m1 = name; break }
	second, err := V3MakeGenerationTwoTasks(tmp, m1)
	if err != nil { t.Fatal(err) }
	learned, err := V3LearnTwoGenerations(first, second, base, 7, 1200)
	if err != nil { t.Fatal(err) }

	hidden, err := V3MakeHiddenSuccessors(learned.Library, learned.Added[0].Name, learned.Added[1].Name, 202)
	if err != nil { t.Fatal(err) }

	before := r.V3Library.Digest()
	if err := r.PromoteV3Library(learned.Library, learned.BeforeCost, learned.AfterCost, hidden, 8, 1200); err != nil {
		t.Fatal(err)
	}
	after := r.V3Library.Digest()
	if before == after || r.Version != 1 {
		t.Fatalf("promotion did not advance runtime library: before=%s after=%s version=%d", before, after, r.Version)
	}
	receipt, ok := r.ImprovementLedger.Latest()
	if !ok || !receipt.HiddenPassed || receipt.AfterCost >= receipt.BeforeCost {
		t.Fatalf("missing proof-carrying V3 receipt: %+v", receipt)
	}
	if err := r.RollbackV3Library(); err != nil {
		t.Fatal(err)
	}
	if r.V3Library.Digest() != before || r.Version != 0 {
		t.Fatalf("rollback failed: digest=%s version=%d", r.V3Library.Digest(), r.Version)
	}
}
