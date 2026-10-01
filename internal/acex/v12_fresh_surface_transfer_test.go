package acex

import (
    "fmt"
    "testing"
)

func v12FreshSurfaceState(seed int, generator int, width int) (RelationalState, []string, string) {
    actions := make([]string, width)
    nodes := make([]RelNode, 0, width*6)
    edges := make([]RelEdge, 0, width*6)
    kinds := []string{"lever", "token", "marker", "gate", "switch"}
    support := []string{"peg", "port", "node", "joint", "cell"}
    edgeKinds := []string{"route", "bind", "engage", "link", "flow"}
    correct := (seed*3 + generator) % width
    for i := 0; i < width; i++ {
        a := fmt.Sprintf("g%d-a%d-%d", generator, seed, i)
        actions[i] = a
        sk := support[(i+generator)%len(support)]
        nodes = append(nodes, RelNode{ID:a, Kind:kinds[(i+generator)%len(kinds)]})
        for _, suffix := range []string{"p","q","r","s","t"} {
            nodes = append(nodes, RelNode{ID:a+"-"+suffix, Kind:sk})
        }
    }
    ek := edgeKinds[generator%len(edgeKinds)]
    for i, a := range actions {
        x,y,z := a+"-p", a+"-q", a+"-r"
        if i == correct {
            edges = append(edges,
                RelEdge{From:a, To:x, Kind:ek},
                RelEdge{From:x, To:y, Kind:ek},
                RelEdge{From:y, To:z, Kind:ek},
            )
            if generator%2 == 1 {
                edges = append(edges, RelEdge{From:a+"-s", To:a, Kind:ek})
            }
            if generator >= 4 {
                edges = append(edges, RelEdge{From:a+"-t", To:a+"-s", Kind:ek})
            }
        } else {
            edges = append(edges,
                RelEdge{From:x, To:a, Kind:ek},
                RelEdge{From:y, To:x, Kind:ek},
                RelEdge{From:z, To:y, Kind:ek},
            )
            if generator%2 == 1 {
                edges = append(edges, RelEdge{From:a+"-s", To:a, Kind:ek})
            }
        }
    }
    return RelationalState{Nodes:nodes, Edges:edges}, actions, actions[correct]
}

func TestV12FreshSurfaceHermeticDirectedTransfer(t *testing.T) {
    entity := NewV8CognitiveEntity()

    for i := 0; i < 7; i++ {
        state, actions, correct := v12FreshSurfaceState(14000+i*113, i%4, 3+(i%2))
        for _, action := range actions {
            entity.DirectedRepresentation.Record(state, action, -1, false)
        }
        entity.DirectedRepresentation.Record(state, correct, 1, true)
        if !entity.DirectedRepresentation.Synthesize() {
            t.Fatalf("source synthesis failed i=%d expansions=%d key=%s", i, entity.DirectedRepresentation.SearchExpansions, entity.DirectedRepresentation.Key())
        }
    }
    if !entity.DirectedRepresentation.Valid {
        t.Fatalf("learned representation invalid")
    }

    artifact, err := entity.ExportRetainedRepresentation()
    if err != nil {
        t.Fatal(err)
    }

    fresh := NewV8CognitiveEntity()
    if err := fresh.LoadRetainedRepresentation(artifact); err != nil {
        t.Fatal(err)
    }

    fresh.DirectedRepresentation.Nodes = 2
    fresh.DirectedRepresentation.Edges = []V11DirectedPatternEdge{{From:0,To:1},{From:1,To:0}}
    fresh.DirectedRepresentation.Valid = true
    fresh.DirectedRepresentation.Retained = true

    holdoutCases := []struct{ seed, generator, width int }{
        {88001, 7, 5},
        {99103, 9, 6},
        {77117, 11, 4},
    }

    for _, tc := range holdoutCases {
        state, actions, correct := v12FreshSurfaceState(tc.seed, tc.generator, tc.width)
        got, err := fresh.ObserveAndAct(state, actions)
        if err != nil {
            t.Fatalf("holdout seed=%d generator=%d: %v", tc.seed, tc.generator, err)
        }
        if got != correct {
            t.Fatalf("fresh hermetic transfer failed seed=%d generator=%d: got=%q want=%q", tc.seed, tc.generator, got, correct)
        }
    }
}
