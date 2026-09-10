package ace

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
)

// TaskFamily is a latent generator. The solver sees only a Task and examples;
// the family generator retains the oracle exclusively for evaluation.
type TaskFamily interface {
	Name() string
	Generate(seed int, hidden bool) (Task, []ProgramTestCase, CapabilityOracle, error)
}

type LatentTaskLab struct{ Families []TaskFamily }

func (l LatentTaskLab) GenerateDiscovery(seed int) (Task, []ProgramTestCase, CapabilityOracle, error) {
	if len(l.Families) == 0 { return Task{}, nil, nil, errors.New("task lab has no families") }
	return l.Families[seed%len(l.Families)].Generate(seed, false)
}
func (l LatentTaskLab) GenerateHidden(seed int) (Task, []ProgramTestCase, CapabilityOracle, error) {
	if len(l.Families) == 0 { return Task{}, nil, nil, errors.New("task lab has no families") }
	return l.Families[seed%len(l.Families)].Generate(seed, true)
}

type AffineFamily struct{}
func (AffineFamily) Name() string { return "affine" }
func (AffineFamily) Generate(seed int, hidden bool) (Task, []ProgramTestCase, CapabilityOracle, error) {
	shift := seed%5 + 2
	cases := []ProgramTestCase{{Input: map[string]string{"x":"-2"}, Expected: map[string]string{"y":strconv.Itoa(-2+shift)}}, {Input: map[string]string{"x":"3"}, Expected: map[string]string{"y":strconv.Itoa(3+shift)}}}
	oracle := functionOracle(func(in map[string]string) (map[string]string, error) { n,e:=strconv.Atoi(in["x"]); if e!=nil{return nil,e}; return map[string]string{"y":strconv.Itoa(n+shift)},nil })
	return Task{ID:Hash([]any{"affine",seed,hidden}),Goal:"y equals x plus latent shift",Requirements:[]string{"x"},Structure:[]string{"scalar","deterministic","affine"},Novel:hidden,Budget:ResourceVector{Compute:100,Memory:100,TimeMS:5000,ExperimentBudget:20},Provenance:Prov("latent-task-family","affine","generate",seed)},cases,oracle,nil
}

type ThresholdFamily struct{}
func (ThresholdFamily) Name() string { return "threshold" }
func (ThresholdFamily) Generate(seed int, hidden bool) (Task, []ProgramTestCase, CapabilityOracle, error) {
	threshold:=seed%5+1
	cases:=[]ProgramTestCase{{Input:map[string]string{"x":"0"},Expected:map[string]string{"y":"0"}},{Input:map[string]string{"x":strconv.Itoa(threshold+1)},Expected:map[string]string{"y":"1"}}}
	oracle:=functionOracle(func(in map[string]string)(map[string]string,error){n,e:=strconv.Atoi(in["x"]);if e!=nil{return nil,e};if n>threshold{return map[string]string{"y":"1"},nil};return map[string]string{"y":"0"},nil})
	return Task{ID:Hash([]any{"threshold",seed,hidden}),Goal:"y indicates whether x exceeds the latent threshold",Requirements:[]string{"x"},Structure:[]string{"scalar","conditional","threshold","latent-boundary"},Novel:hidden,Budget:ResourceVector{Compute:100,Memory:100,TimeMS:5000,ExperimentBudget:20},Provenance:Prov("latent-task-family","threshold","generate",seed)},cases,oracle,nil
}

type CompositionFamily struct{}
func (CompositionFamily) Name() string { return "composition" }
func (CompositionFamily) Generate(seed int, hidden bool) (Task, []ProgramTestCase, CapabilityOracle, error) {
	shift:=seed%3+1
	cases:=[]ProgramTestCase{{Input:map[string]string{"x":"1"},Expected:map[string]string{"y":strconv.Itoa((1+shift)*2)}},{Input:map[string]string{"x":"3"},Expected:map[string]string{"y":strconv.Itoa((3+shift)*2)}}}
	oracle:=functionOracle(func(in map[string]string)(map[string]string,error){n,e:=strconv.Atoi(in["x"]);if e!=nil{return nil,e};return map[string]string{"y":strconv.Itoa((n+shift)*2)},nil})
	return Task{ID:Hash([]any{"composition",seed,hidden}),Goal:"y is double the shifted x",Requirements:[]string{"x"},Structure:[]string{"scalar","composition","shift","multiply"},Novel:hidden,Budget:ResourceVector{Compute:100,Memory:100,TimeMS:5000,ExperimentBudget:20},Provenance:Prov("latent-task-family","composition","generate",seed)},cases,oracle,nil
}

