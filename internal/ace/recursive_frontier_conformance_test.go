package ace

import("sort";"testing")

// canonicalFrontierBehavior reduces candidate programs to observable behavior
// on a fixed probe family. Candidate ordering is intentionally ignored.
func canonicalFrontierBehavior(candidates []ArchitectureCandidate,s CapabilitySpecification,probes []ProgramTestCase)map[string]bool{out:=map[string]bool{};for _,c:=range candidates{p,err:=(UniversalProgramBuilder{}).Build(c,s);if err!=nil{continue};sig:=c14nBehaviorSignature(pArtifactProgram(p.Artifact),probes);out[sig]=true};return out}

func c14nBehaviorSignature(p UniversalProgram,probes []ProgramTestCase)string{parts:=make([]string,0,len(probes));for _,tc:=range probes{got,err:=p.Run(tc.Input);if err!=nil{parts=append(parts,"ERR");continue};keys:=make([]string,0,len(got));for k:=range got{keys=append(keys,k)};sort.Strings(keys);s:="";for _,k:=range keys{s+=k+"="+got[k]+";"};parts=append(parts,s)};return Hash([]any{parts})}

func TestEffectiveFrontierControlsDoNotCountReordering(t *testing.T){
	cases:=[]ProgramTestCase{methodInputOutputExample(0,0),methodInputOutputExample(1,1),methodInputOutputExample(2,2)}
	s,err:=GeneralCapabilitySpecification(Task{ID:"frontier-control",Goal:"identity",Requirements:[]string{"x"},Structure:[]string{"scalar"},Budget:ResourceVector{Compute:20,Memory:20,TimeMS:1000,ExperimentBudget:5}},cases);if err!=nil{t.Fatal(err)}
	base,err:=(UniversalMechanismSearch{}).SearchMechanisms(s,s.ResourceLimits);if err!=nil{t.Fatal(err)}
	reversed:=append([]ArchitectureCandidate(nil),base...);for i,j:=0,len(reversed)-1;i<j;i,j=i+1,j-1{reversed[i],reversed[j]=reversed[j],reversed[i]}
	f0:=canonicalFrontierBehavior(base,s,cases);f1:=canonicalFrontierBehavior(reversed,s,cases);if len(f0)!=len(f1){t.Fatalf("reorder-only changed effective frontier cardinality: %d vs %d",len(f0),len(f1))};for k:=range f0{if !f1[k]{t.Fatalf("reorder-only changed behavioral frontier")}}
}

func TestRecursiveCapabilityFrontierConformance(t *testing.T){
	_,err:=RunRecursiveCapabilityProtocolV3()
	if err!=nil{t.Fatalf("FRONTIER_EXPANSION_NOT_ESTABLISHED: primary boundary=runtime recursive acquisition path: %v",err)}
}
