package acex

import (
	"fmt"
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

		// Belief revision: generate distinct hypothesis signatures. The
		// experimenter must repeatedly intervene and revise until the true
		// hypothesis becomes uniquely identified.
		nH := 4 + r.Intn(4)
		h := make([]Belief, 0, nH)
		signatures := map[string]bool{}
		for len(h) < nH {
			preds := map[string]string{
				"a": fmt.Sprintf("o-%d", r.Intn(3)),
				"b": fmt.Sprintf("o-%d", r.Intn(3)),
				"c": fmt.Sprintf("o-%d", r.Intn(3)),
				"d": fmt.Sprintf("o-%d", r.Intn(3)),
			}
			sig := fmt.Sprintf("%s|%s|%s|%s", preds["a"], preds["b"], preds["c"], preds["d"])
			if signatures[sig] {
				continue
			}
			signatures[sig] = true
			h = append(h, Belief{
				ID:        fmt.Sprintf("h-%d", len(h)),
				Prior:     0.25 + r.Float64(),
				Predicted: preds,
			})
		}
		trueIdx := r.Intn(nH)
		trueID := h[trueIdx].ID
		engine := BeliefRevision{}
		h = NormalizeBeliefs(h)
		used := map[string]bool{}
		steps := 0
		for steps < 4 {
			filtered := make([]Belief, len(h))
			for i, x := range h {
				filtered[i] = x
				filtered[i].Predicted = map[string]string{}
				for action, outcome := range h[i].Predicted {
					if !used[action] {
						filtered[i].Predicted[action] = outcome
					}
				}
			}
			action, gain, err := engine.ChooseIntervention(filtered, nil)
			if err != nil || gain <= 0 || action == "" {
				t.Fatalf("seed %d: sequential experimenter got stuck: action=%q gain=%v err=%v post=%+v", seed, action, gain, err, h)
			}
			used[action] = true
			outcome := ""
			for _, x := range h {
				if x.ID == trueID {
					outcome = x.Predicted[action]
					break
				}
			}
			if outcome == "" {
				t.Fatalf("seed %d: true hypothesis lost before intervention %s", seed, action)
			}
			revised, err := engine.Revise(h, action, outcome)
			if err != nil {
				t.Fatalf("seed %d: revision failed: %v", seed, err)
			}
			h = revised
			steps++
			truePosterior := 0.0
			for _, x := range h {
				if x.ID == trueID {
					truePosterior = x.Posterior
				}
			}
			if truePosterior > 0.999 {
				break
			}
		}
		truePosterior := 0.0
		for _, x := range h {
			if x.ID == trueID {
				truePosterior = x.Posterior
			}
		}
		if truePosterior <= 0.999 {
			t.Fatalf("seed %d: sequential evidence did not identify true hypothesis after %d interventions: posterior=%v", seed, steps, truePosterior)
		}

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
