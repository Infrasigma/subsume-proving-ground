package acex

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sort"
)

// V5Mechanism is the smallest mutable unit of cognition in V5. A promoted
// mechanism changes how the substrate searches/retrieves/inquires; it is not a
// new task-specific concept.
type V5Mechanism struct {
	Name            string
	SearchPolicy    SearchPolicy
	MaxAtoms        int
	AttentionBudget int
	RetrievalLimit  int
	Intervention   string
	Abstraction    string
}

func (m V5Mechanism) Key() string {
	return fmt.Sprintf("%s|search=%s|max=%d|attention=%d|retrieve=%d|intervention=%s|abstraction=%s",
		m.Name, m.SearchPolicy, m.MaxAtoms, m.AttentionBudget, m.RetrievalLimit, m.Intervention, m.Abstraction)
}

type V5MemoryTrace struct {
	ID            string
	Context       []string
	PredictionErr float64
	Failure       bool
	Utility       float64
	Verified      bool
}

type V5AdaptiveMemory struct {
	Items []V5MemoryTrace
}

func (m *V5AdaptiveMemory) Record(x V5MemoryTrace) error {
	if x.ID == "" {
		return errors.New("memory trace requires id")
	}
	if x.PredictionErr < 0 || x.PredictionErr > 1 {
		return errors.New("prediction error out of range")
	}
	m.Items = append(m.Items, x)
	return nil
}

func v5TokenOverlap(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	set := map[string]bool{}
	for _, x := range a {
		set[x] = true
	}
	hit := 0
	for _, x := range b {
		if set[x] {
			hit++
		}
	}
	return float64(hit) / math.Max(float64(len(set)), float64(len(b)))
}

func (m V5AdaptiveMemory) Retrieve(context []string, limit int) []V5MemoryTrace {
	if limit <= 0 {
		return nil
	}
	type scored struct {
		item  V5MemoryTrace
		score float64
	}
	out := make([]scored, 0, len(m.Items))
	for _, item := range m.Items {
		sim := v5TokenOverlap(item.Context, context)
		score := 0.55*sim + 0.25*item.PredictionErr + 0.15*item.Utility
		if item.Failure {
			score += 0.15
		}
		if item.Verified {
			score += 0.10
		}
		if score > 0 {
			out = append(out, scored{item: item, score: score})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score
		}
		return out[i].item.ID < out[j].item.ID
	})
	if len(out) > limit {
		out = out[:limit]
	}
	result := make([]V5MemoryTrace, 0, len(out))
	for _, x := range out {
		result = append(result, x.item)
	}
	return result
}

type V5Hypothesis struct {
	ID       string
	Prior    float64
	Outcome  map[string]string
	Survival float64
}

type V5InterventionResult struct {
	Action       string
	Disagreement float64
	ExpectedGain float64
}

func V5ChooseIntervention(h []V5Hypothesis) (V5InterventionResult, error) {
	if len(h) < 2 {
		return V5InterventionResult{}, errors.New("need at least two competing hypotheses")
	}
	totalPrior := 0.0
	for _, x := range h {
		if x.Prior > 0 {
			totalPrior += x.Prior
		}
	}
	if totalPrior <= 0 {
		totalPrior = float64(len(h))
	}
	baseH := 0.0
	priors := make([]float64, len(h))
	for i, x := range h {
		p := x.Prior
		if p <= 0 {
			p = 1
		}
		priors[i] = p / totalPrior
		baseH -= priors[i] * math.Log2(priors[i])
	}
	actions := map[string]bool{}
	for _, x := range h {
		for action := range x.Outcome {
			actions[action] = true
		}
	}
	best := V5InterventionResult{}
	for action := range actions {
		mass := map[string]float64{}
		for i, x := range h {
			if outcome, ok := x.Outcome[action]; ok {
				mass[outcome] += priors[i]
			}
		}
		if len(mass) < 2 {
			continue
		}
		expectedH := 0.0
		for outcome, pOutcome := range mass {
			if pOutcome <= 0 {
				continue
			}
			postH := 0.0
			for i, x := range h {
				if x.Outcome[action] != outcome {
					continue
				}
				p := priors[i] / pOutcome
				if p > 0 {
					postH -= p * math.Log2(p)
				}
			}
			_ = outcome
			expectedH += pOutcome * postH
		}
		gain := baseH - expectedH
		disagreement := float64(len(mass))
		if disagreement > best.Disagreement ||
			(disagreement == best.Disagreement && gain > best.ExpectedGain) ||
			(disagreement == best.Disagreement && gain == best.ExpectedGain && action < best.Action) {
			best = V5InterventionResult{Action: action, Disagreement: disagreement, ExpectedGain: gain}
		}
	}
	if best.Action == "" || best.ExpectedGain <= 0 {
		return V5InterventionResult{}, errors.New("no informative intervention")
	}
	return best, nil
}

