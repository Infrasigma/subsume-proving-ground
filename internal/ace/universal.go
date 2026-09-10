package ace

import (
 "encoding/json"
 "errors"
 "fmt"
 "strconv"
 "strings"
)

type UExpr struct { Kind string `json:"kind"`; Value string `json:"value,omitempty"`; Left, Right *UExpr `json:"left,omitempty"` }
type UStmt struct { Kind string `json:"kind"`; Target string `json:"target,omitempty"`; Expr *UExpr `json:"expr,omitempty"`; Cond *UExpr `json:"cond,omitempty"`; Then, Else []UStmt `json:"then,omitempty"`; Body []UStmt `json:"body,omitempty"`; Count int `json:"count,omitempty"` }
type UniversalProgram struct { Statements []UStmt `json:"statements"` }

func (e UExpr) eval(env map[string]string) (int, bool, error) {
 switch e.Kind {
 case "const": n,err:=strconv.Atoi(e.Value); return n,err==nil,err
 case "var": v,ok:=env[e.Value]; if !ok{return 0,false,fmt.Errorf("missing variable %s",e.Value)}; n,err:=strconv.Atoi(v); return n,err==nil,err
 case "add","sub","mul":
  if e.Left==nil||e.Right==nil{return 0,false,errors.New("binary expression missing operand")}
  a,_,err:=e.Left.eval(env);if err!=nil{return 0,false,err}; b,_,err:=e.Right.eval(env);if err!=nil{return 0,false,err}
  if e.Kind=="add"{return a+b,true,nil};if e.Kind=="sub"{return a-b,true,nil};return a*b,true,nil
 case "lt","eq","and","or":
  if e.Left==nil||e.Right==nil{return 0,false,errors.New("boolean expression missing operand")}
  a,_,err:=e.Left.eval(env);if err!=nil{return 0,false,err};b,_,err:=e.Right.eval(env);if err!=nil{return 0,false,err}
  switch e.Kind {case "lt":if a<b{return 1,true,nil};case "eq":if a==b{return 1,true,nil};case "and":if a!=0&&b!=0{return 1,true,nil};case "or":if a!=0||b!=0{return 1,true,nil}}
  return 0,true,nil
 case "not":
  if e.Left==nil{return 0,false,errors.New("not missing operand")};a,_,err:=e.Left.eval(env);if err!=nil{return 0,false,err};if a==0{return 1,true,nil};return 0,true,nil
 default:return 0,false,fmt.Errorf("unknown expression %s",e.Kind)
 }
}

func (p UniversalProgram) Run(input map[string]string) (map[string]string,error) {
 env:=map[string]string{};for k,v:=range input{env[k]=v};steps:=0
 var exec func([]UStmt) error
 exec=func(ss []UStmt) error {
  for _,s:=range ss {
   steps++;if steps>1000{return errors.New("program step limit exceeded")}
   switch s.Kind {
   case "assign":
    if s.Expr==nil{return errors.New("assign missing expression")};n,_,err:=s.Expr.eval(env);if err!=nil{return err};env[s.Target]=strconv.Itoa(n)
   case "if":
    if s.Cond==nil{return errors.New("if missing condition")};n,_,err:=s.Cond.eval(env);if err!=nil{return err};if n!=0{if err:=exec(s.Then);err!=nil{return err}}else{if err:=exec(s.Else);err!=nil{return err}}
   case "repeat":
    if s.Count<0||s.Count>100{return errors.New("invalid repeat count")};for i:=0;i<s.Count;i++{if err:=exec(s.Body);err!=nil{return err}}
   default:return fmt.Errorf("unknown statement %s",s.Kind)
   }
  }
  return nil
 }
 if err:=exec(p.Statements);err!=nil{return nil,err};return env,nil
}

func programFits(p UniversalProgram,cases []ProgramTestCase) bool {for _,tc:=range cases{got,err:=p.Run(tc.Input);if err!=nil{return false};for k,v:=range tc.Expected{if got[k]!=v{return false}}};return true}

func GeneralCapabilitySpecification(t Task,cases []ProgramTestCase)(CapabilitySpecification,error) {
 if len(cases)<2{return CapabilitySpecification{},errors.New("general capability requires at least two examples")}
 inputs:=map[string]bool{};outputs:=map[string]bool{};for _,c:=range cases{for k:=range c.Input{inputs[k]=true};for k:=range c.Expected{outputs[k]=true}}
 in,out:=make([]string,0,len(inputs)),make([]string,0,len(outputs));for k:=range inputs{in=append(in,k)};for k:=range outputs{out=append(out,k)}
 return CapabilitySpecification{ID:Hash([]any{"general",t.ID,cases}),DesiredBehaviour:t.Goal,Inputs:in,Outputs:out,Invariants:[]string{"all observed examples preserved"},AcceptanceTests:[]string{"observed-examples","held-out-execution"},ResourceLimits:t.Budget,FailureCriteria:[]string{"wrong output","runtime error"},RegressionConstraints:[]string{"existing capabilities preserved"},KnownExamples:cases,Provenance:Prov("capability-discovery",t.ID,"infer-from-examples",cases)},nil
}