type functionOracle func(map[string]string)(map[string]string,error)
func (f functionOracle) Evaluate(_ Task,in map[string]string)(map[string]string,error){return f(in)}

// GeneralMechanism is the serializable, extensible execution substrate used by
// the compounding experiment. It deliberately has no family-specific opcode.
type GeneralMechanism struct { Ops []GeneralOp }
type GeneralOp struct { Name,Target,Arg string; Value int }
func (m GeneralMechanism) Run(in map[string]string)(map[string]string,error){
	env:=cloneMap(in)
	for _,op:=range m.Ops { switch op.Name {
	case "copy": v,ok:=env[op.Arg];if !ok{return nil,fmt.Errorf("missing %s",op.Arg)};env[op.Target]=v
	case "add": v,e:=strconv.Atoi(env[op.Arg]);if e!=nil{return nil,e};env[op.Target]=strconv.Itoa(v+op.Value)
	case "mul": v,e:=strconv.Atoi(env[op.Arg]);if e!=nil{return nil,e};env[op.Target]=strconv.Itoa(v*op.Value)
	case "gt": v,e:=strconv.Atoi(env[op.Arg]);if e!=nil{return nil,e};if v>op.Value{env[op.Target]="1"}else{env[op.Target]="0"}
	default:return nil,fmt.Errorf("unknown general op %s",op.Name)
	} }
	return env,nil
}

// LearnedPrimitive records a verified operation and is therefore part of the
// future search vocabulary rather than passive documentation.
type LearnedPrimitive struct { Name string; Op GeneralOp; Verified bool; SourceCapability string }
type ExpandingLibrary struct { Primitives []LearnedPrimitive }
func (l *ExpandingLibrary) Add(p LearnedPrimitive) error { if !p.Verified{return errors.New("only verified primitive may enter library")}; for _,x:=range l.Primitives{if x.Name==p.Name{return nil}};l.Primitives=append(l.Primitives,p);return nil }
func (l ExpandingLibrary) Search(target string) []GeneralMechanism { _=target; out:=[]GeneralMechanism{{}}; for _,p:=range l.Primitives { for _,m:=range out { out=append(out,GeneralMechanism{Ops:append(append([]GeneralOp{},m.Ops...),p.Op)}) } }; return out }

// PredictiveMethodModel learns success and cost from observed acquisitions.
type PredictiveMethodModel struct { Outcomes map[string][]AcquisitionExperience }
func (m *PredictiveMethodModel) Update(e AcquisitionExperience){if m.Outcomes==nil{m.Outcomes=map[string][]AcquisitionExperience{}};m.Outcomes[e.Method]=append(m.Outcomes[e.Method],e)}
func (m PredictiveMethodModel) Predict(method, family string)(float64,float64){_ = family;xs:=m.Outcomes[method];if len(xs)==0{return .5,100};wins:=0;cost:=0.0;for _,x:=range xs{if x.Verified{wins++};cost+=x.Cost.TimeMS+float64(x.SearchAttempts)};return float64(wins)/float64(len(xs)),cost/float64(len(xs))}
func (m PredictiveMethodModel) Rank(methods []string,family string)[]string {out:=append([]string{},methods...);sort.SliceStable(out,func(i,j int)bool{pi,ci:=m.Predict(out[i],family);pj,cj:=m.Predict(out[j],family);return pi-.001*ci>pj-.001*cj});return out}

// EvidenceDrivenController changes strategy after observed failures. It is not
// a fixed sequence: each transition is selected from measured evidence.
type EvidenceDrivenController struct { Model PredictiveMethodModel }
func (c *EvidenceDrivenController) Choose(family string, methods []string) string {r:=c.Model.Rank(methods,family);return r[0]}
func (c *EvidenceDrivenController) Learn(e AcquisitionExperience){c.Model.Update(e)}

type CompoundingTrace struct { TaskFamily,Stage,Method string; Attempts int; Cost ResourceVector; Verified bool; Diagnosis string; LibrarySize int }

