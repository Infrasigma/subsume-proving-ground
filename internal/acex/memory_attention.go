package acex

import (
	"errors"
	"math"
	"sort"
	"strings"
)

type MemoryClass string

const (
	EpisodicMemory   MemoryClass = "episodic"
	SemanticMemory   MemoryClass = "semantic"
	ProceduralMemory MemoryClass = "procedural"
	FailureMemory    MemoryClass = "failure"
	ModelMemory      MemoryClass = "model"
)

type MemoryItem struct {
	ID          string
	Class       MemoryClass
	Keys        []string
	Abstract   []string
	Reliability float64
	Utility     float64
	Scope       string
	Negative    []string
	Version     int
}

type MemoryManager struct {
	Items []MemoryItem
}

func keyOverlap(a, b []string) float64 {
	set := map[string]bool{}
	for _, x := range a {
		set[x] = true
	}
	if len(set) == 0 {
		return 0
	}
	n := 0
	for _, x := range b {
		if set[x] {
			n++
		}
	}
	return float64(n) / math.Max(float64(len(set)), float64(len(b)))
}

func negativeConflict(item MemoryItem, context []string) bool {
	for _, x := range item.Negative {
		for _, y := range context {
			if x == y {
				return true
			}
		}
	}
	return false
}

func (m MemoryManager) Retrieve(context []string, limit int) []MemoryItem {
	if limit <= 0 {
		return nil
	}
	type scored struct {
		item MemoryItem
		score float64
	}
	candidates := make([]scored, 0, len(m.Items))
	for _, item := range m.Items {
		if negativeConflict(item, context) {
			continue
		}
		sim := keyOverlap(item.Keys, context)
		score := sim*item.Reliability*item.Utility + 0.10*float64(len(item.Abstract))
		if item.Class == FailureMemory {
			score += 0.05
		}
		if score > 0 {
			candidates = append(candidates, scored{item: item, score: score})
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].item.ID < candidates[j].item.ID
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	out := make([]MemoryItem, 0, len(candidates))
	for _, x := range candidates {
		out = append(out, x.item)
	}
	return out
}

func (m *MemoryManager) Add(item MemoryItem) error {
	if item.ID == "" || item.Class == "" {
		return errors.New("invalid memory item")
	}
	if item.Reliability < 0 || item.Reliability > 1 {
		return errors.New("invalid reliability")
	}
	m.Items = append(m.Items, item)
	return nil
}

func (m *MemoryManager) Consolidate(source []Trace, concept Concept, class MemoryClass, scope string) (MemoryItem, Resource, error) {
	if len(source) < 2 {
		return MemoryItem{}, Resource{}, errors.New("need repeated experience for consolidation")
	}
	keys := make([]string, 0)
	seen := map[string]bool{}
	for _, t := range source {
		for _, step := range t.Steps {
			key := step.Op
			if !seen[key] {
				seen[key] = true
				keys = append(keys, key)
			}
		}
	}
	sort.Strings(keys)
	item := MemoryItem{
		ID:          "memory-" + concept.ID,
		Class:       class,
		Keys:        keys,
		Abstract:    append([]string(nil), concept.Features...),
		Reliability: concept.Accuracy,
		Utility:     1,
		Scope:       scope,
		Version:     1,
	}
	return item, Resource{Search: len(keys), Memory: 1, Storage: len(item.Abstract)}, nil
}

type AttentionController struct{}

func informationScore(p FeatureProfile) float64 {
	return math.Abs(p.Positive-p.Negative)
}

func (AttentionController) Select(data Dataset, budget int) []string {
	if budget <= 0 {
		return nil
	}
	profile := Profile(data)
	names := make([]string, 0, len(profile))
	for name := range profile {
		names = append(names, name)
	}
	sort.SliceStable(names, func(i, j int) bool {
		si, sj := informationScore(profile[names[i]]), informationScore(profile[names[j]])
		if si != sj {
			return si > sj
		}
		return names[i] < names[j]
	})
	if len(names) > budget {
		names = names[:budget]
	}
	return names
}

type ExecutiveDecision string

const (
	DecisionRetrieve  ExecutiveDecision = "retrieve"
	DecisionExperiment ExecutiveDecision = "experiment"
	DecisionSearch    ExecutiveDecision = "search"
	DecisionConsolidate ExecutiveDecision = "consolidate"
)

type ExecutiveState struct {
	Uncertainty float64
	MemoryMatch float64
	Resource    Resource
	Gap         Gap
}

type ExecutiveController struct{}

func (ExecutiveController) Choose(s ExecutiveState) ExecutiveDecision {
	if s.Gap.TransferFail {
		return DecisionExperiment
	}
	if s.Uncertainty >= 0.70 {
		return DecisionExperiment
	}
	if s.MemoryMatch >= 0.75 {
		return DecisionRetrieve
	}
	if s.Gap.SearchFail {
		return DecisionConsolidate
	}
	return DecisionSearch
}

func (ExecutiveController) Explain(s ExecutiveState, d ExecutiveDecision) string {
	return strings.Join([]string{string(d), "uncertainty", "memory", "gap"}, ":")
}
