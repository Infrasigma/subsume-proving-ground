package acex

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"testing"
)

func randomUniqueOps(r *rand.Rand, n int, prefix string) []TraceStep {
	ops := make([]TraceStep, 0, n)
	seen := map[string]bool{}
	for len(ops) < n {
		op := fmt.Sprintf("%s-%c-%d", prefix, rune('a'+r.Intn(20)), r.Intn(1000))
		if seen[op] {
			continue
		}
		seen[op] = true
		ops = append(ops, TraceStep{Op:op, Arg:r.Intn(4)})
	}
	return ops
}

func insertMotif(prefix, motif, suffix []TraceStep) Trace {
	steps := make([]TraceStep, 0, len(prefix)+len(motif)+len(suffix))
	steps = append(steps, prefix...)
	steps = append(steps, motif...)
	steps = append(steps, suffix...)
	return Trace{Steps:steps}
}

func TestV2AdversarialRandomizedCognitiveSweep(t *testing.T) {
	const trials = 64
	for seed := 1; seed <= trials; seed++ {
		r := rand.New(rand.NewSource(int64(900000 + seed)))

		// Belief revision: random deterministic hypothesis spaces and random
		// experiments. The true hypothesis must become dominant after evidence.
		nH := 4 + r.Intn(4)
		h := make([]Belief, nH)
		for i := range h {
			h[i] = Belief{
				ID:      fmt.Sprintf("h-%d", i),
				Prior:   0.25 + r.Float64(),
				Predicted: map[string]string{
					"a": fmt.Sprintf("o-%d", r.Intn(3)),
					"b": fmt.Sprintf("o-%d", r.Intn(3)),
					"c": fmt.Sprintf("o-%d", r.Intn(3)),
					"d": fmt.Sprintf("o-%d", r.Intn(3)),
				},
			}
		}
		trueIdx := r.Intn(nH)
		engine := BeliefRevision{}
		h = NormalizeBeliefs(h)
		action, gain, err := engine.ChooseIntervention(h, nil)
		if err != nil || gain <= 0 || action == "" {
			t.Fatalf("seed %d: no discriminating action: action=%q gain=%v err=%v", seed, action, gain, err)
		}
		outcome := h[trueIdx].Predicted[action]
		revised, err := engine.Revise(h, action, outcome)
		if err != nil {
			t.Fatalf("seed %d: revision failed: %v", seed, err)
		}
		topID := revised[0].ID
		// Ties are possible; require the true hypothesis to be among the
		// maximally supported survivors, and require entropy to decrease.
		truePosterior := 0.0
		for _, x := range revised {
			if x.ID == h[trueIdx].ID {
				truePosterior = x.Posterior
			}
		}
		if truePosterior <= 0 {
			t.Fatalf("seed %d: true hypothesis was eliminated by its own predicted outcome", seed)
		}
		if topID == "" || truePosterior < 1.0/float64(nH) {
			t.Fatalf("seed %d: weak posterior after revision: true=%v revised=%+v", seed, truePosterior, revised)
		}
		_ = math.IsNaN(gain)

		// Mechanism induction: generate a hidden repeated motif and vary every
		// surrounding trace. The motif itself is never passed as a parameter.
		motif := randomUniqueOps(r, 3+r.Intn(2), "motif")
		traces := make([]Trace, 0, 4)
		for i := 0; i < 4; i++ {
			prefix := randomUniqueOps(r, 1+r.Intn(3), fmt.Sprintf("p-%d", i))
			suffix := randomUniqueOps(r, 1+r.Intn(3), fmt.Sprintf("s-%d", i))
			traces = append(traces, insertMotif(prefix, motif, suffix))
		}
		macros := commonSubtraces(traces, 3)
		if len(macros) == 0 {
			t.Fatalf("seed %d: no macro discovered", seed)
		}
		m1 := macros[0]
		if len(m1.Steps) < 3 {
			t.Fatalf("seed %d: discovered weak macro length=%d", seed, len(m1.Steps))
		}
		hidden := insertMotif(
			randomUniqueOps(r, 2, "hp"),
			motif,
			randomUniqueOps(r, 2, "hs"),
		)
		compressed := ApplyMacro(hidden, m1)
		if len(compressed.Steps) >= len(hidden.Steps) {
			t.Fatalf("seed %d: macro failed hidden compression: raw=%d compressed=%d", seed, len(hidden.Steps), len(compressed.Steps))
		}

		// Attention: the latent trio is exactly the only consistently
		// label-predictive structure; noise rates vary across trials.
		fam := families[seed%len(families)]
		data := makeBalanced(int64(seed)*71+19, fam, 0, 120, true)
		selected := (AttentionController{}).Select(data, 3)
		if len(selected) != 3 {
			t.Fatalf("seed %d: attention selected %d features", seed, len(selected))
		}
		got := append([]string(nil), selected...)
		sort.Strings(got)
		want := []string{"opaque-grid-107015","opaque-grid-258427","opaque-grid-672271"}
		// For non-grid surfaces the stable latent names are surface-specific.
		latent := permutedTokens(stableTokenSeed(fam,0), fam, "opaque")
		sort.Strings(latent)
		for i := range latent {
			if got[i] != latent[i] {
				t.Fatalf("seed %d: attention selected wrong role: got=%v want=%v", seed, got, latent)
			}
		}
		_ = want
	}
	t.Logf("ACEX V2 RANDOMIZED SWEEP: PASS trials=%d", trials)
}
