package ace

import "testing"

func TestRepresentationRevisionInventsMissingVariableAndRejectsIrrelevantDistinction(t *testing.T){
    xs:=[]Experience{
        trace("r1","act",map[string]string{"visible":"same","mode":"A","noise":"0"},map[string]string{"out":"1"}),
        trace("r2","act",map[string]string{"visible":"same","mode":"B","noise":"0"},map[string]string{"out":"2"}),
        trace("r3","act",map[string]string{"visible":"same","mode":"A","noise":"1"},map[string]string{"out":"1"}),
        trace("r4","act",map[string]string{"visible":"same","mode":"B","noise":"1"},map[string]string{"out":"2"}),
    }
    current:=Representation{Variables:[]string{"visible"}}
    if residualCount(xs,current)==0{t.Fatal("constructed world does not alias under the current representation")}
    cs,err:=ProposeRepresentationRevisions(xs,current);if err!=nil{t.Fatal(err)};if len(cs)<2{t.Fatalf("expected competing revisions, got %d",len(cs))}
    best,err:=SelectRepresentationRevision(xs,current);if err!=nil{t.Fatal(err)}
    if !containsString(best.Representation.Variables,"mode"){t.Fatalf("winner did not invent causal distinction: %+v",best)}
    if best.ResidualAfter>=best.ResidualBefore{t.Fatalf("winning distinction did not reduce residual: %+v",best)}
    for _,c:=range cs{if containsString(c.Representation.Variables,"noise")&&c.Score>=best.Score{t.Fatalf("irrelevant distinction tied/beats causal distinction: %+v",cs)}}
}

func TestRepresentationRevisionWorksForDifferentWorldStructure(t *testing.T){
    xs:=[]Experience{
        trace("q1","move",map[string]string{"position":"same","phase":"cold"},map[string]string{"result":"left"}),
        trace("q2","move",map[string]string{"position":"same","phase":"hot"},map[string]string{"result":"right"}),
        trace("q3","move",map[string]string{"position":"same","phase":"cold","irrelevant":"x"},map[string]string{"result":"left"}),
        trace("q4","move",map[string]string{"position":"same","phase":"hot","irrelevant":"x"},map[string]string{"result":"right"}),
    }
    best,err:=SelectRepresentationRevision(xs,Representation{Variables:[]string{"position"}});if err!=nil{t.Fatal(err)}
    if !containsString(best.Representation.Variables,"phase"){t.Fatalf("failed structurally different alias world: %+v",best)}
}
