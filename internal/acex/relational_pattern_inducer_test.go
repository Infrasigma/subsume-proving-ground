package acex

import "testing"

func aliasPatternState(prefix string, surface int, positive bool) (RelationalState, string) {
	root := prefix + "-root"
	a, b := prefix+"-a", prefix+"-b"
	c, d, e, f := prefix+"-c", prefix+"-d", prefix+"-e", prefix+"-f"
	kindRoot := "lever"
	kindSupport := "peg"
	edgeKind := "engage"
	if surface == 1 {
		kindRoot = "token"
		kindSupport = "port"
		edgeKind = "route"
	}
	nodes := []RelNode{
		{ID:root, Kind:kindRoot, Attrs:map[string]string{"surface":"opaque"}},
		{ID:a, Kind:kindSupport, Attrs:map[string]string{"surface":"opaque"}},
		{ID:b, Kind:kindSupport, Attrs:map[string]string{"surface":"opaque"}},
		{ID:c, Kind:kindSupport, Attrs:map[string]string{"surface":"opaque"}},
		{ID:d, Kind:kindSupport, Attrs:map[string]string{"surface":"opaque"}},
		{ID:e, Kind:kindSupport, Attrs:map[string]string{"surface":"opaque"}},
		{ID:f, Kind:kindSupport, Attrs:map[string]string{"surface":"opaque"}},
	}
	edges := []RelEdge{
		{From:root,To:a,Kind:edgeKind},
		{From:root,To:b,Kind:edgeKind},
		{From:a,To:c,Kind:edgeKind},
		{From:a,To:d,Kind:edgeKind},
		{From:b,To:e,Kind:edgeKind},
		{From:b,To:f,Kind:edgeKind},
	}
	if positive {
		edges = append(edges, RelEdge{From:c,To:d,Kind:edgeKind})
	} else {
		edges = append(edges, RelEdge{From:c,To:e,Kind:edgeKind})
	}
	return RelationalState{Nodes:nodes,Edges:edges},root
}

func TestRelationalPatternSynthesizerBreaksBoundedFamily(t *testing.T) {
	inducer := NewV8RelationalPatternInducer()
	pos, posRoot := aliasPatternState("src",0,true)
	neg, negRoot := aliasPatternState("src-wrong",0,false)
	inducer.Record(neg,negRoot,-1,false)
	inducer.Record(pos,posRoot,1,false)
	if !inducer.TrySynthesize() {
		t.Fatalf("generic relational pattern synthesis failed; expansions=%d",inducer.SearchExpansions)
	}
	if inducer.Pattern == nil {
		t.Fatal("no retained pattern")
	}
	if !inducer.Pattern.Separates(inducer.Examples) {
		t.Fatalf("retained pattern does not separate examples: %s",inducer.Pattern.Key())
	}
	targetPos, targetRoot := aliasPatternState("surface-changed",1,true)
	targetNeg, targetNegRoot := aliasPatternState("surface-changed",1,false)
	if !inducer.Pattern.Match(targetPos,targetRoot) || inducer.Pattern.Match(targetNeg,targetNegRoot) {
		t.Fatalf("retained relational representation did not transfer across surface change: %s",inducer.Pattern.Key())
	}
	before := inducer.Pattern.Key()
	inducer.ForgetExamples()
	if len(inducer.Examples) != 0 {
		t.Fatal("raw examples were not deleted")
	}
	got, ok := inducer.Select(targetPos, []string{targetNegRoot,targetRoot})
	if !ok || got != targetRoot {
		t.Fatalf("retained pattern failed after raw-example deletion: got=%q want=%q",got,targetRoot)
	}
	if before != inducer.Pattern.Key() {
		t.Fatal("retained representation changed after raw-example deletion")
	}
}

func TestRelationalPatternSynthesizerBudgetIsBounded(t *testing.T) {
	inducer := NewV8RelationalPatternInducer()
	inducer.Budget = 3
	pos, posRoot := aliasPatternState("budget-pos",0,true)
	neg, negRoot := aliasPatternState("budget-neg",0,false)
	inducer.Record(neg,negRoot,-1,false)
	inducer.Record(pos,posRoot,1,false)
	inducer.TrySynthesize()
	if inducer.SearchExpansions > 3 {
		t.Fatalf("search budget exceeded: expansions=%d",inducer.SearchExpansions)
	}
}
