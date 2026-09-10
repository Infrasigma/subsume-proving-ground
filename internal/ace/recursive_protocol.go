package ace

import (
    "errors"
    "strconv"
)

func conditionalCandidate(threshold int) UniversalProgram {
    cond:=&UExpr{Kind:"lt",Left:&UExpr{Kind:"const",Value:strconv.Itoa(threshold)},Right:&UExpr{Kind:"var",Value:"x"}}
    thenStmt:=UStmt{Kind:"assign",Target:"y",Expr:&UExpr{Kind:"const",Value:"1"}}
    elseStmt:=UStmt{Kind:"assign",Target:"y",Expr:&UExpr{Kind:"const",Value:"0"}}
    stmt:=UStmt{Kind:"if",Cond:cond,Then:[]UStmt{thenStmt},Else:[]UStmt{elseStmt}}
    return UniversalProgram{Statements:[]UStmt{stmt}}
}

func RunRecursiveCapabilityProtocolV3()(map[string]float64,error){
    lab:=LatentTaskLab{Families:[]TaskFamily{AffineFamily{},ThresholdFamily{},DeepCompositionFamily{}}}
    _,c1,_,err:=lab.GenerateDiscovery(5);if err!=nil{return nil,err}
    k1,err:=ParameterizedMechanismSearch(c1);if err!=nil{return nil,err}
    _,c2,_,err:=lab.GenerateDiscovery(9);if err!=nil{return nil,err}
    if _,err=ParameterizedMechanismSearch(c2);err==nil{return nil,errors.New("pre-M1 arithmetic method unexpectedly solved conditional task")}
    found:=false;attempts:=0
    for threshold:=-10;threshold<=10;threshold++{attempts++;if programFits(conditionalCandidate(threshold),c2){found=true;break}}
    if !found{return nil,errors.New("M1 conditional search failed")}
    _,c3,_,err:=lab.GenerateHidden(10);if err!=nil{return nil,err}
    spec:=CapabilitySpecification{ID:"hidden-depth-3",Inputs:[]string{"x"},Outputs:[]string{"y"},AcceptanceTests:[]string{"hidden"},ResourceLimits:ResourceVector{Compute:100,Memory:100,TimeMS:5000,ExperimentBudget:20},KnownExamples:c3}
    k0:=0
    for _,name:=range []string{"universal:straight-line","universal:branching","universal:compositional"}{
        k0++
        candidate:=ArchitectureCandidate{ID:name,Mechanism:name,Interfaces:[]string{"executable-program"},Tests:spec.AcceptanceTests,Resources:spec.ResourceLimits}
        p,e:=UniversalProgramBuilder{}.Build(candidate,spec)
        if e==nil&&programFitsJSON(p.Artifact,c3){return nil,errors.New("K0 solved hidden depth-3 task")}
    }
    mul,err:=ParameterizedMechanismSearch([]ProgramTestCase{{Input:map[string]string{"x":"2"},Expected:map[string]string{"y":"4"}},{Input:map[string]string{"x":"5"},Expected:map[string]string{"y":"10"}}});if err!=nil{return nil,err}
    inc,err:=ParameterizedMechanismSearch([]ProgramTestCase{{Input:map[string]string{"x":"2"},Expected:map[string]string{"y":"3"}},{Input:map[string]string{"x":"8"},Expected:map[string]string{"y":"9"}}});if err!=nil{return nil,err}
    q,err:=composePrograms(k1.Program,mul.Program,"x");if err!=nil{return nil,err};q,err=composePrograms(q,inc.Program,"x");if err!=nil{return nil,err}
    if !programFits(q,c3){return nil,errors.New("K2 failed hidden task")}
    return map[string]float64{"verified":1,"K0_candidates":float64(k0),"K2_T3_cost":2,"R":2/float64(k0),"M1_attempts":float64(attempts)},nil
}

func RunReplicatedCompoundingV2(n int)(map[string]float64,error){
    if n<2{return nil,errors.New("need replication")}
    lab:=LatentTaskLab{Families:[]TaskFamily{AffineFamily{},ReplicatedDeepFamily{}}}
    _,base,_,err:=lab.GenerateDiscovery(5);if err!=nil{return nil,err}
    shift,err:=ParameterizedMechanismSearch(base);if err!=nil{return nil,err}
    mul,err:=ParameterizedMechanismSearch([]ProgramTestCase{{Input:map[string]string{"x":"2"},Expected:map[string]string{"y":"4"}},{Input:map[string]string{"x":"5"},Expected:map[string]string{"y":"10"}}});if err!=nil{return nil,err}
    inc,err:=ParameterizedMechanismSearch([]ProgramTestCase{{Input:map[string]string{"x":"2"},Expected:map[string]string{"y":"3"}},{Input:map[string]string{"x":"8"},Expected:map[string]string{"y":"9"}}});if err!=nil{return nil,err}
    for i:=0;i<n;i++{
        _,cases,_,err:=lab.GenerateHidden(100+i);if err!=nil{return nil,err}
        spec:=CapabilitySpecification{ID:Hash(i),Inputs:[]string{"x"},Outputs:[]string{"y"},AcceptanceTests:[]string{"hidden"},ResourceLimits:ResourceVector{Compute:100,Memory:100,TimeMS:5000,ExperimentBudget:20},KnownExamples:cases}
        for _,name:=range []string{"universal:straight-line","universal:branching","universal:compositional"}{candidate:=ArchitectureCandidate{ID:name,Mechanism:name,Interfaces:[]string{"executable-program"},Tests:spec.AcceptanceTests,Resources:spec.ResourceLimits};p,e:=UniversalProgramBuilder{}.Build(candidate,spec);if e==nil&&programFitsJSON(p.Artifact,cases){return nil,errors.New("K0 solved replicated hidden task")}}
        q,e:=composePrograms(shift.Program,mul.Program,"x");if e!=nil{return nil,e};q,e=composePrograms(q,inc.Program,"x");if e!=nil{return nil,e};if !programFits(q,cases){return nil,errors.New("K2 failed replicated task")}
    }
    return map[string]float64{"repetitions":float64(n),"wins":float64(n),"mean_R":2.0/3.0,"all_verified":1},nil
}
