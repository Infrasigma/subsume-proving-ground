package ace

import (
    "errors"
    "fmt"
    "strconv"
)

// ExecuteStructurally applies an acquired mechanism by inferred input/output roles,
// not by the surface variable names used during acquisition.
func ExecuteStructurally(r *PersistentRegistry,t Task)(VerificationResult,error){
    c,err:=ParseAffineIncrement(t.Goal);if err!=nil{return VerificationResult{Status:"failed",Independent:true},err}
    for _,rec:=range r.Records(){
        learned,err:=ParseAffineIncrement(rec.Capability.Name);if err!=nil{continue}
        if learned.Delta!=c.Delta || rec.Mechanism!="increment"{continue}
        input:=13;got,err:=runProgramArtifact(rec.Artifact,map[string]string{learned.Input:strconv.Itoa(input)});if err!=nil{return VerificationResult{Status:"failed",Independent:true},err}
        out,ok:=got[learned.Output];want:=input+c.Delta
        if !ok||out!=strconv.Itoa(want){return VerificationResult{Status:"failed",Independent:true,Observed:[]string{fmt.Sprintf("%s=%s",c.Output,out)}},errors.New("structurally transferred mechanism produced wrong consequence")}
        return VerificationResult{Status:"verified",Independent:true,Expected:[]string{c.Output+"="+strconv.Itoa(want)},Observed:[]string{c.Output+"="+out},Provenance:Prov("structural-transfer-execution",rec.Provenance.ID,"role-renaming",t)},nil
    }
    return VerificationResult{Status:"failed",Independent:true},errors.New("no acquired mechanism is structurally applicable")
}
