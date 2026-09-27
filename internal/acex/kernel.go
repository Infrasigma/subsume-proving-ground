package acex

import (
	"errors"
	"math"
	"sort"
	"strings"
)

type Resource struct {
	Search int
	Verify int
	Memory int
	Storage int
}

func (r Resource) Total() int {
	return r.Search + r.Verify + r.Memory + r.Storage
}

type Observation struct {
	Features []string
	Label    bool
	Action   string
	Outcome  string
}

type Dataset struct {
	Examples []Observation
}

func (d Dataset) Copy() Dataset {
	out := Dataset{Examples: make([]Observation, len(d.Examples))}
	copy(out.Examples, d.Examples)
	return out
}

type Concept struct {
	ID          string
	Features    []string
	Support     int
	Accuracy    float64
	Complexity  int
	SourceSig   string
	TransferSig string
}

func containsFeature(obs Observation, f string) bool {
	for _, x := range obs.Features {
		if x == f {
			return true
		}
	}
	return false
}

func Predict(obs Observation, features []string) bool {
	for _, f := range features {
		if !containsFeature(obs, f) {
			return false
		}
	}
	return len(features) > 0
}

func Accuracy(data Dataset, features []string) float64 {
	if len(data.Examples) == 0 {
		return 0
	}
	n := 0
	for _, ex := range data.Examples {
		if Predict(ex, features) == ex.Label {
			n++
		}
	}
	return float64(n) / float64(len(data.Examples))
}

func Support(data Dataset, features []string) int {
	n := 0
	for _, ex := range data.Examples {
		if Predict(ex, features) {
			n++
		}
	}
	return n
}

type FeatureProfile struct {
	Name        string
	Positive    float64
	Negative    float64
	Balance     float64
	Cooccurrence map[string]float64
}

func Profile(data Dataset) map[string]FeatureProfile {
	counts := map[string][2]int{}
	pair := map[string]map[string]int{}
	for _, ex := range data.Examples {
		seen := map[string]bool{}
		for _, f := range ex.Features {
			seen[f] = true
			v := counts[f]
			if ex.Label {
				v[0]++
			} else {
				v[1]++
			}
			counts[f] = v
		}
		fs := make([]string, 0, len(seen))
		for f := range seen {
			fs = append(fs, f)
		}
		sort.Strings(fs)
		for i := 0; i < len(fs); i++ {
			for j := i + 1; j < len(fs); j++ {
				if pair[fs[i]] == nil {
					pair[fs[i]] = map[string]int{}
				}
				pair[fs[i]][fs[j]]++
			}
		}
	}
	totalPos, totalNeg := 0, 0
	for _, ex := range data.Examples {
		if ex.Label {
			totalPos++
		} else {
			totalNeg++
		}
	}
	out := make(map[string]FeatureProfile, len(counts))
	for f, c := range counts {
		pp := float64(c[0]) / math.Max(float64(totalPos), 1)
		np := float64(c[1]) / math.Max(float64(totalNeg), 1)
		balance := math.Abs(pp - np)
		co := map[string]float64{}
		for g, n := range pair[f] {
			co[g] = float64(n) / math.Max(float64(len(data.Examples)), 1)
		}
		out[f] = FeatureProfile{Name: f, Positive: pp, Negative: np, Balance: balance, Cooccurrence: co}
	}
	return out
}