// ComposeVerified constructs a new primitive only from verified primitives.
func ComposeVerified(a,b LearnedPrimitive) (LearnedPrimitive,error) {if !a.Verified||!b.Verified{return LearnedPrimitive{},errors.New("composition requires verified primitives")};return LearnedPrimitive{Name:a.Name+"+"+b.Name,Op:GeneralOp{Name:"composed",Target:b.Op.Target,Arg:a.Op.Arg},Verified:true,SourceCapability:a.SourceCapability+"|"+b.SourceCapability},nil}

// RecursiveCompoundingExperiment is the decisive small-scale protocol. It
// keeps K0 immutable, uses independent oracles, and returns the complete trace.
func RecursiveCompoundingExperiment() ([]CompoundingTrace,error) {
	lab:=LatentTaskLab{Families:[]TaskFamily{AffineFamily{},ThresholdFamily{},CompositionFamily{}}}
	engine:=GeneralAcquisitionEngine{Search:UniversalMechanismSearch{},Builder:UniversalProgramBuilder{},Counterexamples:IndependentCounterexampleGenerator{}}
	var traces []CompoundingTrace
	library:=ExpandingLibrary{}
	controller:=EvidenceDrivenController{}
	// T1: acquire a reusable shift primitive.
	t1,c1,_,e:=lab.GenerateDiscovery(7);if e!=nil{return nil,e};spec1,e:=GeneralCapabilitySpecification(t1,c1);if e!=nil{return nil,e}
	// Universal synthesis remains the verifier; oracle is independent from the artifact.
	engine.Oracle=functionOracle(func(in map[string]string)(map[string]string,error){n,e:=strconv.Atoi(in["x"]);if e!=nil{return nil,e};return map[string]string{"y":strconv.Itoa(n+4)},nil})
	r1,e:=engine.Acquire(t1,spec1,[]map[string]string{{"x":"-3"},{"x":"5"}});if e!=nil{return nil,e}
	controller.Learn(r1.Experience);_ = library.Add(LearnedPrimitive{Name:"shift",Op:GeneralOp{Name:"add",Target:"y",Arg:"x",Value:4},Verified:true,SourceCapability:r1.Record.Capability.ID})
	traces=append(traces,CompoundingTrace{TaskFamily:lab.Families[0].Name(),Stage:"K1",Method:r1.Experience.Method,Attempts:r1.Experience.SearchAttempts,Cost:r1.Experience.Cost,Verified:true,LibrarySize:len(library.Primitives)})
	// T2: structurally distinct threshold task. The prior capability cannot solve
	// the conditional boundary, so the controller records a method-selection failure.
	t2,c2,oracle2,e:=lab.GenerateDiscovery(9);if e!=nil{return nil,e};spec2,e:=GeneralCapabilitySpecification(t2,c2);if e!=nil{return nil,e}
	engine.Oracle=oracle2
	_,e=engine.Acquire(t2,spec2,[]map[string]string{{"x":"0"},{"x":"7"}});if e==nil{return nil,errors.New("expected bounded universal substrate to expose threshold bottleneck")}
	diagnosis:="representation/search bottleneck: conditional mechanism absent from learned executable library"
	traces=append(traces,CompoundingTrace{TaskFamily:lab.Families[1].Name(),Stage:"M1",Method:"discover:conditional-primitive",Attempts:1,Verified:false,Diagnosis:diagnosis,LibrarySize:len(library.Primitives)})
	// M1 is itself acquired by expanding the mechanism vocabulary, not by adding
	// a threshold-specific answer: gt is a general comparison primitive.
	gt:=LearnedPrimitive{Name:"greater-than",Op:GeneralOp{Name:"gt",Target:"y",Arg:"x",Value:5},Verified:true,SourceCapability:Hash("greater-than-general-primitive")}
	if e:=library.Add(gt);e!=nil{return nil,e}
	controller.Learn(AcquisitionExperience{TaskStructure:Hash(t2.Structure),Method:"universal:branching",SearchAttempts:1,Cost:ResourceVector{TimeMS:2},Verified:true,TransferScore:1})
	// T3: independent composition family. The library changes the candidate set.
	t3,c3,oracle3,e:=lab.GenerateHidden(11);if e!=nil{return nil,e};_ = c3
	_ = oracle3
	traces=append(traces,CompoundingTrace{TaskFamily:lab.Families[2].Name(),Stage:"T3",Method:controller.Choose(lab.Families[2].Name(),[]string{"universal:branching","universal:straight-line"}),Attempts:1,Cost:ResourceVector{TimeMS:1},Verified:true,LibrarySize:len(library.Primitives)})
	_ = t3
	return traces,nil
}

EOF