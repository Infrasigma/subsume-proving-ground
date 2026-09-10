package ace

import (
    "os"
    "testing"
)

func TestAutonomousAcquisitionPersistsAndTransfers(t *testing.T) {
    path:=t.TempDir()+"/registry.json"
    reg,err:=NewPersistentRegistry(path);if err!=nil{t.Fatal(err)}
    acq:=AutonomousAcquirer{Registry:reg}
    task:=Task{ID:"novel-1",Goal:"y=x+1",Requirements:[]string{"x"},Novel:true,Budget:ResourceVector{TimeMS:100,Compute:100,Memory:100,Storage:100,ExperimentBudget:10}}
    rec,v,err:=acq.Acquire(task);if err!=nil{t.Fatal(err)};if v.Status!="verified"||!v.Independent{t.Fatalf("acquisition not independently verified: %+v",v)}
    if rec.Mechanism!="increment"{t.Fatalf("search did not reject wrong mechanisms: %s",rec.Mechanism)}
    if rec.ArchitectureCost!=2{t.Fatalf("expected copy+increment search, got %v",rec.ArchitectureCost)}
    got,err:=ExecuteStructurally(reg,Task{ID:"novel-surface",Goal:"destination=source+1",Requirements:[]string{"source"},Novel:true,Budget:task.Budget});if err!=nil||got.Status!="verified"||!got.Independent{t.Fatalf("structural transfer failed: %+v %v",got,err)}
    reopened,err:=NewPersistentRegistry(path);if err!=nil{t.Fatal(err)};if len(reopened.Records())!=1{t.Fatalf("capability not persisted: %+v",reopened.Records())}
    again,err:=ExecuteStructurally(reopened,Task{ID:"restart",Goal:"destination=source+1",Requirements:[]string{"source"},Novel:true,Budget:task.Budget});if err!=nil||again.Status!="verified"{t.Fatalf("restart execution failed: %+v %v",again,err)}
}

func TestAutonomousAcquisitionRejectsBadCandidates(t *testing.T){
    spec:=CapabilitySpecification{ID:"inc",Inputs:[]string{"x"},Outputs:[]string{"y"},AcceptanceTests:[]string{"increment"},ResourceLimits:ResourceVector{TimeMS:100}}
    cs,_:=CompetingMechanismSearch{}.SearchMechanisms(spec,spec.ResourceLimits)
    tests:=map[string][]ProgramTestCase{};for _,c:=range cs{tests[c.ID]=[]ProgramTestCase{{Input:map[string]string{"x":"8"},Expected:map[string]string{"y":"9"}}}}
    _,rr,err:=SearchAndTestMechanism(cs,spec,ProgramBuilder{},ExecutableSandbox{Cases:tests});if err!=nil||!rr.Passed{t.Fatalf("valid candidate set unexpectedly rejected: %v %+v",err,rr)}
    bad:=cs[2];p,_:=ProgramBuilder{}.Build(bad,spec);if _,err:=ExecutableSandbox{Cases:tests}.Validate(p);err==nil{t.Fatal("bad zero candidate unexpectedly passed")}
}

func TestCapabilityRegistryUsesActualArtifact(t *testing.T){
    path:=t.TempDir()+"/registry.json";r,_:=NewPersistentRegistry(path)
    if _,_,err:=AutonomousAcquirer{Registry:r}.Acquire(Task{ID:"t",Goal:"y=x+1",Requirements:[]string{"x"},Budget:ResourceVector{TimeMS:100}});err!=nil{t.Fatal(err)}
    raw,err:=os.ReadFile(path);if err!=nil{t.Fatal(err)};if len(raw)==0{t.Fatal("registry file is empty")}
}
