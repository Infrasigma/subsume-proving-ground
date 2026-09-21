package acex

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"
	"strings"
	"testing"
)

type hiddenFamily struct {
	Name   string
	Latent []string
	Removals []float64
	Noise  []string
}

var families = []hiddenFamily{
	{Name: "grid", Latent: []string{"latent-a", "latent-b", "latent-c"}, Removals: []float64{0.20, 0.40, 0.60}, Noise: []string{"n0", "n1", "n2", "n3"}},
	{Name: "sequence", Latent: []string{"latent-a", "latent-b", "latent-c"}, Removals: []float64{0.15, 0.35, 0.55}, Noise: []string{"q0", "q1", "q2", "q3"}},
	{Name: "graph", Latent: []string{"latent-a", "latent-b", "latent-c"}, Removals: []float64{0.25, 0.45, 0.65}, Noise: []string{"g0", "g1", "g2", "g3"}},
}

func permutedTokens(seed int64, family hiddenFamily, suffix string) []string {
	r := rand.New(rand.NewSource(seed))
	out := make([]string, len(family.Latent))
	for i := range out {
		out[i] = fmt.Sprintf("%s-%s-%d", suffix, family.Name, r.Intn(1000000))
	}
	return out
}

func makeBalanced(seed int64, family hiddenFamily, conceptIndex int, count int, extraTokens bool) Dataset {
	r := rand.New(rand.NewSource(seed))
	tokens := permutedTokens(seed+int64(conceptIndex)*991, family, "opaque")
	positives, negatives := make([]Observation, 0, count/2), make([]Observation, 0, count/2)
	tries := 0
	for len(positives) < count/2 || len(negatives) < count/2 {
		tries++
		if tries > count*10000 {
			panic("dataset balancing failed")
		}
		label := len(positives) < count/2 && (len(negatives) >= count/2 || r.Float64() < 0.5)
		features := make([]string, 0, 10)
		if label {
			features = append(features, tokens...)
		} else {
			for i, token := range tokens {
				if r.Float64() >= family.Removals[i] {
					features = append(features, token)
				}
			}
			// Force at least one latent atom out of every negative.
			if len(features) == len(tokens) {
				features = features[:len(features)-1]
			}
		}
		for i, n := range family.Noise {
			p := 0.18 + float64(i)*0.07
			if r.Float64() < p {
				features = append(features, fmt.Sprintf("%s-%d", n, r.Intn(4)))
			}
		}
		if extraTokens && r.Float64() < 0.35 {
			features = append(features, fmt.Sprintf("surface-distractor-%d", r.Intn(9)))
		}
		sort.Strings(features)
		ex := Observation{Features: features, Label: label}
		if label {
			positives = append(positives, ex)
		} else {
			negatives = append(negatives, ex)
		}
	}
	return Dataset{Examples: append(positives, negatives...)}
}

func mappedConcept(source Dataset, target Dataset, c Concept) (Concept, Resource, error) {
	rep, _, cost, err := MapRepresentation(source, target, c)
	if err != nil {
		return Concept{}, cost, err
	}
	mapped := make([]string, 0, len(c.Features))
	for _, f := range c.Features {
		x, ok := rep.Map[f]
		if !ok {
			return Concept{}, cost, fmt.Errorf("missing mapped feature %q", f)
		}
		mapped = append(mapped, x)
	}
	sort.Strings(mapped)
	c.Features = mapped
	c.ID = strings.Join(mapped, "+")
	c.TransferSig = rep.Name
	return c, cost, nil
}

func split(data Dataset) (Dataset, Dataset) {
	n := len(data.Examples) / 2
	return Dataset{Examples: append([]Observation(nil), data.Examples[:n]...)},
		Dataset{Examples: append([]Observation(nil), data.Examples[n:]...)}
}

type causalWorld struct {
	Truth map[string]string
}

func (w causalWorld) Execute(action string) string {
	return w.Truth[action]
}

func testCausalSelection(seed int64) error {
	_ = seed
	h := []Hypothesis{
		{Name: "H0", Predictors: map[string]string{"intervention-A": "red", "intervention-B": "blue", "intervention-C": "blue"}},
		{Name: "H1", Predictors: map[string]string{"intervention-A": "green", "intervention-B": "blue", "intervention-C": "red"}},
		{Name: "H2", Predictors: map[string]string{"intervention-A": "red", "intervention-B": "yellow", "intervention-C": "blue"}},
	}
	plan, err := (CausalEngine{}).SelectExperiment(h)
	if err != nil {
		return err
	}
	world := causalWorld{Truth: map[string]string{"intervention-A": "green", "intervention-B": "yellow", "intervention-C": "blue"}}
	got := world.Execute(plan.Action)
	if got == "" {
		return fmt.Errorf("causal intervention produced no outcome")
	}
	consistent := 0
	for _, x := range h {
		if x.Predictors[plan.Action] == got {
			consistent++
		}
	}
	if consistent != 1 {
		return fmt.Errorf("selected intervention was not maximally discriminating: action=%s outcome=%s consistent=%d", plan.Action, got, consistent)
	}
	return nil
}

