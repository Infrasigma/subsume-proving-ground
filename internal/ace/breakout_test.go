package ace

import (
    "encoding/json"
    "testing"
)

func TestUniversalSynthesisEscapesAffineCeiling(t *testing.T) {
    task:=Task{ID:"abs-task",Goal:"produce absolute magnitude"}
    examples:=[]ProgramTestCase{{Input:map[string]string{"x":"-3"},Expected:map[string]string{"y":"3"}},{Input:map[string]string{"x":"4"},Expected:map[string]string{"y":"4"}},{Input:map[string]string{"x":"-1"},Expected:map[string]string{"y":"1"}}}
    if _,err:=ParseAffineIncrement(task.Goal);err==nil{t.Fatal("old affine task parser unexpectedly accepts a non-affine capability")}
    spec,err:=GeneralCapabilitySpecification(task,examples);if err!=nil{t.Fatal(err)}
    cs,err:=UniversalMechanismSearch{}.SearchMechanisms(spec,ResourceVector{TimeMS:10000});if err!=nil||len(cs)<2{t.Fatalf("expected competing construction strategies: %v",err)}
    var chosen ModificationProposal
    for _,c:=range cs {p,e:=UniversalProgramBuilder{}.Build(c,spec);if e==nil {chosen=p;break}}
    if chosen.Artifact==""{t.Fatal("no executable mechanism synthesized")}
    var prog UniversalProgram;if err:=json.Unmarshal([]byte(chosen.Artifact),&prog);err!=nil{t.Fatal(err)}
    got,err:=prog.Run(map[string]string{"x":"-9"});if err!=nil||got["y"]!="9"{t.Fatalf("synthesized mechanism failed held-out case: got=%v err=%v",got,err)}
}

func TestUniversalProgramRejectsIncorrectCandidate(t *testing.T) {
    bad:=UniversalProgram{Statements:[]UStmt{{Kind:"assign",Target:"y",Expr:&UExpr{Kind:"var",Value:"x"}}}}
    cases:=[]ProgramTestCase{{Input:map[string]string{"x":"-3"},Expected:map[string]string{"y":"3"}}}
    if programFits(bad,cases){t.Fatal("incorrect candidate was accepted")}
}

func TestUniversalProgramSupportsCompositionAndBranching(t *testing.T) {
    x:=UExpr{Kind:"var",Value:"x"};zero:=UExpr{Kind:"const",Value:"0"};one:=UExpr{Kind:"const",Value:"1"};cond:=UExpr{Kind:"lt",Left:&x,Right:&zero};neg:=UExpr{Kind:"sub",Left:&zero,Right:&x};plus:=UExpr{Kind:"add",Left:&x,Right:&one}
    p:=UniversalProgram{Statements:[]UStmt{{Kind:"if",Cond:&cond,Then:[]UStmt{{Kind:"assign",Target:"y",Expr:&neg}},Else:[]UStmt{{Kind:"assign",Target:"y",Expr:&x}}}}}
    got,err:=p.Run(map[string]string{"x":"-2"});if err!=nil||got["y"]!="2"{t.Fatalf("branch failed: %v %v",got,err)}
    got,err=p.Run(map[string]string{"x":"2"});if err!=nil||got["y"]!="2"{t.Fatalf("branch failed: %v %v",got,err)}
    loop:=UniversalProgram{Statements:[]UStmt{{Kind:"assign",Target:"y",Expr:&x},{Kind:"repeat",Count:2,Body:[]UStmt{{Kind:"assign",Target:"y",Expr:&plus}}}}}
    got,err=loop.Run(map[string]string{"x":"1"});if err!=nil||got["y"]!="2"{t.Fatalf("iteration failed: %v %v",got,err)}
    _=one
}
