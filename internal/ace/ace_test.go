package ace

import (
    "encoding/json"
    "testing"
)

func TestPromotionRequiresEvidence(t *testing.T) {
    k:=KnowledgeObject{ID:"k",Level:C2}
    if _,err:=Promote(k,C3,nil);err==nil{t.Fatal("promotion without evidence must fail")}
    e:=Evidence{ID:"e",Verified:true}
    k,err:=Promote(k,C4,[]Evidence{e});if err!=nil||k.Level!=C4{t.Fatalf("promotion failed: %v",err)}
}

func TestIndependentVerificationRejectsWrongState(t *testing.T){
    a:=Action{ID:"a",Arguments:map[string]string{"x":"1"},ExpectedEffects:[]string{"x=1"}}
    before:=State{ID:"s",Values:map[string]string{}}
    after:=State{ID:"s",Values:map[string]string{"x":"2"},Version:1}
    v,err:=IndependentVerifier{}.Verify(a,Prediction{},before,after);if err!=nil||v.Status!="failed"||!v.Independent{t.Fatalf("expected independent rejection: %+v %v",v,err)}
}

func TestSandboxAndRollback(t *testing.T){
    s:=CapabilitySpecification{ID:"c",DesiredBehaviour:"x",AcceptanceTests:[]string{"behavioural"}}
    c:=ArchitectureCandidate{ID:"a",Interfaces:[]string{"Capability"},Tests:[]string{"behavioural"}}
    p:=ModificationProposal{ID:"p",Capability:s,Candidate:c}
    r,err:=StrictSandbox{}.Validate(p);if err!=nil||!r.Passed{t.Fatalf("sandbox reject: %v",err)}
    i:=&TransactionalIntegrator{};if err=i.Integrate(p,r);err!=nil{t.Fatal(err)};if err=i.Rollback();err!=nil{t.Fatal(err)}
}

func TestFileStoreRoundTrip(t *testing.T){
    path:=t.TempDir()+"/state.json";s,err:=NewFileStore(path);if err!=nil{t.Fatal(err)}
    x:=Experience{ID:"x"};if err=s.Put(x);err!=nil{t.Fatal(err)};s2,err:=NewFileStore(path);if err!=nil{t.Fatal(err)};xs,_:=s2.Experiences();if len(xs)!=1||xs[0].ID!="x"{t.Fatalf("round trip failed: %+v",xs)}
}

func trace(id,op string,before,after map[string]string) Experience {
    tr:=TransitionRecord{Before:State{ID:"before-"+id,Values:before},Action:Action{ID:"action-"+id,Operation:op},After:State{ID:"after-"+id,Values:after},Uncertainty:Uncertainty{Score:0.1,Basis:"observed"}}
    return EncodeTransition(Experience{ID:id,Source:"test",Sequence:1,Observational:false,Provenance:Prov("test","","raw",id)},tr)
}

func TestTransitionRoundTripPreservesRawEvidence(t *testing.T){
    x:=trace("x","advance",map[string]string{"x":"1"},map[string]string{"x":"2"})
    original:=append([]byte(nil),x.Raw...); tr,err:=DecodeTransition(x);if err!=nil{t.Fatal(err)}
    tr.After.Values["x"]="99"
    if string(x.Raw)!=string(original){t.Fatal("mutating derived transition must not mutate raw evidence")}
    if tr.After.Values["x"]!="99"{t.Fatal("derived mutation did not apply")}
}

func TestCausalInterventionDiscriminatesAndUpdates(t *testing.T){
    key:=interventionKey(Action{Operation:"toggle",Arguments:map[string]string{"target":"x"}})
    m:=CausalModel{ID:"m",Hypotheses:[]Hypothesis{
        {ID:"h1",Confidence:0.5,Statement:"toggle causes high",InterventionOutcomes:map[string]string{key:"high"}},
        {ID:"h2",Confidence:0.5,Statement:"toggle causes low",InterventionOutcomes:map[string]string{key:"low"}},
    }}
    ex:=InformationGainExperimenter{}
    es,err:=ex.Propose(m,nil,ResourceVector{ExperimentBudget:1});if err!=nil||len(es)!=1{t.Fatalf("expected one discriminating experiment: %v",err)}
    if es[0].Objective<=0{t.Fatal("experiment has no discrimination value")}
    revised,err:=BayesianCausalEngine{}.Revise(m,Observation{ID:"obs",Outcome:map[string]string{"intervention":key,"outcome":"high"}});if err!=nil{t.Fatal(err)}
    if revised.Hypotheses[0].Confidence<=revised.Hypotheses[1].Confidence{t.Fatalf("evidence did not reweight hypotheses: %+v",revised.Hypotheses)}
    if len(revised.Hypotheses[1].Counterexamples)!=1{t.Fatal("contradictory evidence was not retained")}
}

type causalTestWorld struct{}
func(causalTestWorld)Intervene(_ map[string]string)(map[string]string,error){return map[string]string{"outcome":"high"},nil}