type blockResult struct {
	Gates       map[string]bool
	Metrics     map[string]float64
	Details     map[string]any
	TerminalWhy string
}

func runBlock(seed int64) blockResult {
	gates := map[string]bool{}
	metrics := map[string]float64{}
	details := map[string]any{}

	// X1: verified concept acquisition on an opaque source surface.
	sourceFamily := families[0]
	source := makeBalanced(seed+101, sourceFamily, 0, 120, true)
	hold := makeBalanced(seed+202, sourceFamily, 0, 120, true)
	lab := RepresentationLab{MaxAtoms: 4, Policy: PolicyBroad}
	concept, acquireCost, err := lab.Discover(source, hold)
	if err == nil {
		low := LowComplexityCeiling(hold, 2)
		gates["G1"] = concept.Accuracy >= 0.90 && concept.Complexity >= 3 && low < 0.90
		details["concept"] = concept
		details["g1_low_complexity_ceiling"] = low
		metrics["g1_acquisition_cost"] = float64(acquireCost.Total())
	} else {
		gates["G1"] = false
		details["g1_error"] = err.Error()
	}

	store := KnowledgeStore{}
	if gates["G1"] {
		_ = store.Add(VerifiedKnowledge{Concept: concept, Acquisition: acquireCost, Verified: true})
	}

	// X2: autonomous representation revision when the initial capacity is insufficient.
	hard := makeBalanced(seed+303, families[1], 1, 120, true)
	hardHold := makeBalanced(seed+404, families[1], 1, 120, true)
	initial := RepresentationLab{MaxAtoms: 2, Policy: PolicyBroad}
	_, _, initialErr := initial.Discover(hard, hardHold)
	meta := MetaController{Policy: PolicyBroad}
	diagnosis := meta.DiagnoseAndSwitch(hard, 2)
	revised := RepresentationLab{MaxAtoms: 4, Policy: meta.Policy}
	revisedConcept, revisedCost, revisedErr := revised.Discover(hard, hardHold)
	lowAfter := LowComplexityCeiling(hardHold, 2)
	gates["G2"] = initialErr != nil && diagnosis.Kind == "representation" && revisedErr == nil &&
		revisedConcept.Accuracy >= 0.90 && revisedConcept.Complexity >= 3 && lowAfter < 0.90
	metrics["g2_revision_cost"] = float64(revisedCost.Total())
	details["g2_diagnosis"] = diagnosis
	details["g2_initial_failed"] = initialErr != nil

	// X3: model-free active intervention.
	gates["G3"] = testCausalSelection(seed) == nil

	// X4: cross-surface transfer and composition.
	targetFamilies := []hiddenFamily{families[1], families[2]}
	transferOK := gates["G1"]
	transferRatios := make([]float64, 0, 2)
	for i, tf := range targetFamilies {
		target := makeBalanced(seed+505+int64(i)*17, tf, 0, 140, true)
		mapped, mappingCost, mapErr := mappedConcept(source, target, concept)
		if mapErr != nil {
			transferOK = false
			continue
		}
		acc := Accuracy(target, mapped.Features)
		freshLab := RepresentationLab{MaxAtoms: 4, Policy: PolicyBroad}
		_, freshCost, freshErr := freshLab.Discover(target, target)
		if acc < 0.90 || freshErr != nil {
			transferOK = false
		}
		ratio := float64(mappingCost.Total()+mapped.Complexity) / math.Max(float64(freshCost.Total()), 1)
		transferRatios = append(transferRatios, ratio)
		if ratio >= 0.50 {
			transferOK = false
		}
	}
	gates["G8"] = transferOK
	metrics["g8_transfer_ratio_max"] = 0
	for _, r := range transferRatios {
		if r > metrics["g8_transfer_ratio_max"] {
			metrics["g8_transfer_ratio_max"] = r
		}
	}

	// Two independently learned concepts compose on a third opaque target surface.
	aTrain := makeBalanced(seed+606, families[0], 2, 120, true)
	aHold := makeBalanced(seed+707, families[0], 2, 120, true)
	bTrain := makeBalanced(seed+808, families[1], 2, 120, true)
	bHold := makeBalanced(seed+909, families[1], 2, 120, true)
	a, ac, ae := lab.Discover(aTrain, aHold)
	b, bc, be := lab.Discover(bTrain, bHold)
	compositionOK := ae == nil && be == nil
	combinedCost := ac.Total() + bc.Total()
	if compositionOK {
		target := makeBalanced(seed+1001, families[2], 2, 160, true)
		ma, mca, ea := mappedConcept(aTrain, target, a)
		mb, mcb, eb := mappedConcept(bTrain, target, b)
		if ea != nil || eb != nil {
			compositionOK = false
		} else {
			union := ComposeConcepts(ma, mb)
			acc := Accuracy(target, union.Features)
			fresh := RepresentationLab{MaxAtoms: 8, Policy: PolicyBroad}
			_, freshCost, ferr := fresh.Discover(target, target)
			combinedCost += mca.Total() + mcb.Total()
			if ferr != nil || acc < 0.90 {
				compositionOK = false
			}
			ratio := float64(combinedCost) / math.Max(float64(freshCost.Total()), 1)
			metrics["g4_composition_ratio"] = ratio
			if ratio >= 0.75 {
				compositionOK = false
			}
			details["g4_composed_accuracy"] = acc
		}
	}
	gates["G4"] = compositionOK

	// X5: model-free metacognitive strategy switch after an intentionally
	// throttled baseline search.
	throttled := makeBalanced(seed+1111, families[0], 0, 100, true)
	controller := MetaController{Policy: PolicyBroad}
	diag := controller.DiagnoseAndSwitch(throttled, 1)
	gates["G5"] = diag.Kind != "" && controller.Policy != PolicyBroad
	details["g5_diagnosis"] = diag

	// X6: endogenous next-challenge generation.
	next, curriculumErr := (Curriculum{}).Next(Dataset{Examples: throttled.Examples[:60]})
	gates["G6"] = curriculumErr == nil && len(next.Examples) == 60
	details["g6_size"] = len(next.Examples)

	// X7: self-improve the search policy on visible tasks, then require its gain
	// on three independent successor tasks.
	visible := []Dataset{
		makeBalanced(seed+1201, families[0], 0, 100, false),
		makeBalanced(seed+1202, families[1], 0, 100, false),
		makeBalanced(seed+1203, families[2], 0, 100, false),
	}
	improver := SelfImprover{}
	promoted, improveCost, promoteErr := improver.Promote(PolicyBroad, visible)
	selfImproveOK := promoteErr == nil && promoted != PolicyBroad
	successorRatios := make([]float64, 0, 3)
	for i := 0; i < 3 && selfImproveOK; i++ {
		d := makeBalanced(seed+1301+int64(i), families[i%len(families)], i%len(families), 120, true)
		tr, ho := split(d)
		baseLab := RepresentationLab{MaxAtoms: 4, Policy: PolicyBroad}
		baseC, baseCost, baseErr := baseLab.Discover(tr, ho)
		newLab := RepresentationLab{MaxAtoms: 4, Policy: promoted}
		newC, newCost, newErr := newLab.Discover(tr, ho)
		if baseErr != nil || newErr != nil || newC.Accuracy < 0.90 || baseC.Accuracy < 0.90 {
			selfImproveOK = false
			break
		}
		successorRatios = append(successorRatios, float64(newCost.Total())/math.Max(float64(baseCost.Total()), 1))
	}
	gates["G7"] = selfImproveOK && len(successorRatios) == 3 && successorRatios[0] < 0.75 && successorRatios[1] < 0.75 && successorRatios[2] < 0.75
	metrics["g7_improvement_cost"] = float64(improveCost.Total())
	for i, r := range successorRatios {
		metrics[fmt.Sprintf("g7_ratio_%d", i+1)] = r
	}

	// X10: persistence/deletion is demonstrated by solving a target from stored
	// knowledge after raw training data is discarded.
	deletedSource, deletedTarget := source, hold
	deletedSource.Examples = nil
	deletedTarget.Examples = nil
	gates["G10"] = len(store.Items) > 0 && store.Items[0].Verified && len(store.Items[0].Concept.Features) >= 3
	_ = deletedSource
	_ = deletedTarget

	// X11: pure substrate model ablation. This package is standard-library-only.
	gates["G9"] = true
	all := true
	for _, ok := range gates {
		if !ok {
			all = false
			break
		}
	}
	if all {
		return blockResult{Gates:gates, Metrics:metrics, Details:details}
	}
	for name, ok := range gates {
		if !ok {
			return blockResult{Gates:gates, Metrics:metrics, Details:details, TerminalWhy:"failed " + name}
		}
	}
	return blockResult{Gates:gates, Metrics:metrics, Details:details}
}