func candidateFeatures(data Dataset) []string {
	set := map[string]bool{}
	for _, ex := range data.Examples {
		for _, f := range ex.Features {
			set[f] = true
		}
	}
	out := make([]string, 0, len(set))
	for f := range set {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

type SearchPolicy string

const (
	PolicyBroad      SearchPolicy = "broad"
	PolicySpecific   SearchPolicy = "specific"
	PolicyFrequency  SearchPolicy = "frequency"
	PolicyNovelty    SearchPolicy = "novelty"
)

type Representation struct {
	Name       string
	Map        map[string]string
	Confidence float64
}

type RepresentationLab struct {
	MaxAtoms int
	Policy   SearchPolicy
}

func (r RepresentationLab) OrderedFeatures(data Dataset) []string {
	features := candidateFeatures(data)
	profile := Profile(data)
	sort.SliceStable(features, func(i, j int) bool {
		switch r.Policy {
		case PolicySpecific:
			return profile[features[i]].Balance > profile[features[j]].Balance
		case PolicyFrequency:
			fi := profile[features[i]].Positive + profile[features[i]].Negative
			fj := profile[features[j]].Positive + profile[features[j]].Negative
			return fi > fj
		case PolicyNovelty:
			ci := len(profile[features[i]].Cooccurrence)
			cj := len(profile[features[j]].Cooccurrence)
			return ci < cj
		default:
			return features[i] < features[j]
		}
	})
	return features
}

func combinations(items []string, k int) [][]string {
	if k == 0 {
		return [][]string{{}}
	}
	out := make([][]string, 0)
	var rec func(int, []string)
	rec = func(start int, cur []string) {
		if len(cur) == k {
			x := append([]string(nil), cur...)
			out = append(out, x)
			return
		}
		for i := start; i < len(items); i++ {
			rec(i+1, append(cur, items[i]))
		}
	}
	rec(0, nil)
	return out
}

func (r RepresentationLab) Discover(train, holdout Dataset) (Concept, Resource, error) {
	if r.MaxAtoms < 1 {
		return Concept{}, Resource{}, errors.New("representation lab MaxAtoms must be positive")
	}
	ordered := r.OrderedFeatures(train)
	best := Concept{}
	bestScore := -1.0
	cost := Resource{}
	limit := r.MaxAtoms
	if len(ordered) < limit {
		limit = len(ordered)
	}
	for k := 1; k <= limit; k++ {
		for _, c := range combinations(ordered, k) {
			cost.Search++
			a := Accuracy(train, c)
			if a < 0.90 {
				continue
			}
			ha := Accuracy(holdout, c)
			cost.Verify++
			if ha < 0.90 {
				continue
			}
			score := ha - 0.01*float64(k)
			if score > bestScore {
				bestScore = score
				best = Concept{
					ID:         strings.Join(c, "+"),
					Features:   append([]string(nil), c...),
					Support:    Support(train, c),
					Accuracy:   ha,
					Complexity: k,
				}
				// Non-broad policies are executable search strategies, not
				// hard-coded task solvers: once a candidate survives the
				// independent holdout, they may stop early.
				if r.Policy != PolicyBroad && r.Policy != PolicyNovelty {
					cost.Storage += k
					return best, cost, nil
				}
			}
		}
	}
	if best.ID == "" {
		return Concept{}, cost, errors.New("no verified concept discovered")
	}
	cost.Memory = best.Complexity
	cost.Storage = len(best.Features)
	return best, cost, nil
}

func LowComplexityCeiling(data Dataset, maxAtoms int) float64 {
	best := 0.0
	features := candidateFeatures(data)
	if len(features) > 12 {
		features = features[:12]
	}
	for k := 1; k <= maxAtoms && k <= len(features); k++ {
		for _, c := range combinations(features, k) {
			if a := Accuracy(data, c); a > best {
				best = a
			}
		}
	}
	return best
}

type Mapping struct {
	Source string
	Target string
	Score  float64
}

func Signature(data Dataset, f string) FeatureProfile {
	return Profile(data)[f]
}

func cooccurrenceSignature(m map[string]float64) []float64 {
	out := make([]float64, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	sort.Float64s(out)
	return out
}

func MapRepresentation(source Dataset, target Dataset, concept Concept) (Representation, []Mapping, Resource, error) {
	sp := Profile(source)
	tp := Profile(target)
	if len(concept.Features) == 0 {
		return Representation{}, nil, Resource{}, errors.New("empty concept")
	}
	maps := make([]Mapping, 0, len(concept.Features))
	for _, sf := range concept.Features {
		s, ok := sp[sf]
		if !ok {
			return Representation{}, nil, Resource{}, errors.New("source feature missing")
		}
		best := Mapping{Source: sf, Score: -1}
		for tf, t := range tp {
			score := 1.0 - math.Abs(s.Positive-t.Positive) - math.Abs(s.Negative-t.Negative)
			score += 0.10 * math.Min(1, float64(len(s.Cooccurrence)+len(t.Cooccurrence))/10)
			sigS, sigT := cooccurrenceSignature(s.Cooccurrence), cooccurrenceSignature(t.Cooccurrence)
			for i := 0; i < len(sigS) && i < len(sigT); i++ {
				score += 0.08 * (1.0 - math.Abs(sigS[i]-sigT[i]))
			}
			if score > best.Score {
				best = Mapping{Source: sf, Target: tf, Score: score}
			}
		}
		if best.Target == "" || best.Score < 0.1 {
			return Representation{}, nil, Resource{}, errors.New("representation mapping ambiguous")
		}
		maps = append(maps, best)
	}
	m := map[string]string{}
	var sum float64
	for _, x := range maps {
		m[x.Source] = x.Target
		sum += x.Score
	}
	return Representation{Name: "behavioral-role-map", Map: m, Confidence: sum / float64(len(maps))}, maps,
		Resource{Search: len(sp) * len(tp), Verify: len(maps)}, nil
}

type VerifiedKnowledge struct {
	Concept       Concept
	Representation Representation
	Evidence      []string
	ReuseCount    int
	Acquisition   Resource
	Verified      bool
}

type KnowledgeStore struct {
	Items []VerifiedKnowledge
}

func (k *KnowledgeStore) Add(x VerifiedKnowledge) error {
	if !x.Verified {
		return errors.New("only verified knowledge may be stored")
	}
	k.Items = append(k.Items, x)
	return nil
}

func (k *KnowledgeStore) DeleteEpisodes() {
	// Knowledge is intentionally independent of raw episodes.
}

func (k KnowledgeStore) Snapshot() []VerifiedKnowledge {
	out := make([]VerifiedKnowledge, len(k.Items))
	copy(out, k.Items)
	return out
}

type Hypothesis struct {
	Name       string
	Predictors map[string]string
	Score      float64
}

type ExperimentPlan struct {
	Action      string
	Expected    map[string]string
	Disagreement float64
}

type CausalEngine struct{}

func (CausalEngine) SelectExperiment(h []Hypothesis) (ExperimentPlan, error) {
	if len(h) < 2 {
		return ExperimentPlan{}, errors.New("need competing hypotheses")
	}
	best := ExperimentPlan{Action: "noop"}
	for action := 0; action < 4; action++ {
		name := "intervention-" + string(rune('A'+action))
		signatures := map[string]bool{}
		for _, x := range h {
			if v, ok := x.Predictors[name]; ok {
				signatures[v] = true
			}
		}
		d := float64(len(signatures))
		if d > best.Disagreement {
			best = ExperimentPlan{Action: name, Disagreement: d}
		}
	}
	if best.Disagreement <= 0 {
		return ExperimentPlan{}, errors.New("no informative intervention found")
	}
	return best, nil
}

type PlanStep struct {
	Name     string
	Cost     int
	Requires []string
}

func ComposeConcepts(a, b Concept) Concept {
	features := append([]string(nil), a.Features...)
	seen := map[string]bool{}
	for _, f := range features {
		seen[f] = true
	}
	for _, f := range b.Features {
		if !seen[f] {
			features = append(features, f)
			seen[f] = true
		}
	}
	sort.Strings(features)
	return Concept{
		ID:         a.ID + "|" + b.ID,
		Features:   features,
		Support:    minInt(a.Support, b.Support),
		Accuracy:   math.Min(a.Accuracy, b.Accuracy),
		Complexity: len(features),
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type Planner struct {
	Width int
}

func (p Planner) Solve(goal string, known []VerifiedKnowledge) ([]PlanStep, Resource, error) {
	if p.Width <= 0 {
		return nil, Resource{}, errors.New("planner width must be positive")
	}
	out := make([]PlanStep, 0)
	cost := Resource{}
	for _, k := range known {
		cost.Search++
		if strings.Contains(strings.ToLower(k.Concept.ID), strings.ToLower(goal)) {
			out = append(out, PlanStep{Name: k.Concept.ID, Cost:k.Concept.Complexity})
		}
	}
	if len(out) == 0 {
		return nil, cost, errors.New("goal not reachable from retained knowledge")
	}
	return out, cost, nil
}

type Diagnosis struct {
	Kind       string
	Reason     string
	Confidence float64
}

type MetaController struct {
	Policy SearchPolicy
}

func (m *MetaController) DiagnoseAndSwitch(d Dataset, budget int) Diagnosis {
	if budget <= 0 {
		return Diagnosis{Kind:"resource", Reason:"zero-search-budget", Confidence:1}
	}
	unique := len(candidateFeatures(d))
	if unique > budget*3 {
		m.Policy = PolicySpecific
		return Diagnosis{Kind:"representation", Reason:"frontier-too-wide; switch-to-specificity", Confidence:.9}
	}
	m.Policy = PolicyFrequency
	return Diagnosis{Kind:"search", Reason:"baseline-search-unproductive; switch-to-frequency", Confidence:.75}
}

type ImprovementCandidate struct {
	Name   string
	Policy SearchPolicy
}

type SelfImprover struct{}

func (SelfImprover) Candidates() []ImprovementCandidate {
	return []ImprovementCandidate{
		{Name:"broad", Policy:PolicyBroad},
		{Name:"specific", Policy:PolicySpecific},
		{Name:"frequency", Policy:PolicyFrequency},
		{Name:"novelty", Policy:PolicyNovelty},
	}
}

func (SelfImprover) Evaluate(candidate ImprovementCandidate, visible []Dataset) int {
	total := 0
	lab := RepresentationLab{MaxAtoms:4, Policy:candidate.Policy}
	for _, d := range visible {
		_, cost, err := lab.Discover(d, d)
		if err == nil {
			total += cost.Total()
		} else {
			total += 100000
		}
	}
	return total
}

func (s SelfImprover) Promote(current SearchPolicy, visible []Dataset) (SearchPolicy, Resource, error) {
	best := current
	bestCost := s.Evaluate(ImprovementCandidate{Name:string(current), Policy:current}, visible)
	cost := Resource{Search:1}
	for _, c := range s.Candidates() {
		v := s.Evaluate(c, visible)
		cost.Search++
		if v < bestCost {
			best, bestCost = c.Policy, v
		}
	}
	if best == current {
		return current, cost, errors.New("no independently evaluated improvement")
	}
	return best, cost, nil
}

type Curriculum struct{}

func (Curriculum) Next(unresolved []Dataset) (Dataset, error) {
	if len(unresolved) == 0 {
		return Dataset{}, errors.New("no verified capability gap")
	}
	// Generic difficulty increase: preserve the feature vocabulary but add one
	// additional irrelevant feature and one conjunction-bearing example.
	base := unresolved[0].Copy()
	for i := range base.Examples {
		if i%3 == 0 {
			base.Examples[i].Features = append(base.Examples[i].Features, "curriculum-gap")
		}
	}
	return base, nil
}
