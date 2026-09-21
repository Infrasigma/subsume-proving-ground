package ace

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

type f14Task struct {
	Family string
	Train []ProgramTestCase
	Hidden []ProgramTestCase
}

func f14Tasks() []f14Task {
	return []f14Task{
		{
			Family:"arithmetic-affine",
			Train:[]ProgramTestCase{
				{Input:map[string]string{"x":"-4"},Expected:map[string]string{"y":"5"}},
				{Input:map[string]string{"x":"3"},Expected:map[string]string{"y":"19"}},
				{Input:map[string]string{"x":"7"},Expected:map[string]string{"y":"31"}},
			},
			Hidden:[]ProgramTestCase{
				{Input:map[string]string{"x":"-9"},Expected:map[string]string{"y":"-5"}},
				{Input:map[string]string{"x":"12"},Expected:map[string]string{"y":"51"}},
			},
		},
		{
			Family:"conditional-threshold",
			Train:[]ProgramTestCase{
				{Input:map[string]string{"x":"-1"},Expected:map[string]string{"y":"0"}},
				{Input:map[string]string{"x":"3"},Expected:map[string]string{"y":"1"}},
				{Input:map[string]string{"x":"8"},Expected:map[string]string{"y":"1"}},
			},
			Hidden:[]ProgramTestCase{
				{Input:map[string]string{"x":"0"},Expected:map[string]string{"y":"0"}},
				{Input:map[string]string{"x":"5"},Expected:map[string]string{"y":"1"}},
			},
		},
		{
			Family:"string-vowel-transform",
			Train:[]ProgramTestCase{
				{Input:map[string]string{"x":"ace"},Expected:map[string]string{"y":"AcE"}},
				{Input:map[string]string{"x":"robot"},Expected:map[string]string{"y":"rObOt"}},
			},
			Hidden:[]ProgramTestCase{
				{Input:map[string]string{"x":"verification"},Expected:map[string]string{"y":"vErIfIcAtIOn"}},
			},
		},
		{
			Family:"sequence-reversal",
			Train:[]ProgramTestCase{
				{Input:map[string]string{"x":"a,b,c"},Expected:map[string]string{"y":"c,b,a"}},
				{Input:map[string]string{"x":"1,2,3,4"},Expected:map[string]string{"y":"4,3,2,1"}},
			},
			Hidden:[]ProgramTestCase{
				{Input:map[string]string{"x":"q,r,s,t,u"},Expected:map[string]string{"y":"u,t,s,r,q"}},
			},
		},
		{
			Family:"graph-reachability",
			Train:[]ProgramTestCase{
				{Input:map[string]string{"x":"A>B;B>C|A>C"},Expected:map[string]string{"y":"1"}},
				{Input:map[string]string{"x":"A>B;C>D|A>D"},Expected:map[string]string{"y":"0"}},
			},
			Hidden:[]ProgramTestCase{
				{Input:map[string]string{"x":"A>B;B>C;C>D|A>D"},Expected:map[string]string{"y":"1"}},
			},
		},
		{
			Family:"causal-xor",
			Train:[]ProgramTestCase{
				{Input:map[string]string{"x":"0,0"},Expected:map[string]string{"y":"0"}},
				{Input:map[string]string{"x":"0,1"},Expected:map[string]string{"y":"1"}},
				{Input:map[string]string{"x":"1,0"},Expected:map[string]string{"y":"1"}},
			},
			Hidden:[]ProgramTestCase{
				{Input:map[string]string{"x":"1,1"},Expected:map[string]string{"y":"0"}},
			},
		},
		{
			Family:"symbolic-parity-composition",
			Train:[]ProgramTestCase{
				{Input:map[string]string{"x":"2"},Expected:map[string]string{"y":"1"}},
				{Input:map[string]string{"x":"5"},Expected:map[string]string{"y":"0"}},
				{Input:map[string]string{"x":"8"},Expected:map[string]string{"y":"1"}},
			},
			Hidden:[]ProgramTestCase{
				{Input:map[string]string{"x":"11"},Expected:map[string]string{"y":"0"}},
			},
		},
		{
			Family:"compression-run-length",
			Train:[]ProgramTestCase{
				{Input:map[string]string{"x":"aaabb"},Expected:map[string]string{"y":"a3b2"}},
				{Input:map[string]string{"x":"cccc"},Expected:map[string]string{"y":"c4"}},
			},
			Hidden:[]ProgramTestCase{
				{Input:map[string]string{"x":"bbbaac"},Expected:map[string]string{"y":"b3a2c1"}},
			},
		},
	}
}