func V5ReviseHypotheses(h []V5Hypothesis, action, observed string) ([]V5Hypothesis, error) {
	out := make([]V5Hypothesis, 0, len(h))
	total := 0.0
	for _, x := range h {
		if x.Outcome[action] != observed {
			continue
		}
		x.Survival = math.Max(x.Prior, 1e-9)
		out = append(out, x)
		total += x.Survival
	}
	if len(out) == 0 {
		return nil, errors.New("observation falsified all hypotheses")
	}
	for i := range out {
		out[i].Survival /= total
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Survival != out[j].Survival {
			return out[i].Survival > out[j].Survival
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// V5Task deliberately contains no "family" field visible to a learner. The
// field exists only for evaluator telemetry so successor surfaces can be kept
// independent and disjoint.
type V5Task struct {
	ID       string
	Train    Dataset
	Holdout  Dataset
	FamilyID string
}

type V5TaskResult struct {
	TaskID     string
	Verified   bool
	Cost       Resource
	Accuracy   float64
	Mechanism  string
}

type V5MechanismResult struct {
	Mechanism V5Mechanism
	Tasks     []V5TaskResult
	TotalCost int
	Verified  bool
}

func V5EvaluateMechanism(mech V5Mechanism, tasks []V5Task) V5MechanismResult {
	results := make([]V5TaskResult, 0, len(tasks))
	total := 0
	all := len(tasks) > 0
	for _, task := range tasks {
		lab := RepresentationLab{MaxAtoms: mech.MaxAtoms, Policy: mech.SearchPolicy}
		concept, cost, err := lab.Discover(task.Train, task.Holdout)
		verified := err == nil && concept.Accuracy >= 0.90
		if mech.AttentionBudget > 0 {
			cost.Search += len(candidateFeatures(task.Train)) / mech.AttentionBudget
		}
		if mech.RetrievalLimit > 0 && mech.RetrievalLimit < 4 {
			cost.Memory += 4 - mech.RetrievalLimit
		}
		if !verified {
			all = false
		}
		total += cost.Total()
		results = append(results, V5TaskResult{
			TaskID: task.ID, Verified: verified, Cost: cost,
			Accuracy: concept.Accuracy, Mechanism: mech.Key(),
		})
	}
	return V5MechanismResult{Mechanism: mech, Tasks: results, TotalCost: total, Verified: all}
}

func V5MechanismCandidates(base V5Mechanism, gap string) []V5Mechanism {
	policies := []SearchPolicy{PolicySpecific, PolicyFrequency, PolicyNovelty, PolicyBroad}
	out := make([]V5Mechanism, 0, len(policies)*4)
	seen := map[string]bool{}
	for _, policy := range policies {
		for _, attention := range []int{2, 4, 8} {
			for _, retrieve := range []int{1, 2, 4} {
				x := base
				x.SearchPolicy = policy
				x.AttentionBudget = attention
				x.RetrievalLimit = retrieve
				switch gap {
				case "representation":
					x.Abstraction = "semantic-equivalence"
				case "transfer":
					x.Intervention = "surface-shift"
				case "model":
					x.Intervention = "counterfactual"
				default:
					x.Abstraction = "typed-compression"
				}
				x.Name = fmt.Sprintf("v5-candidate-%s-%d-%d", policy, attention, retrieve)
				if !seen[x.Key()] {
					seen[x.Key()] = true
					out = append(out, x)
				}
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Key() < out[j].Key() })
	return out
}

type V5PromotionReceipt struct {
	ParentVersion uint64
	NewVersion    uint64
	ParentKey     string
	NewKey        string
	BeforeCost    int
	AfterCost     int
	HiddenRatios  []float64
	Verified      bool
	RollbackKey   string
}

type V5MechanismRuntime struct {
	Active        V5Mechanism
	History       []V5Mechanism
	Version       uint64
	Promotions    []V5PromotionReceipt
	Memory        V5AdaptiveMemory
	Failures      []string
	Frontier      []string
}

func NewV5MechanismRuntime() *V5MechanismRuntime {
	return &V5MechanismRuntime{
		Active: V5Mechanism{
			Name:            "v5-baseline",
			SearchPolicy:    PolicyBroad,
			MaxAtoms:        4,
			AttentionBudget: 8,
			RetrievalLimit: 4,
			Intervention:   "information-gain",
			Abstraction:     "normalized-structure",
		},
	}
}

func v5NoRegression(a, b V5MechanismResult) bool {
	if len(a.Tasks) != len(b.Tasks) {
		return false
	}
	for i := range a.Tasks {
		if !a.Tasks[i].Verified || !b.Tasks[i].Verified {
			return false
		}
		if b.Tasks[i].Accuracy+1e-12 < a.Tasks[i].Accuracy {
			return false
		}
	}
	return true
}

func (r *V5MechanismRuntime) TryPromote(c V5Mechanism, visible, hidden []V5Task) error {
	if r == nil {
		return errors.New("nil V5 runtime")
	}
	if c.Key() == r.Active.Key() {
		return errors.New("candidate mechanism unchanged")
	}
	baseVisible := V5EvaluateMechanism(r.Active, visible)
	candVisible := V5EvaluateMechanism(c, visible)
	if !baseVisible.Verified || !candVisible.Verified || candVisible.TotalCost >= baseVisible.TotalCost {
		return errors.New("candidate does not improve verified visible cognition")
	}
	baseHidden := V5EvaluateMechanism(r.Active, hidden)
	candHidden := V5EvaluateMechanism(c, hidden)
	if !baseHidden.Verified || !candHidden.Verified {
		return errors.New("candidate or baseline failed independent hidden suite")
	}
	if !v5NoRegression(baseHidden, candHidden) {
		return errors.New("candidate regressed hidden capability")
	}
	ratios := make([]float64, len(hidden))
	for i := range hidden {
		ratios[i] = float64(candHidden.Tasks[i].Cost.Total()) /
			math.Max(float64(baseHidden.Tasks[i].Cost.Total()), 1)
		if ratios[i] >= 0.80 {
			return fmt.Errorf("hidden mechanism ratio %.3f >= 0.80 on %s", ratios[i], hidden[i].ID)
		}
	}
	receipt := V5PromotionReceipt{
		ParentVersion: r.Version,
		NewVersion:    r.Version + 1,
		ParentKey:     r.Active.Key(),
		NewKey:        c.Key(),
		BeforeCost:    baseVisible.TotalCost,
		AfterCost:     candVisible.TotalCost,
		HiddenRatios:  append([]float64(nil), ratios...),
		Verified:      true,
		RollbackKey:   r.Active.Key(),
	}
	r.History = append(r.History, r.Active)
	r.Active = c
	r.Version++
	r.Promotions = append(r.Promotions, receipt)
	return nil
}

func (r *V5MechanismRuntime) Rollback() error {
	if r == nil || len(r.History) == 0 {
		return errors.New("no V5 mechanism rollback")
	}
	r.Active = r.History[len(r.History)-1]
	r.History = r.History[:len(r.History)-1]
	if r.Version > 0 {
		r.Version--
	}
	return nil
}

type V5Challenge struct {
	ID        string
	Gap       string
	Difficulty float64
	Transform string
}

func V5NextChallenge(failures []string, seed int64) V5Challenge {
	gap := "novel-composition"
	if len(failures) > 0 {
		switch failures[len(failures)-1] {
		case "representation":
			gap = "representation"
		case "transfer":
			gap = "transfer"
		case "model":
			gap = "model"
		}
	}
	r := rand.New(rand.NewSource(seed))
	transforms := []string{"rename", "permute", "distract", "countertest", "compose"}
	t := transforms[r.Intn(len(transforms))]
	return V5Challenge{
		ID: fmt.Sprintf("v5-challenge-%d-%s", seed, gap),
		Gap: gap, Difficulty: 1.0 + 0.25*float64(len(failures)),
		Transform: t,
	}
}

// V5IndependentSuite is evaluator-owned. It deliberately does not consume a
// candidate's learned concepts or library. The latent task construction stays
// outside the runtime API, so a mechanism cannot influence the hidden target.
func V5IndependentSuite(seed int64, block int) []V5Task {
	r := rand.New(rand.NewSource(seed + int64(block)*7919))
	tasks := make([]V5Task, 0, 6)
	for i := 0; i < 6; i++ {
		count := 96 + r.Intn(48)
		family := i % 3
		perm := make([]string, 3)
		for j := range perm {
			perm[j] = fmt.Sprintf("opaque-%d-%d", r.Intn(1000000), family)
		}
		sort.Strings(perm)
		makeSet := func(extra int) Dataset {
			pos := make([]Observation, 0, count/2)
			neg := make([]Observation, 0, count/2)
			for len(pos) < count/2 || len(neg) < count/2 {
				label := len(pos) < count/2 && (len(neg) >= count/2 || r.Float64() < 0.5)
				fs := make([]string, 0, 8)
				switch family {
				case 0:
					if label {
						fs = append(fs, perm...)
					} else {
						fs = append(fs, perm[0], perm[1])
					}
				case 1:
					if label {
						fs = append(fs, perm...)
					} else {
						fs = append(fs, perm[1], perm[2])
					}
				default:
					if label {
						fs = append(fs, perm...)
					} else {
						fs = append(fs, perm[0], perm[2])
					}
				}
				for n := 0; n < extra; n++ {
					fs = append(fs, fmt.Sprintf("noise-%d-%d", n, r.Intn(12)))
				}
				sort.Strings(fs)
				ex := Observation{Features: fs, Label: label}
				if label {
					pos = append(pos, ex)
				} else {
					neg = append(neg, ex)
				}
			}
			return Dataset{Examples: append(pos, neg...)}
		}
		train := makeSet(2)
		hold := makeSet(2)
		tasks = append(tasks, V5Task{
			ID:       fmt.Sprintf("v5-independent-%d-%d", block, i),
			Train:    train,
			Holdout:  hold,
			FamilyID: fmt.Sprintf("f%d", family),
		})
	}
	return tasks
}
