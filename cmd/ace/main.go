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
    state:=fs.String("state",".ace/state.json","persistent cognitive state path")
    goal:=fs.String("goal","","task goal, e.g. y=x+1")
    novel:=fs.Bool("novel",false,"mark task as novel")
    fs.Parse(os.Args[1:])
    if *goal==""{fmt.Fprintln(os.Stderr,"--goal is required");os.Exit(2)}

    reg,err:=ace.NewPersistentRegistry(*state);if err!=nil{fail(err)}
    task:=ace.Task{ID:ace.Hash([]string{*goal}),Goal:*goal,Novel:*novel,Budget:ace.ResourceVector{Compute:100,Memory:1e6,Storage:1e6,TimeMS:10000,ExperimentBudget:10}}

    // First prefer an actually persisted capability. Only an unavailable capability enters acquisition.
    for _,r:=range reg.Records(){
        if r.Capability.Name==*goal{
            v,err:=ace.ExecuteStructurally(reg,task);if err!=nil{fail(err)}
            printJSON(v);return
        }
    }
    acq:=ace.AutonomousAcquirer{Registry:reg,Search:ace.LearningMechanismSearch{Base:ace.CompetingMechanismSearch{},Registry:reg},Builder:ace.ProgramBuilder{}}
    rec,v,err:=acq.Acquire(task);if err!=nil{fail(err)}
    if v.Status!="verified"||!v.Independent{fail(fmt.Errorf("acquisition did not independently verify: %+v",v))}
    _=rec
    v,err=ace.ExecuteStructurally(reg,task);if err!=nil{fail(err)}
    printJSON(v)
}
func printJSON(v any){b,_:=json.MarshalIndent(v,"","  ");fmt.Println(string(b))}
func fail(e error){fmt.Fprintln(os.Stderr,"ACE:",e);os.Exit(1)}
