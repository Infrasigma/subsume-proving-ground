package acex

import (
	"fmt"
	"os"
	"strconv"
	"testing"
)

func TestV3ArchitectureTerminal(t *testing.T) {
	seeds := []int64{610117, 830921}
	runtimeSeed := int64(0)
	if raw := os.Getenv("ACEX_RUNTIME_SEED"); raw != "" {
		if v, err := strconv.ParseInt(raw, 10, 64); err == nil {
			runtimeSeed = v % 1000000
		}
	}
	for _, seed := range seeds {
		if runtimeSeed != 0 {
			seed += runtimeSeed * 37
		}
		base := NewV3Library()
		first, err := V3MakeGenerationOneTasks(base, seed)
		if err != nil { t.Fatalf("seed=%d generation1 task construction: %v", seed, err) }
		learned, err := V3LearnGeneration(first, base, 7, 1200)
		if err != nil { t.Fatalf("seed=%d generation1 learning: %v", seed, err) }
		var m1 string
		for name := range learned.Macros { m1 = name; break }
		if m1 == "" { t.Fatalf("seed=%d generation1 produced no macro", seed) }

		second, err := V3MakeGenerationTwoTasks(learned.Library, m1)
		if err != nil { t.Fatalf("seed=%d generation2 task construction: %v", seed, err) }
		combined, err := V3LearnTwoGenerations(first, second, base, 7, 1200)
		if err != nil { t.Fatalf("seed=%d recursive learning: %v", seed, err) }
		if len(combined.Added) != 2 {
			t.Fatalf("seed=%d expected 2 learned macros, got %+v", seed, combined.Added)
		}

		hidden, err := V3MakeHiddenSuccessors(combined.Library, combined.Added[0].Name, combined.Added[1].Name, seed)
		if err != nil { t.Fatalf("seed=%d hidden generation: %v", seed, err) }
		ratios, beforeHidden, afterHidden, err := V3VerifyHiddenSuccessors(combined.Library, base, hidden, 8, 1200)
		if err != nil { t.Fatalf("seed=%d hidden verification: %v", seed, err) }

		rt := NewCognitiveRuntime()
		if err := rt.PromoteV3Library(combined.Library, combined.BeforeCost, combined.AfterCost, hidden, 8, 1200); err != nil {
			t.Fatalf("seed=%d proof-carrying promotion: %v", seed, err)
		}
		digest := rt.V3Library.Digest()
		if digest != combined.Library.Digest() {
			t.Fatalf("seed=%d promotion digest mismatch: runtime=%s candidate=%s", seed, digest, combined.Library.Digest())
		}
		if err := rt.RollbackV3Library(); err != nil { t.Fatalf("seed=%d rollback: %v", seed, err) }

		t.Logf("ACEX V3 seed=%d added=%v visible_cost=%d->%d hidden_cost=%d->%d ratios=%v digest=%s",
			seed, []string{combined.Added[0].Name, combined.Added[1].Name},
			combined.BeforeCost, combined.AfterCost, beforeHidden, afterHidden, ratios, combined.Library.Digest())
		fmt.Printf("ACEX V3 TERMINAL BLOCK PASS seed=%d ratios=%v\n", seed, ratios)
	}
	t.Logf("ACEX V3 TERMINAL PASS fixed_seeds=%v runtimeSeed=%d", seeds, runtimeSeed)
}
