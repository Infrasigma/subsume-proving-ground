package ace

import "errors"

type AdaptiveAcquisitionRuntime struct { Methods InstalledMethodRegistry; History []AcquisitionExperience; PersistentMethods *PersistentMethodRegistry }
type AdaptiveAcquisitionResult struct { Method AcquisitionMethodArtifact; Diagnosis BottleneckDiagnosis; Evaluations []MethodEvaluation; Future CapabilityRecord; FutureCost ResourceVector; Trace []string }

func (r *AdaptiveAcquisitionRuntime) ImproveAndAcquire(telemetry AcquisitionTelemetry,failedSpec CapabilitySpecification,methodHidden []ProgramTestCase,futureSpec CapabilitySpecification,futureHidden []ProgramTestCase)(AdaptiveAcquisitionResult,error){
	if len(methodHidden)==0||len(futureHidden)==0{return AdaptiveAcquisitionResult{},errors.New("adaptive runtime requires independent evaluation cases")}
	if r.PersistentMethods!=nil&&len(r.Methods.Methods)==0{if err:=r.PersistentMethods.Restore(&r.Methods);err!=nil{return AdaptiveAcquisitionResult{},err}}
	method,diagnosis,evals,err:=AutonomousMethodImprovement(telemetry,failedSpec,methodHidden,nil,r.History);if err!=nil{return AdaptiveAcquisitionResult{},err}
	if err:=r.Methods.Install(method);err!=nil{return AdaptiveAcquisitionResult{},err};if r.PersistentMethods!=nil{if err:=r.PersistentMethods.Install(method);err!=nil{return AdaptiveAcquisitionResult{},err}}
	for _,e:=range evals{r.History=append(r.History,AcquisitionExperience{TaskStructure:telemetry.TaskID,Method:e.Candidate.Name,SearchAttempts:1,Cost:e.Cost,Verified:e.Verified,TransferScore:boolScore(e.Transfer),Provenance:e.Candidate.Provenance})}
	candidates,err:=r.Methods.Apply(futureSpec);if err!=nil{return AdaptiveAcquisitionResult{},err};for i,c:=range candidates{p,e:=(UniversalProgramBuilder{}).Build(c,futureSpec);if e!=nil{continue};if !programFitsJSON(p.Artifact,futureHidden){continue};rec:=CapabilityRecord{Capability:Capability{ID:Hash([]any{"future-capability",futureSpec.ID,method.ID}),Name:futureSpec.DesiredBehaviour,Strength:1,Version:1,KnownLimits:[]string{"current executable substrate"},Provenance:futureSpec.Provenance},Artifact:p.Artifact,Tests:futureHidden,Mechanism:c.Mechanism,ArchitectureCost:float64(i+1)};r.History=append(r.History,AcquisitionExperience{TaskStructure:Hash([]any{futureSpec.Inputs,futureSpec.Outputs,futureSpec.Invariants}),Method:method.Name,SearchAttempts:i+1,Cost:c.Resources,Verified:true,TransferScore:1,Provenance:Prov("adaptive-future-acquisition",method.ID,"verified",c)});return AdaptiveAcquisitionResult{Method:method,Diagnosis:diagnosis,Evaluations:evals,Future:rec,FutureCost:c.Resources,Trace:append([]string(nil),r.Methods.Trace...)},nil};return AdaptiveAcquisitionResult{},errors.New("installed acquisition method could not acquire future capability")
}
func boolScore(v bool)float64{if v{return 1};return 0}
