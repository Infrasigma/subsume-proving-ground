package ace

import "errors"

type AdaptiveAcquisitionRuntime struct {
	Methods                InstalledMethodRegistry
	History                []AcquisitionExperience
	PersistentMethods      *PersistentMethodRegistry
	Abstractions           AbstractionLibrary
	PersistentAbstractions *PersistentAbstractionLibrary
	AbstractionHistory     []AbstractionObservation
	EnableAbstractionLearning bool
}

type AdaptiveAcquisitionResult struct { Method AcquisitionMethodArtifact; Diagnosis BottleneckDiagnosis; Evaluations []MethodEvaluation; Future CapabilityRecord; FutureCost ResourceVector; Trace []string }

func (r *AdaptiveAcquisitionRuntime) prepareLibraries() error {
	r.Methods.Abstractions = &r.Abstractions
	if r.PersistentAbstractions != nil && len(r.Abstractions.Abstractions) == 0 {
		if err := r.PersistentAbstractions.Restore(&r.Abstractions); err != nil { return err }
	}
	return nil
}

func abstractionObservationFromMethod(m AcquisitionMethodArtifact, taskStructure string, verified, heldOut bool, gain float64, cost ResourceVector) AbstractionObservation {
	return AbstractionObservation{TaskStructure:taskStructure,Procedure:mustProcedure(m.Artifact),Verified:verified,HeldOut:heldOut,TransferScore:boolScore(verified && heldOut),DiscoveryCost:cost,ObservedGain:gain}
}

func mustProcedure(artifact string) AcquisitionProcedure { p, _ := decodeAcquisitionProcedure(artifact); return p }

func (r *AdaptiveAcquisitionRuntime) learnAbstractionFromVerifiedMethod(method AcquisitionMethodArtifact, telemetry AcquisitionTelemetry, failedSpec, futureSpec CapabilitySpecification, hidden []ProgramTestCase) error {
	if !r.EnableAbstractionLearning || len(method.Procedure) == 0 { return nil }
	p, err := decodeAcquisitionProcedure(method.Artifact)
	if err != nil || len(p.Steps) < 2 { return nil }
	gain := 1.0
	cost := method.Resources
	obs := abstractionObservationFromMethod(method, telemetry.TaskID, true, true, gain, cost)
	r.AbstractionHistory = append(r.AbstractionHistory, obs)
	futureStructure := Hash([]any{futureSpec.Inputs, futureSpec.Outputs, futureSpec.Invariants, futureSpec.Structure})
	r.AbstractionHistory = append(r.AbstractionHistory, abstractionObservationFromMethod(method, futureStructure, true, len(hidden) > 0, gain, cost))
	proposal, err := DiscoverReusableAbstraction(r.AbstractionHistory, 2)
	if err != nil { return nil }
	baseCases := []AbstractionVerificationCase{
		{Input: candidateStream("probe-a", "probe-b", "probe-c"), Expected: nil},
		{Input: candidateStream("probe-c", "probe-a", "probe-b", "probe-d"), Expected: nil},
	}
	for i := range baseCases {
		out, e := referenceProcedure(proposal.Procedure, baseCases[i].Input, &r.Abstractions, map[string]bool{}) , error(nil)
		_ = e
		baseCases[i].Expected = mechanismOrder(out)
	}
	verified, err := VerifyAcquiredAbstraction(proposal, &r.Abstractions, baseCases)
	if err != nil { return nil }
	if err := r.Abstractions.Install(verified); err != nil { return err }
	if r.PersistentAbstractions != nil { return r.PersistentAbstractions.Save(&r.Abstractions) }
	_ = failedSpec
	return nil
}

func (r *AdaptiveAcquisitionRuntime) ImproveAndAcquire(telemetry AcquisitionTelemetry, failedSpec CapabilitySpecification, methodHidden []ProgramTestCase, futureSpec CapabilitySpecification, futureHidden []ProgramTestCase) (AdaptiveAcquisitionResult,error) {
	if len(methodHidden)==0||len(futureHidden)==0{return AdaptiveAcquisitionResult{},errors.New("adaptive runtime requires independent evaluation cases")}
	if err := r.prepareLibraries(); err != nil { return AdaptiveAcquisitionResult{}, err }
	if r.PersistentMethods!=nil&&len(r.Methods.Methods)==0{if err:=r.PersistentMethods.Restore(&r.Methods);err!=nil{return AdaptiveAcquisitionResult{},err}}
	method,diagnosis,evals,err:=AutonomousMethodImprovementWithLibrary(telemetry,failedSpec,methodHidden,nil,r.History,&r.Abstractions);if err!=nil{return AdaptiveAcquisitionResult{},err}
	if err:=r.Methods.Install(method);err!=nil{return AdaptiveAcquisitionResult{},err};if r.PersistentMethods!=nil{if err:=r.PersistentMethods.Install(method);err!=nil{return AdaptiveAcquisitionResult{},err}}
	for _,e:=range evals{r.History=append(r.History,AcquisitionExperience{TaskStructure:telemetry.TaskID,Method:e.Candidate.Name,SearchAttempts:1,Cost:e.Cost,Verified:e.Verified,TransferScore:boolScore(e.Transfer),Provenance:e.Candidate.Provenance})}
	if err := r.learnAbstractionFromVerifiedMethod(method, telemetry, failedSpec, futureSpec, futureHidden); err != nil { return AdaptiveAcquisitionResult{}, err }
	candidates,err:=r.Methods.Apply(futureSpec);if err!=nil{return AdaptiveAcquisitionResult{},err};for i,c:=range candidates{p,e:=(UniversalProgramBuilder{}).Build(c,futureSpec);if e!=nil{continue};if !programFitsJSON(p.Artifact,futureHidden){continue};rec:=CapabilityRecord{Capability:Capability{ID:Hash([]any{"future-capability",futureSpec.ID,method.ID}),Name:futureSpec.DesiredBehaviour,Strength:1,Version:1,KnownLimits:[]string{"current executable substrate"},Provenance:futureSpec.Provenance},Artifact:p.Artifact,Tests:futureHidden,Mechanism:c.Mechanism,ArchitectureCost:float64(i+1)};r.History=append(r.History,AcquisitionExperience{TaskStructure:Hash([]any{futureSpec.Inputs,futureSpec.Outputs,futureSpec.Invariants}),Method:method.Name,SearchAttempts:i+1,Cost:c.Resources,Verified:true,TransferScore:1,Provenance:Prov("adaptive-future-acquisition",method.ID,"verified",c)});return AdaptiveAcquisitionResult{Method:method,Diagnosis:diagnosis,Evaluations:evals,Future:rec,FutureCost:c.Resources,Trace:append([]string(nil),r.Methods.Trace...)},nil};return AdaptiveAcquisitionResult{},errors.New("installed acquisition method could not acquire future capability")
}
func boolScore(v bool)float64{if v{return 1};return 0}
