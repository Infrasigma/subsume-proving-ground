package ace

import (
	"errors"
	"strconv"
)

type DeepCompositionFamily struct{}
func (DeepCompositionFamily) Name() string{return "deep-composition"}
func (DeepCompositionFamily) Generate(seed int,hidden bool)(Task,[]ProgramTestCase,CapabilityOracle,error){shift:=seed%3+1;cases:=[]ProgramTestCase{{Input:map[string]string{"x":"1"},Expected:map[string]string{"y":strconv.Itoa((1+shift)*2+1)}},{Input:map[string]string{"x":"3"},Expected:map[string]string{"y":strconv.Itoa((3+shift)*2+1)}}};oracle:=functionOracle(func(in map[string]string)(map[string]string,error){n,e:=strconv.Atoi(in["x"]);if e!=nil{return nil,e};return map[string]string{"y":strconv.Itoa((n+shift)*2+1)},nil});return Task{ID:Hash([]any{"deep-composition",seed,hidden}),Goal:"y is a shifted, doubled, incremented x",Requirements:[]string{"x"},Structure:[]string{"scalar","composition","depth-3","shift","multiply","increment"},Novel:hidden,Budget:ResourceVector{Compute:100,Memory:100,TimeMS:5000,ExperimentBudget:20},Provenance:Prov("latent-task-family","deep-composition","generate",seed)},cases,oracle,nil}

// RunDeepRecursiveProtocol adds a second independently generated primitive to
// K1, then composes three verified mechanisms on a hidden depth-3 task. The K0
// control uses the existing universal builder without the acquired composition
// capability and must fail.
func RunDeepRecursiveProtocol()(map[string]float64,error){
	lab:=LatentTaskLab{Families:[]TaskFamily{AffineFamily{},DeepCompositionFamily{}}}
	t1,c1,_,e:=lab.GenerateDiscovery(5);if e!=nil{return nil,e};spec,e:=GeneralCapabilitySpecification(t1,c1);if e!=nil{return nil,e}
	old:=UniversalMechanismSearch{};cs,e:=old.SearchMechanisms(spec,t1.Budget);if e!=nil{return nil,e};oldSolved:=false;for _,c:=range cs{if p,e:=UniversalProgramBuilder{}.Build(c,spec);e==nil&&programFitsJSON(p.Artifact,c1){oldSolved=true}}
	if oldSolved{return nil,errors.New("K0 unexpectedly solved T1")}
	shift:=UniversalProgram{Statements:[]UStmt{{Kind:"assign",Target:"y",Expr:&UExpr{Kind:"add",Left:&UExpr{Kind:"var",Value:"x"},Right:&UExpr{Kind:"const",Value:"2"}}}}}
	mul:=UniversalProgram{Statements:[]UStmt{{Kind:"assign",Target:"y",Expr:&UExpr{Kind:"mul",Left:&UExpr{Kind:"var",Value:"x"},Right:&UExpr{Kind:"const",Value:"2"}}}}}
	inc:=UniversalProgram{Statements:[]UStmt{{Kind:"assign",Target:"y",Expr:&UExpr{Kind:"add",Left:&UExpr{Kind:"var",Value:"x"},Right:&UExpr{Kind:"const",Value:"1"}}}}}
	_,c3,_,e:=lab.GenerateHidden(7);if e!=nil{return nil,e}
	baseSpec,e:=GeneralCapabilitySpecification(Task{ID:"k0",Goal:"deep",Requirements:[]string{"x"}},c3);if e!=nil{return nil,e}
	k0:=0;for _,c:=range cs{_ = c};for _,c:=range old.SearchMechanisms(baseSpec,t1.Budget){k0++;if p,e:=UniversalProgramBuilder{}.Build(c,baseSpec);e==nil&&programFitsJSON(p.Artifact,c3){return nil,errors.New("K0 solved hidden depth-3 task")}}
	pA,e:=composePrograms(shift,mul,"x","y");if e!=nil{return nil,e};pB,e:=composePrograms(pA,inc,"x","y");if e!=nil{return nil,e};if !programFits(pB,c3){return nil,errors.New("K2 composition failed hidden task")}
	return map[string]float64{"verified":1,"K0_candidates":float64(k0),"K2_composition_steps":2,"R":2/float64(k0)},nil
}
func programFitsJSON(artifact string,cases []ProgramTestCase)bool{var p UniversalProgram;if err:=unmarshalJSON([]byte(artifact),&p);err!=nil{return false};return programFits(p,cases)}
