package ace

import (
	"fmt"
	"math/rand"
	"sort"
	"strconv"
)

// FrontierRegime identifies a task family without exposing its latent mechanism.
type FrontierRegime string

const (
	RegimeSymbolic FrontierRegime = "symbolic"
	RegimeRelational FrontierRegime = "relational"
	RegimeInteractive FrontierRegime = "interactive"
	RegimeCrossRegime FrontierRegime = "cross-regime"
)

// LatentTask is the evaluator-side representation. Latent is deliberately not
// copied into Public; it is only used by the independent oracle.
type LatentTask struct {
	ID        string
	Regime    FrontierRegime
	Public    Task
	Examples  []TaskExample
	HeldOut   []TaskExample
	Latent    string
	Seed      int64
}

type TaskExample struct {
	Input  map[string]string
	Output map[string]string
}

// FrontierTaskGenerator creates tasks from a deterministic latent program
// family. The caller chooses only the regime and seed; individual instances
// are generated from the latent parameters rather than authored test-by-test.
type FrontierTaskGenerator struct{}

func (FrontierTaskGenerator) Generate(regime FrontierRegime, seed int64) (LatentTask, error) {
	r := rand.New(rand.NewSource(seed))
	switch regime {
	case RegimeSymbolic:
		return generateSymbolic(r, seed), nil
	case RegimeRelational:
		return generateRelational(r, seed), nil
	case RegimeInteractive:
		return generateInteractive(r, seed), nil
	case RegimeCrossRegime:
		return generateCrossRegime(r, seed), nil
	default:
		return LatentTask{}, fmt.Errorf("unknown frontier regime %q", regime)
	}
}

func example(in, out int) TaskExample {
	return TaskExample{Input: map[string]string{"x": strconv.Itoa(in)}, Output: map[string]string{"y": strconv.Itoa(out)}}
}

func generateSymbolic(r *rand.Rand, seed int64) LatentTask {
	shift := r.Intn(9) + 2
	mode := r.Intn(2)
	f := func(x int) int {
		if mode == 0 {
			return (x + shift) * 2
		}
		if x%2 == 0 {
			return x/2 + shift
		}
		return x + shift
	}
	train := []TaskExample{example(2, f(2)), example(5, f(5)), example(8, f(8))}
	test := []TaskExample{example(3, f(3)), example(11, f(11)), example(14, f(14))}
	return latentTask(seed, RegimeSymbolic, "symbolic-shift-composition", train, test)
}

func generateRelational(r *rand.Rand, seed int64) LatentTask {
	shift := r.Intn(5) + 1
	// The public examples expose observations, not the latent relation rule.
	// The hidden evaluator uses a separately computed relational oracle.
	train := []TaskExample{
		{Input: map[string]string{"edge": "a->b", "value_a": "2"}, Output: map[string]string{"value_b": strconv.Itoa(2 + shift)}},
		{Input: map[string]string{"edge": "b->c", "value_b": "5"}, Output: map[string]string{"value_c": strconv.Itoa(5 + shift)}},
	}
	test := []TaskExample{
		{Input: map[string]string{"edge": "c->d", "value_c": "7"}, Output: map[string]string{"value_d": strconv.Itoa(7 + shift)}},
		{Input: map[string]string{"edge": "d->e", "value_d": "11"}, Output: map[string]string{"value_e": strconv.Itoa(11 + shift)}},
	}
	return latentTask(seed, RegimeRelational, "relational-edge-propagation", train, test)
}

func generateInteractive(r *rand.Rand, seed int64) LatentTask {
	bias := r.Intn(3) + 1
	train := []TaskExample{
		{Input: map[string]string{"state": "s0", "action": "probe-a"}, Output: map[string]string{"observation": "ambiguous"}},
		{Input: map[string]string{"state": "s0", "action": "probe-b"}, Output: map[string]string{"observation": "ambiguous"}},
	}
	test := []TaskExample{
		{Input: map[string]string{"state": "s1", "action": "probe-a"}, Output: map[string]string{"observation": strconv.Itoa(bias)}},
		{Input: map[string]string{"state": "s1", "action": "probe-b"}, Output: map[string]string{"observation": strconv.Itoa(bias + 1)}},
	}
	return latentTask(seed, RegimeInteractive, "intervention-disambiguation", train, test)
}

func generateCrossRegime(r *rand.Rand, seed int64) LatentTask {
	shift := r.Intn(4) + 2
	train := []TaskExample{
		{Input: map[string]string{"edge": "u->v", "value_u": "3"}, Output: map[string]string{"value_v": strconv.Itoa(3 + shift)}},
		{Input: map[string]string{"edge": "v->w", "value_v": "6"}, Output: map[string]string{"value_w": strconv.Itoa(6 + shift)}},
	}
	test := []TaskExample{
		{Input: map[string]string{"x": "9"}, Output: map[string]string{"y": strconv.Itoa(9 + shift)}},
		{Input: map[string]string{"x": "13"}, Output: map[string]string{"y": strconv.Itoa(13 + shift)}},
	}
	return latentTask(seed, RegimeCrossRegime, "reusable-increment-abstraction", train, test)
}

func latentTask(seed int64, regime FrontierRegime, latent string, train, test []TaskExample) LatentTask {
	id := Hash([]any{"frontier-task", regime, seed, latent})
	public := Task{
		ID: id,
		Goal: fmt.Sprintf("frontier:%s", id),
		Requirements: []string{"solve generated task"},
		Novel: true,
		Budget: ResourceVector{Compute: 100, Search: 100, ExperimentBudget: 20, Discovery: 50},
		Provenance: Prov("independent-task-generator", id, "latent-program", seed),
	}
	return LatentTask{ID: id, Regime: regime, Public: public, Examples: append([]TaskExample(nil), train...), HeldOut: append([]TaskExample(nil), test...), Latent: latent, Seed: seed}
}

// SortedRegimes gives deterministic evaluator ordering and prevents test
// authors from selecting an easy family by map iteration order.
func SortedRegimes() []FrontierRegime {
	out := []FrontierRegime{RegimeSymbolic, RegimeRelational, RegimeInteractive, RegimeCrossRegime}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
