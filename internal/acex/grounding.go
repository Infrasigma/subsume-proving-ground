package acex

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

type GroundedObservation struct {
	State   RelationalState
	Actions []string
	Digest  string
}

type Grounder interface {
	Ground(raw []byte) (GroundedObservation, error)
}

type JSONGrounder struct{}

type jsonEntity struct {
	ID    string            `json:"id"`
	Kind  string            `json:"kind"`
	Attrs map[string]string `json:"attrs"`
}

type jsonRelation struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

type jsonObservation struct {
	Entities  []jsonEntity   `json:"entities"`
	Relations []jsonRelation `json:"relations"`
	Actions   []string       `json:"actions"`
}

func (JSONGrounder) Ground(raw []byte) (GroundedObservation, error) {
	var in jsonObservation
	if err := json.Unmarshal(raw, &in); err != nil {
		return GroundedObservation{}, err
	}
	if len(in.Entities) == 0 {
		return GroundedObservation{}, errors.New("grounding requires objects")
	}

	nodes := make([]RelNode, 0, len(in.Entities))
	seen := map[string]bool{}
	for _, e := range in.Entities {
		if e.ID == "" || seen[e.ID] {
			return GroundedObservation{}, errors.New("invalid object identity")
		}
		seen[e.ID] = true
		attrs := map[string]string{}
		for k, v := range e.Attrs {
			attrs[k] = v
		}
		nodes = append(nodes, RelNode{ID: e.ID, Kind: e.Kind, Attrs: attrs})
	}

	edges := make([]RelEdge, 0, len(in.Relations))
	for _, e := range in.Relations {
		if !seen[e.From] || !seen[e.To] || e.Kind == "" {
			return GroundedObservation{}, errors.New("relation references unknown object")
		}
		edges = append(edges, RelEdge{From: e.From, To: e.To, Kind: e.Kind})
	}

	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		if edges[i].To != edges[j].To {
			return edges[i].To < edges[j].To
		}
		return edges[i].Kind < edges[j].Kind
	})
	sort.Strings(in.Actions)

	state := RelationalState{Nodes: nodes, Edges: edges}
	return GroundedObservation{
		State:   state,
		Actions: append([]string(nil), in.Actions...),
		Digest:  WLInvariant(state, 3),
	}, nil
}

type GroundingRuntime struct {
	Grounder Grounder
}

func (g GroundingRuntime) Observe(raw []byte) (GroundedObservation, error) {
	if g.Grounder == nil {
		return GroundedObservation{}, fmt.Errorf("no grounding engine")
	}
	return g.Grounder.Ground(raw)
}

func GroundedTextSummary(o GroundedObservation) string {
	parts := make([]string, 0, len(o.State.Nodes)+len(o.State.Edges))
	for _, n := range o.State.Nodes {
		parts = append(parts, "node:"+n.Kind+":"+strings.Join(sortedKeys(n.Attrs), ","))
	}
	for _, e := range o.State.Edges {
		parts = append(parts, "edge:"+e.Kind)
	}
	sort.Strings(parts)
	return strings.Join(parts, "|")
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
