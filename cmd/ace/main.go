package main

import (
    "encoding/json"
    "flag"
    "fmt"
    "os"

    "github.com/Infrasigma/subsume-proving-ground/internal/ace"
)

func main(){
    fs:=flag.NewFlagSet("ace",flag.ExitOnError)
    state:=fs.String("state",".ace/state.json","persistent ACE state path")
    goal:=fs.String("goal","","task goal")
    novel:=fs.Bool("novel",false,"mark task as novel")
    fs.Parse(os.Args[1:])
    if *goal==""{fmt.Fprintln(os.Stderr,"--goal is required");os.Exit(2)}
    store,err:=ace.NewFileStore(*state);if err!=nil{fail(err)}
    rt:=&ace.Runtime{Store:store,Controller:ace.Controller{},Source:ace.StaticSource{},Causal:ace.NoopCausal{},Experiment:ace.DeterministicExperimenter{},Abstract:ace.DeterministicAbstractor{},Retrieve:ace.StructuralRetriever{},Sim:ace.ForwardSimulator{},Search:ace.BestFirstSearcher{},Authorize:ace.CapabilityGate{},Execute:&ace.StateExecutor{},Verify:ace.IndependentVerifier{},Diagnose:ace.HierarchicalDiagnoser{},Transfer:ace.StructuralTransfer{},ArchSearch:ace.MechanismCatalog{},Build:ace.DeclarativeBuilder{},Sandbox:ace.StrictSandbox{},Integrate:&ace.TransactionalIntegrator{}}
    task:=ace.Task{ID:ace.Hash([]string{*goal}),Goal:*goal,Novel:*novel,Budget:ace.ResourceVector{Compute:1,Memory:1e6,Storage:1e6,TimeMS:10000,ExperimentBudget:1}}
    v,err:=rt.Acquire(task);if err!=nil{fail(err)}
    b,_:=json.MarshalIndent(v,"","  ");fmt.Println(string(b))
}
func fail(e error){fmt.Fprintln(os.Stderr,"ACE:",e);os.Exit(1)}
