package ace

import (
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