func f14GeneralSolve(ctx context.Context, task f14Task) bool {
	if ctx==nil {ctx=context.Background()}
	spec,err:=GeneralCapabilitySpecification(
		Task{ID:"f14-"+task.Family,Goal:"infer the deterministic transformation",Budget:ResourceVector{Compute:500,Memory:256,TimeMS:5000,ExperimentBudget:32}},
		task.Train,
	)
	if err!=nil{return false}
	engine:=GeneralAcquisitionEngine{
		Search:UniversalMechanismSearch{},
		Builder:UniversalProgramBuilder{},
		Counterexamples:IndependentCounterexampleGenerator{},
		Oracle:f14Oracle{family:task.Family},
	}
	result,err:=engine.Acquire(
		Task{ID:"f14-"+task.Family,Goal:"infer the deterministic transformation",Budget:spec.ResourceLimits},
		spec,
		f14SeedInputs(task.Train),
	)
	if err!=nil{return false}
	if result.Verification.Status!="verified"||!result.Verification.Independent{return false}
	var p UniversalProgram
	if err:=json.Unmarshal([]byte(result.Record.Artifact),&p);err!=nil{return false}
	return programFits(p,task.Hidden)
}

func f14SeedInputs(train []ProgramTestCase) []map[string]string {
	out:=make([]map[string]string,0,len(train))
	for _,tc:=range train{m:=map[string]string{};for k,v:=range tc.Input{m[k]=v};out=append(out,m)}
	return out
}

type f14Oracle struct{family string}

func (o f14Oracle) Evaluate(_ Task,in map[string]string)(map[string]string,error){
	x:=in["x"]
	switch o.family{
	case "arithmetic-affine":
		n,e:=strconv.Atoi(x);if e!=nil{return nil,e};return map[string]string{"y":strconv.Itoa(4*n+21)},nil
	case "conditional-threshold":
		n,e:=strconv.Atoi(x);if e!=nil{return nil,e};if n>2{return map[string]string{"y":"1"},nil};return map[string]string{"y":"0"},nil
	case "causal-xor":
		parts:=strings.Split(x,",");if len(parts)!=2{return nil,fmt.Errorf("xor arity")};a:=parts[0]=="1";b:=parts[1]=="1";if a!=b{return map[string]string{"y":"1"},nil};return map[string]string{"y":"0"},nil
	case "symbolic-parity-composition":
		n,e:=strconv.Atoi(x);if e!=nil{return nil,e};if n%2==0{return map[string]string{"y":"1"},nil};return map[string]string{"y":"0"},nil
	case "string-vowel-transform":
		return map[string]string{"y":f14UpperVowels(x)},nil
	case "sequence-reversal":
		parts:=strings.Split(x,",");for i,j:=0,len(parts)-1;i<j;i,j=i+1,j-1{parts[i],parts[j]=parts[j],parts[i]};return map[string]string{"y":strings.Join(parts,",")},nil
	case "graph-reachability":
		return map[string]string{"y":f14Reachability(x)},nil
	case "compression-run-length":
		return map[string]string{"y":f14RLE(x)},nil
	default:return nil,fmt.Errorf("unknown family %s",o.family)
	}
}

func f14UpperVowels(s string)string{out:=[]rune(s);for i,r:=range out{switch r{case'a','e','i','o','u':out[i]=r-'a'+'A'}};return string(out)}
func f14Reachability(s string)string{parts:=strings.Split(s,"|");if len(parts)!=2{return"0"};edges:=strings.Split(parts[0],";");goal:=parts[1];adj:=map[string][]string{};for _,e:=range edges{p:=strings.Split(e,">");if len(p)==2{adj[p[0]]=append(adj[p[0]],p[1])}};seen:=map[string]bool{};q:=[]string{string(goal[0])};_ = q;start:=string(goal[0]);target:=string(goal[2]);q=[]string{start};for len(q)>0{u:=q[0];q=q[1:];if u==target{return"1"};if seen[u]{continue};seen[u]=true;q=append(q,adj[u]...)};return"0"}
func f14RLE(s string)string{if s==""{return""};out:="";count:=1;for i:=1;i<=len(s);i++{if i<len(s)&&s[i]==s[i-1]{count++;continue};out+=string(s[i-1])+strconv.Itoa(count);count=1};return out}

func TestF14BroadCognitiveBattery(t *testing.T) {
	ctx:=context.Background()
	results:=map[string]bool{}
	for _,task:=range f14Tasks(){results[task.Family]=f14GeneralSolve(ctx,task)}
	passed:=0
	for _,ok:=range results{if ok{passed++}}
	t.Logf("F14_BROAD_BATTERY passed=%d/%d results=%v",passed,len(results),results)
	// This is intentionally strict: broad competence cannot be inferred from a
	// strong scalar/program subset. Every heterogeneous family must pass hidden cases.
	if passed!=len(results){
		t.Fatalf("broad cognitive battery incomplete: passed=%d/%d; no overall superhuman claim permitted",passed,len(results))
	}
}
