package acex

import "testing"

func v9Graph(ids []string, edges [][2]int) RelationalState {
	nodes := make([]RelNode, len(ids))
	for i, id := range ids {
		nodes[i] = RelNode{ID:id, Kind:"opaque", Attrs:map[string]string{}}
	}
	es := make([]RelEdge, 0, len(edges))
	for _, p := range edges {
		es = append(es, RelEdge{From:ids[p[0]], To:ids[p[1]], Kind:"r"})
	}
	return RelationalState{Nodes:nodes, Edges:es}
}

func TestV9StructuralRepresentationInventionVisible(t *testing.T) {
	train := []V9RepExample{
		{State:v9Graph([]string{"a","b","c","d"}, [][2]int{{0,1},{1,2},{2,3}}), Label:true},
		{State:v9Graph([]string{"p","q","r","s"}, [][2]int{{0,1},{1,2},{2,3}}), Label:true},
		{State:v9Graph([]string{"a","b","c","d"}, [][2]int{{0,1},{1,2},{2,0},{0,3}}), Label:false},
		{State:v9Graph([]string{"p","q","r","s"}, [][2]int{{0,1},{1,2},{2,3},{3,0}}), Label:false},
		{State:v9Graph([]string{"u","v","w","x"}, [][2]int{{0,1},{0,2},{0,3}}), Label:false},
		{State:v9Graph([]string{"m","n","o","z"}, [][2]int{{0,1},{1,2}}), Label:true},
	}
	inv := V9RepresentationInventor{MaxDepth:2, Budget:8000, MinTrainAccuracy:1}
	rep, err := inv.Invent(train)
	if err != nil {
		t.Fatal(err)
	}
	if rep.TrainAcc < 1 || rep.Expr == nil {
		t.Fatalf("representation not exact: %+v", rep)
	}
	holdout := []V9RepExample{
		{State:v9Graph([]string{"h1","h2","h3","h4"}, [][2]int{{0,1},{1,2},{2,3}}), Label:true},
		{State:v9Graph([]string{"k1","k2","k3","k4"}, [][2]int{{0,1},{1,2},{2,0},{0,3}}), Label:false},
		{State:v9Graph([]string{"x1","x2","x3","x4"}, [][2]int{{0,1},{0,2},{0,3}}), Label:false},
	}
	if v9Accuracy(holdout, rep.Expr) < 0.90 {
		t.Fatalf("invented representation failed structural holdout: desc=%s", V9RepresentationDescription(rep))
	}
	var lib V9RepresentationLibrary
	if err := lib.Admit(rep, holdout); err != nil {
		t.Fatal(err)
	}
	rawDeleted := append([]V9RepExample(nil), train...)
	for i := range rawDeleted {
		rawDeleted[i].State = RelationalState{}
	}
	got, err := rep.Apply(holdout[0].State)
	if err != nil || !got {
		t.Fatalf("persisted representation failed after raw-example deletion: got=%v err=%v", got, err)
	}
}
