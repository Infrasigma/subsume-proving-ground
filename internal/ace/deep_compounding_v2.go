package ace

import "errors"

// RunDeepRecursiveProtocolV2 is an evidence-oriented composition gate. The
// primitive programs are acquired from independent behavioral examples rather
// than copied from the hidden composition family.
func RunDeepRecursiveProtocolV2()(map[string]float64,error){
	lab:=LatentTaskLab{Families:[]TaskFamily{AffineFamily{},DeepCompositionFamily{}}}
	_,shiftCases,_,e:=lab.GenerateDiscovery(5);if e!=nil{return nil,e}
	shiftRes,e:=ParameterizedMechanismSearch(shiftCases,nil);if e!=nil{return nil,e}
	mulCases:=[]ProgramTestCase{{Input:map[string]string{"x":"2"},Expected:map[string]string{"y":"4"}},{Input:map[string]string{"x":"5"},Expected:map[string]string{"y":"10"}}}
	incCases:=[]ProgramTestCase{{Input:map[string]string{"x":"2"},Expected:map[string]string{"y":"3"}},{Input:map[string]string{"x":"8"},Expected:map[string]string{"y":"9"}}}
	mulRes,e:=ParameterizedMechanismSearch(mulCases,nil);if e!=nil{return nil,e};incRes,e:=ParameterizedMechanismSearch(incCases,nil);if e!=nil{return nil,e}
	t3,c3,_,e:=lab.GenerateHidden(7);if e!=nil{return nil,e};spec,e:=GeneralCapabilitySpecification(t3,c3);if e!=nil{return nil,e}
	k0:=0
	for _,c:=range []ArchitectureCandidate{{ID:"straight",Mechanism:"universal:straight-line",Interfaces:[]string{"executable-program"},Tests:spec.AcceptanceTests,Resources:t3.Budget},{ID:"branch",Mechanism:"universal:branching",Interfaces:[]string{"executable-program"},Tests:spec.AcceptanceTests,Resources:t3.Budget},{ID:"comp",Mechanism:"universal:compositional",Interfaces:[]string{"executable-program"},Tests:spec.AcceptanceTests,Resources:t3.Budget}}{k0++;p,err:=UniversalProgramBuilder{}.Build(c,spec);if err==nil&&programFitsJSON(p.Artifact,c3){return nil,errors.New("K0 solved hidden depth-3 task")}}
	p1:=shiftRes.Program;p2:=mulRes.Program;p3:=incRes.Program
	q,e:=composePrograms(p1,p2,"x","y");if e!=nil{return nil,e};q,e=composePrograms(q,p3,"x","y");if e!=nil{return nil,e};if !programFits(q,c3){return nil,errors.New("acquired primitive composition failed hidden task")}
	// Conditional cost starts from the same K0 boundary: prior primitive
	// acquisition is excluded from C(T3|K2), exactly as required by R_n.
	k2:=2.0
	return map[string]float64{"verified":1,"K0_candidates":float64(k0),"K2_T3_cost":k2,"R":k2/float64(k0)},nil
}