type UniversalMechanismSearch struct{}
func (UniversalMechanismSearch) SearchMechanisms(s CapabilitySpecification,b ResourceVector)([]ArchitectureCandidate,error) {strategies:=[]string{"universal:straight-line","universal:branching","universal:compositional"};out:=make([]ArchitectureCandidate,0,len(strategies));for _,strategy:=range strategies{out=append(out,ArchitectureCandidate{ID:Hash([]any{s.ID,strategy}),Mechanism:strategy,Interfaces:[]string{"typed-key-value-input","executable-program"},Advantage:"compositional synthesis",Assumptions:"bounded integer/boolean primitives",Resources:b,Tests:s.AcceptanceTests,RegressionRisks:s.RegressionConstraints,Provenance:Prov("architecture-search",s.ID,"strategy",strategy)})};return out,nil}

type UniversalProgramBuilder struct{}
func (UniversalProgramBuilder) Build(c ArchitectureCandidate,s CapabilitySpecification)(ModificationProposal,error) {
 if len(s.KnownExamples)<2||len(s.Inputs)==0||len(s.Outputs)==0{return ModificationProposal{},errors.New("insufficient behavioral evidence")}
 vars:=append([]string{},s.Inputs...);out:=s.Outputs[0];var exprs []UExpr;for _,v:=range vars{exprs=append(exprs,UExpr{Kind:"var",Value:v})};for n:=-2;n<=2;n++{exprs=append(exprs,UExpr{Kind:"const",Value:strconv.Itoa(n)})}
 base:=append([]UExpr{},exprs...);for _,a:=range base{for _,b:=range base{aa,bb:=a,b;exprs=append(exprs,UExpr{Kind:"add",Left:&aa,Right:&bb},UExpr{Kind:"sub",Left:&aa,Right:&bb},UExpr{Kind:"mul",Left:&aa,Right:&bb})}}
 if strings.HasPrefix(c.Mechanism,"universal:branching")||strings.HasPrefix(c.Mechanism,"universal:compositional") {
  for _,v:=range vars {for _,cmp:=range []string{"lt","eq"} {for _,n:=range []int{-1,0,1} {cc:=UExpr{Kind:cmp,Left:&UExpr{Kind:"var",Value:v},Right:&UExpr{Kind:"const",Value:strconv.Itoa(n)}};for _,te0:=range exprs {for _,ee0:=range exprs {te,ee:=te0,ee0;p:=UniversalProgram{Statements:[]UStmt{{Kind:"if",Cond:&cc,Then:[]UStmt{{Kind:"assign",Target:out,Expr:&te}},Else:[]UStmt{{Kind:"assign",Target:out,Expr:&ee}}}}};if programFits(p,s.KnownExamples){return encodeUniversal(p,s,c)}}}}}}
 }
 for _,e:=range exprs {ee:=e;p:=UniversalProgram{Statements:[]UStmt{{Kind:"assign",Target:out,Expr:&ee}}};if programFits(p,s.KnownExamples){return encodeUniversal(p,s,c)}}
 return ModificationProposal{},errors.New("universal synthesis exhausted search space")
}
func encodeUniversal(p UniversalProgram,s CapabilitySpecification,c ArchitectureCandidate)(ModificationProposal,error){b,err:=json.Marshal(p);if err!=nil{return ModificationProposal{},err};return ModificationProposal{ID:Hash([]any{c,s,p}),Capability:s,Candidate:c,Artifact:string(b),Provenance:Prov("mechanism-builder",c.ID,"synthesize-universal-program",p)},nil}
func GenerateAdversarialCases(s CapabilitySpecification)[]ProgramTestCase{out:=make([]ProgramTestCase,0,8);for _,c:=range s.KnownExamples{for k,v:=range c.Input{n,err:=strconv.Atoi(v);if err!=nil{continue};for _,d:=range []int{-1,1}{in:=map[string]string{};for ik,iv:=range c.Input{in[ik]=iv};in[k]=strconv.Itoa(n+d);out=append(out,ProgramTestCase{Input:in})}}};return out}
