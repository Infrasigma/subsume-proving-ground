package acex

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

type RelNode struct {
	ID    string
	Kind  string
	Attrs map[string]string
}

type RelEdge struct {
	From string
	To   string
	Kind string
}

type RelationalState struct {
	Nodes []RelNode
	Edges []RelEdge
}

func hashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// WLInvariant computes an ID-independent relational fingerprint. It is a
// deterministic neighborhood refinement, not a learned model.
func WLInvariant(g RelationalState, rounds int) string {
	labels := map[string]string{}
	adj := map[string][]string{}
	for _, n := range g.Nodes {
		attrs := make([]string, 0, len(n.Attrs))
		for k, v := range n.Attrs {
			attrs = append(attrs, k+"="+v)
		}
		sort.Strings(attrs)
		labels[n.ID] = hashString(n.Kind+"|"+strings.Join(attrs,"|"))
	}
	for _, e := range g.Edges {
		adj[e.From] = append(adj[e.From], e.Kind+">"+e.To)
		adj[e.To] = append(adj[e.To], e.Kind+"<"+e.From)
	}
	for i := 0; i < rounds; i++ {
		next := map[string]string{}
		for _, n := range g.Nodes {
			parts := make([]string, 0, len(adj[n.ID]))
			for _, rel := range adj[n.ID] {
				j := strings.LastIndex(rel, ">")
				if j >= 0 {
					target := rel[j+1:]
					parts = append(parts, rel[:j]+">"+labels[target])
					continue
				}
				j = strings.LastIndex(rel, "<")
				if j >= 0 {
					target := rel[j+1:]
					parts = append(parts, rel[:j]+"<"+labels[target])
				}
			}
			sort.Strings(parts)
			next[n.ID] = hashString(labels[n.ID]+"|"+strings.Join(parts,"|"))
		}
		labels = next
	}
	all := make([]string, 0, len(labels))
	for _, v := range labels {
		all = append(all, v)
	}
	sort.Strings(all)
	return hashString(strings.Join(all,"|"))
}

type RelationalConcept struct {
	ID          string
	Fingerprint string
	Support     int
	Confidence  float64
}

type RelationalMemory struct {
	Items []RelationalConcept
}

func (m *RelationalMemory) Add(c RelationalConcept) {
	m.Items = append(m.Items, c)
}

func (m RelationalMemory) Match(g RelationalState, rounds int) (RelationalConcept, bool) {
	f := WLInvariant(g, rounds)
	for _, c := range m.Items {
		if c.Fingerprint == f && c.Confidence >= 0.9 {
			return c, true
		}
	}
	return RelationalConcept{}, false
}