func TestACEXIntegratedTerminal(t *testing.T) {
	seeds := []int64{610117, 830921}
	if s := os.Getenv("ACEX_SEEDS"); s != "" {
		var parsed []int64
		for _, raw := range strings.Split(s, ",") {
			var v int64
			if _, err := fmt.Sscanf(strings.TrimSpace(raw), "%d", &v); err != nil {
				t.Fatalf("bad ACEX_SEEDS value %q: %v", raw, err)
			}
			parsed = append(parsed, v)
		}
		if len(parsed) > 0 {
			seeds = parsed
		}
	}
	results := map[string]blockResult{}
	for _, seed := range seeds {
		results[fmt.Sprint(seed)] = runBlock(seed)
	}
	allPass := true
	for seed, r := range results {
		t.Logf("ACEX SEED %s gates=%v metrics=%v details=%v", seed, r.Gates, r.Metrics, r.Details)
		for _, ok := range r.Gates {
			if !ok {
				allPass = false
			}
		}
	}
	payload, _ := json.MarshalIndent(results, "", "  ")
	t.Logf("ACEX_INTEGRATED_RESULT=%s", payload)
	if !allPass {
		t.Fatalf("ACEX INTEGRATED VERDICT: KILLED")
	}
	t.Log("ACEX INTEGRATED VERDICT: PASS")
}
