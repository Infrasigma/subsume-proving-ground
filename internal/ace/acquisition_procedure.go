package ace

import (
    "encoding/json"
    "errors"
    "fmt"
    "sort"
)

type AcquisitionProcedure struct { Version int `json:"version"`; Steps []ProcedureStep `json:"steps"` }
type ProcedureStep struct { Op string `json:"op"`; Arg int `json:"arg,omitempty"` }

func (p AcquisitionProcedure) Marshal() (string,error) { if p.Version!=1||len(p.Steps)==0{return "",errors.New("invalid acquisition procedure")};b,e:=json.Marshal(p);return string(b),e }
func decodeAcquisitionProcedure(s string)(AcquisitionProcedure,error){var p AcquisitionProcedure;if e:=json.Unmarshal([]byte(s),&p);e!=nil{return p,e};if p.Version!=1||len(p.Steps)==0{return p,errors.New("invalid acquisition procedure")};return p,nil}

// This is bootstrap-level exhaustive enumeration over generic candidate-stream
// transformations. It contains no target method names or benchmark-specific
// decompositions. The resulting procedure is an executable acquired artifact.
func EnumerateAcquisitionProcedures(maxSteps int)[]AcquisitionProcedure{
    if maxSteps<1{return nil}
    atoms:=[]ProcedureStep{{Op:"identity"},{Op:"reverse"},{Op:"dedupe"},{Op:"sort-cost"},{Op:"take",Arg:1},{Op:"rotate",Arg:1}}
    out:=make([]AcquisitionProcedure,0,64)
    var rec func([]ProcedureStep,int)
    rec=func(prefix []ProcedureStep,depth int){if depth==0{out=append(out,AcquisitionProcedure{Version:1,Steps:append([]ProcedureStep(nil),prefix...)});return};for _,a:=range atoms{rec(append(prefix,a),depth-1)}}
    for d:=1;d<=maxSteps;d++{rec(nil,d)}
    return out
}

func executeSearchProcedure(p AcquisitionProcedure,cs []ArchitectureCandidate)([]ArchitectureCandidate,error){
    cur:=append([]ArchitectureCandidate(nil),cs...)
    for _,s:=range p.Steps{switch s.Op{
    case "identity":
    case "reverse":for i,j:=0,len(cur)-1;i<j;i,j=i+1,j-1{cur[i],cur[j]=cur[j],cur[i]}
    case "dedupe":seen:=map[string]bool{};next:=make([]ArchitectureCandidate,0,len(cur));for _,c:=range cur{if !seen[c.Mechanism]{seen[c.Mechanism]=true;next=append(next,c)}};cur=next
    case "sort-cost":sort.SliceStable(cur,func(i,j int)bool{return cur[i].Resources.Compute+cur[i].Resources.ExperimentBudget<cur[j].Resources.Compute+cur[j].Resources.ExperimentBudget})
    case "take":if s.Arg<1||s.Arg>len(cur){return nil,fmt.Errorf("take argument %d outside stream",s.Arg)};cur=append([]ArchitectureCandidate(nil),cur[:s.Arg]...)
    case "rotate":if len(cur)==0{continue};n:=s.Arg%len(cur);if n<0{n+=len(cur)};cur=append(append([]ArchitectureCandidate(nil),cur[n:]...),cur[:n]...)
    default:return nil,fmt.Errorf("unknown acquisition procedure op %q",s.Op)}}
    return cur,nil
}
func procedureSignature(p AcquisitionProcedure)string{b,_:=json.Marshal(p);return Hash(string(b))}