func TestExperimentExecutesSelectedIntervention(t *testing.T){
    key:=interventionKey(Action{Operation:"toggle",Arguments:map[string]string{"target":"x"}})
    e:=InformationGainExperimenter{World:causalTestWorld{}}
    ob,err:=e.Execute(Experiment{ID:"e",Intervention:map[string]string{"intervention":key}});if err!=nil{t.Fatal(err)}
    if ob.Outcome["outcome"]!="high"||ob.Outcome["intervention"]!=key{t.Fatalf("bad observation: %+v",ob)}
}

func TestAbstractionFindsInvariantAcrossSurfaceValues(t *testing.T){
    xs:=[]Experience{
        trace("a","advance",map[string]string{"x":"1","y":"10"},map[string]string{"x":"1","y":"11"}),
        trace("b","advance",map[string]string{"x":"7","y":"20"},map[string]string{"x":"7","y":"21"}),
    }
    ks,err:=TransitionAbstractor{}.Abstract(xs);if err!=nil||len(ks)!=1{t.Fatalf("abstraction missing: %v %+v",err,ks)}
    found:=false;for _,p:=range ks[0].Pattern{if p=="delta:y=1"{found=true}};if !found{t.Fatalf("expected invariant delta:y=1: %+v",ks[0])}
}

func TestStructuralRetrievalIgnoresVocabulary(t *testing.T){
    k:=KnowledgeObject{ID:"k",Level:C3,Pattern:[]string{"operation:advance","delta:y=1"}}
    tsk:=Task{Structure:[]string{"operation:advance","delta:y=1"}}
    out,err:=StructuralRetriever{}.Retrieve(tsk,[]KnowledgeObject{k,{ID:"wrong",Level:C3,Pattern:[]string{"operation:advance","delta:z=1"}}});if err!=nil||len(out)!=1||out[0].ID!="k"{t.Fatalf("structural retrieval wrong: %+v %v",out,err)}
}

func TestLearnedSimulationProducesEmpiricalDistribution(t *testing.T){
    xs:=[]Experience{
        trace("a","advance",map[string]string{"x":"1"},map[string]string{"x":"2"}),
        trace("b","advance",map[string]string{"x":"1"},map[string]string{"x":"2"}),
        trace("c","advance",map[string]string{"x":"1"},map[string]string{"x":"3"}),
    }
    p,err:=(EmpiricalSimulator{Experiences:xs}).Simulate(State{Values:map[string]string{"x":"1"}},Action{ID:"a",Operation:"advance"},ResourceVector{TimeMS:10});if err!=nil||len(p)!=2{t.Fatalf("bad empirical simulation: %+v %v",p,err)}
    if p[0].Probability!=2.0/3.0{t.Fatalf("expected 2/3 dominant outcome: %+v",p)}
}

func TestPlannerSearchesMultipleActions(t *testing.T){
    s:=State{Values:map[string]string{"x":"0"}}
    one:=Skill{ID:"inc1",Preconditions:[]string{"x=0"},Confidence:1,Actions:[]Action{{ID:"a",Operation:"set",Arguments:map[string]string{"x":"1"}}}}
    two:=Skill{ID:"inc2",Preconditions:[]string{"x=1"},Confidence:1,Actions:[]Action{{ID:"b",Operation:"set",Arguments:map[string]string{"x":"2"}}}}
    p,err:=(SafeSearchPlanner{MaxDepth:3}).Search(Task{ID:"t",Goal:"x=2"},s,[]Skill{one,two},nil,ResourceVector{TimeMS:10});if err!=nil||len(p.Steps)!=2{t.Fatalf("planner did not find two-step plan: %+v %v",p,err)}
}

func TestRepresentationResidualDetectsAliasing(t *testing.T){
    xs:=[]Experience{
        trace("a","act",map[string]string{"visible":"same"},map[string]string{"out":"1"}),
        trace("b","act",map[string]string{"visible":"same"},map[string]string{"out":"2"}),
    }
    r:=DetectRepresentationInsufficiency(xs);if len(r)!=1||len(r[0].Outcomes)!=2{t.Fatalf("representation alias not detected: %+v",r)}
}

func TestExecutableMechanismSearchConstructsAndRunsProgram(t *testing.T){
    spec:=CapabilitySpecification{ID:"inc",Inputs:[]string{"x"},Outputs:[]string{"y"},AcceptanceTests:[]string{"increment x into y"},ResourceLimits:ResourceVector{TimeMS:100}}
    cs,_:=CompetingMechanismSearch{}.SearchMechanisms(spec,spec.ResourceLimits)
    cases:=map[string][]ProgramTestCase{}
    for _,c:=range cs{cases[c.ID]=[]ProgramTestCase{{Input:map[string]string{"x":"4"},Expected:map[string]string{"y":"5"}}}}
    p,rr,err:=SearchAndTestMechanism(cs,spec,ProgramBuilder{},ExecutableSandbox{Cases:cases});if err!=nil||!rr.Passed{t.Fatalf("no executable candidate selected: %v %+v",err,rr)}
    var prog ExecutableProgram;if err=json.Unmarshal([]byte(p.Artifact),&prog);err!=nil{t.Fatal(err)}
    got,err:=prog.Run(map[string]string{"x":"9"});if err!=nil||got["y"]!="10"{t.Fatalf("constructed program not reusable: %+v %v",got,err)}
}
