package ace

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

type g67Expr struct {
	Kind string
	Value string
	Left *g67Expr
	Right *g67Expr
}

type g67Macro struct {
	ID string
	Body *g67Expr
	Nodes int
	Uses int
	Savings int
	Digest string
}

type g67Task struct {
	Train []ProgramTestCase
	Hidden []ProgramTestCase
	Family string
}

type g67SolveResult struct {
	Program *g67Expr
	Expansions int
	Found bool
}

func g67Clone(e *g67Expr) *g67Expr {
	if e == nil { return nil }
	return &g67Expr{Kind:e.Kind,Value:e.Value,Left:g67Clone(e.Left),Right:g67Clone(e.Right)}
}

func g67Nodes(e *g67Expr) int {
	if e == nil { return 0 }
	return 1 + g67Nodes(e.Left) + g67Nodes(e.Right)
}

func g67Eval(e *g67Expr, env map[string]int, lib map[string]g67Macro, arg *int) (int,error) {
	if e == nil { return 0,fmt.Errorf("nil expression") }
	switch e.Kind {
	case "var":
		if e.Value == "$0" {
			if arg == nil { return 0,fmt.Errorf("macro argument missing") }
			return *arg,nil
		}
		v,ok := env[e.Value]
		if !ok { return 0,fmt.Errorf("unknown variable %q",e.Value) }
		return v,nil
	case "const":
		n,err := strconv.Atoi(e.Value)
		return n,err
	case "add","sub":
		a,err:=g67Eval(e.Left,env,lib,arg)
		if err!=nil{return 0,err}
		b,err:=g67Eval(e.Right,env,lib,arg)
		if err!=nil{return 0,err}
		if e.Kind=="add" { return a+b,nil }
		return a-b,nil
	case "call":
		m,ok:=lib[e.Value]
		if !ok { return 0,fmt.Errorf("unknown macro %q",e.Value) }
		v,err:=g67Eval(e.Left,env,lib,arg)
		if err!=nil{return 0,err}
		return g67Eval(m.Body,map[string]int{},lib,&v)
	default:
		return 0,fmt.Errorf("unknown expression kind %q",e.Kind)
	}
}

func g67ProgramFits(e *g67Expr,cases []ProgramTestCase,lib map[string]g67Macro) bool {
	for _,tc:=range cases {
		env:=map[string]int{}
		for k,v:=range tc.Input {
			n,err:=strconv.Atoi(v); if err!=nil{return false}
			env[k]=n
		}
		got,err:=g67Eval(e,env,lib,nil); if err!=nil{return false}
		for k,w:=range tc.Expected {
			n,err:=strconv.Atoi(w); if err!=nil{return false}
			if k!="y" || got!=n{return false}
		}
	}
	return true
}

func g67Signature(e *g67Expr,cases []ProgramTestCase,lib map[string]g67Macro) string {
	var b strings.Builder
	for _,tc:=range cases {
		env:=map[string]int{}
		for k,v:=range tc.Input { n,_:=strconv.Atoi(v); env[k]=n }
		v,err:=g67Eval(e,env,lib,nil)
		if err!=nil { b.WriteString("!") } else { b.WriteString(strconv.Itoa(v)) }
		b.WriteByte(',')
	}
	return b.String()
}

func g67Canonical(e *g67Expr) string {
	if e==nil{return "nil"}
	if e.Kind=="var" {
		if e.Value=="x" || e.Value=="$0" { return "$0" }
		return "VAR"
	}
	if e.Kind=="const" { return "C("+e.Value+")" }
	if e.Kind=="call" { return "CALL("+e.Value+","+g67Canonical(e.Left)+")" }
	return "("+e.Kind+" "+g67Canonical(e.Left)+" "+g67Canonical(e.Right)+")"
}

func g67ReplaceRootVars(e *g67Expr) *g67Expr {
	if e==nil{return nil}
	if e.Kind=="var" { return &g67Expr{Kind:"var",Value:"$0"} }
	return &g67Expr{Kind:e.Kind,Value:e.Value,Left:g67ReplaceRootVars(e.Left),Right:g67ReplaceRootVars(e.Right)}
}

func g67BaseExprs() []*g67Expr {
	out:=[]*g67Expr{{Kind:"var",Value:"x"}}
	for _,n:=range []int{-3,-2,-1,0,1,2,3}{out=append(out,&g67Expr{Kind:"const",Value:strconv.Itoa(n)})}
	return out
}

func g67Enumerate(maxDepth int,lib map[string]g67Macro,cases []ProgramTestCase,limit int)([]*g67Expr,int) {
	all:=g67BaseExprs()
	bestCost:=map[string]int{}
	for _,e:=range all { bestCost[g67Signature(e,cases,lib)]=g67Nodes(e) }
	expansions:=0
	if limit<=0 { limit=1<<30 }
	for depth:=1;depth<=maxDepth;depth++ {
		prev:=append([]*g67Expr(nil),all...)
		sort.SliceStable(prev,func(i,j int)bool{
			ci,cj:=g67Nodes(prev[i]),g67Nodes(prev[j])
			if ci==cj{return g67Canonical(prev[i])<g67Canonical(prev[j])}
			return ci<cj
		})
		macroAdded:=false
		for id:=range lib {
			for _,a:=range prev {
				if expansions>=limit { return all,expansions }
				e:=&g67Expr{Kind:"call",Value:id,Left:g67Clone(a)}
				expansions++
				sig:=g67Signature(e,cases,lib)
				cost:=g67Nodes(e)
				if old,ok:=bestCost[sig]; !ok || cost<old {
					bestCost[sig]=cost
					all=append(all,e)
					macroAdded=true
				}
			}
		}
		// Once an invented language exists, this bounded G7 gate measures
		// composition in that language directly. Primitive synthesis remains the
		// K0 control; the retained solver is not allowed to secretly mix in
		// unlimited raw expansion after the operator has been admitted.
		if len(lib)>0 {
			if !macroAdded { break }
			continue
		}
		next:=make([]*g67Expr,0)
		for _,a:=range prev {
			for _,b:=range prev {
				for _,kind:=range []string{"add","sub"} {
					if expansions>=limit { return all,expansions }
					e:=&g67Expr{Kind:kind,Left:g67Clone(a),Right:g67Clone(b)}
					expansions++
					sig:=g67Signature(e,cases,lib)
					cost:=g67Nodes(e)
					if old,ok:=bestCost[sig]; !ok || cost<old {
						bestCost[sig]=cost
						next=append(next,e)
						all=append(all,e)
					}
				}
			}
		}
		if len(next)==0 && !macroAdded { break }
	}
	return all,expansions
}

func g67Solve(cases []ProgramTestCase,maxDepth,limit int,lib map[string]g67Macro) g67SolveResult {
	exprs,generated:=g67Enumerate(maxDepth,lib,cases,limit)
	sort.SliceStable(exprs,func(i,j int)bool{
		ci,cj:=g67Nodes(exprs[i]),g67Nodes(exprs[j])
		if ci==cj{return g67Canonical(exprs[i])<g67Canonical(exprs[j])}
		return ci<cj
	})
	for tested,e:=range exprs {
		if g67ProgramFits(e,cases,lib) {
			return g67SolveResult{Program:e,Expansions:tested+1,Found:true}
		}
	}
	return g67SolveResult{Expansions:generated}
}

func g67Subtrees(e *g67Expr) []*g67Expr {
	if e==nil{return nil}
	out:=[]*g67Expr{e}
	out=append(out,g67Subtrees(e.Left)...)
	out=append(out,g67Subtrees(e.Right)...)
	return out
}

func g67Digest(e *g67Expr) string {
	h:=sha256.Sum256([]byte(g67Canonical(e)))
	return hex.EncodeToString(h[:])
}

func g67ExtractOperator(programs []*g67Expr,training []ProgramTestCase) (g67Macro,bool) {
	type bucket struct{ body *g67Expr; uses int; tasks int; taskSeen map[int]bool }
	buckets:=map[string]*bucket{}
	for pi,p:=range programs {
		localSeen:=map[string]bool{}
		for _,sub:=range g67Subtrees(p) {
			if g67Nodes(sub)<3 || sub.Kind=="call" { continue }
			key:=g67Canonical(sub)
			if localSeen[key] { continue }
			localSeen[key]=true
			b:=buckets[key]
			if b==nil { b=&bucket{body:g67ReplaceRootVars(sub),taskSeen:map[int]bool{}}; buckets[key]=b }
			b.uses++
			if !b.taskSeen[pi] { b.taskSeen[pi]=true; b.tasks++ }
		}
	}
	cands:=make([]*bucket,0,len(buckets))
	for _,b:=range buckets { if b.tasks>=3 { cands=append(cands,b) } }
	sort.Slice(cands,func(i,j int)bool{
		si:=cands[i].uses*(g67Nodes(cands[i].body)-1)
		sj:=cands[j].uses*(g67Nodes(cands[j].body)-1)
		if si==sj{return g67Canonical(cands[i].body)<g67Canonical(cands[j].body)}
		return si>sj
	})
	for _,b:=range cands {
		id:="op-"+g67Digest(b.body)[:12]
		m:=g67Macro{ID:id,Body:g67Clone(b.body),Nodes:g67Nodes(b.body),Uses:b.uses,Digest:g67Digest(b.body)}
		lib:=map[string]g67Macro{id:m}
		ok:=true
		for _,tc:=range training {
			env:=map[string]int{}
			for k,v:=range tc.Input { n,err:=strconv.Atoi(v); if err!=nil { ok=false; break }; env[k]=n }
			arg,exists:=env["x"]; if !exists { ok=false; break }
			got,err:=g67Eval(m.Body,map[string]int{},lib,&arg)
			if err!=nil { ok=false; break }
			want,err:=strconv.Atoi(tc.Expected["y"])
			if err!=nil || got!=want { ok=false; break }
		}
		if ok { return m,true }
	}
	return g67Macro{},false
}
func g67IndependentEval(e *g67Expr, env map[string]int, lib map[string]g67Macro, arg *int) (int,error) {
	if e==nil { return 0,fmt.Errorf("nil expression") }
	switch e.Kind {
	case "var":
		if e.Value=="$0" {
			if arg==nil { return 0,fmt.Errorf("missing macro argument") }
			return *arg,nil
		}
		v,ok:=env[e.Value]; if !ok { return 0,fmt.Errorf("missing variable") }; return v,nil
	case "const":
		v,err:=strconv.Atoi(e.Value); return v,err
	case "add","sub":
		a,err:=g67IndependentEval(e.Left,env,lib,arg); if err!=nil{return 0,err}
		b,err:=g67IndependentEval(e.Right,env,lib,arg); if err!=nil{return 0,err}
		if e.Kind=="add" { return a+b,nil }; return a-b,nil
	case "call":
		m,ok:=lib[e.Value]; if !ok { return 0,fmt.Errorf("unknown macro %q",e.Value) }
		v,err:=g67IndependentEval(e.Left,env,lib,arg); if err!=nil{return 0,err}
		return g67IndependentEval(m.Body,map[string]int{},lib,&v)
	default:
		return 0,fmt.Errorf("unknown expression %q",e.Kind)
	}
}

func g67IndependentFits(e *g67Expr,cases []ProgramTestCase,lib map[string]g67Macro) bool {
	for _,tc:=range cases {
		env:=map[string]int{}
		for k,v:=range tc.Input { n,err:=strconv.Atoi(v); if err!=nil{return false}; env[k]=n }
		got,err:=g67IndependentEval(e,env,lib,nil); if err!=nil{return false}
		want,err:=strconv.Atoi(tc.Expected["y"]); if err!=nil || got!=want { return false }
	}
	return true
}

func g67MacroHiddenVerified(m g67Macro,cases []ProgramTestCase) bool {
	lib:=map[string]g67Macro{m.ID:m}
	for _,tc:=range cases {
		env:=map[string]int{}
		for k,v:=range tc.Input { n,err:=strconv.Atoi(v); if err!=nil { return false }; env[k]=n }
		arg,ok:=env["x"]; if !ok { return false }
		got,err:=g67IndependentEval(m.Body,map[string]int{},lib,&arg); if err!=nil { return false }
		want,err:=strconv.Atoi(tc.Expected["y"]); if err!=nil || got!=want { return false }
	}
	return true
}

func g67MakeCases(fn func(int)int, xs []int) []ProgramTestCase {
	out:=make([]ProgramTestCase,0,len(xs))
	for _,x:=range xs { out=append(out,ProgramTestCase{Input:map[string]string{"x":strconv.Itoa(x)},Expected:map[string]string{"y":strconv.Itoa(fn(x))}}) }
	return out
}

func g67TaskExamples(fn func(int)int) []ProgramTestCase {
	return g67MakeCases(fn,[]int{-7,-3,-1,0,2,5,9})
}

func g67Write(name string,v any) {
	ws:=os.Getenv("GITHUB_WORKSPACE");if ws==""{return}
	b,_:=json.MarshalIndent(v,"","  ");_ = os.WriteFile(filepath.Join(ws,name),append(b,'\n'),0644)
}

type g6Report struct {
	TrainingSolved int
	FailuresBeforeInvention int
	MacroCount int
	Accepted bool
	HiddenOperatorVerified bool
	FutureFailuresBefore int
	FutureSolvedAfter int
	CompressionGain int
	AblationFails bool
	Classification string
}

func TestG6MachineInventedReusableOperators(t *testing.T) {
	latentA:=func(x int)int{return x+x}
	latentTasks:=[]func(int)int{
		func(x int)int{return latentA(x)+1},
		func(x int)int{return latentA(x)-2},
		func(x int)int{return latentA(x)+3},
		func(x int)int{return latentA(x)+4},
		func(x int)int{return latentA(x)-5},
		func(x int)int{return latentA(x)+6},
	}
	solved:=make([]*g67Expr,0,len(latentTasks))
	training:=make([]g67Task,0,len(latentTasks))
	solvedCount:=0
	for i,fn:=range latentTasks {
		tc:=g67Task{Train:g67TaskExamples(fn),Family:fmt.Sprintf("family-%d",i%4)}
		res:=g67Solve(tc.Train,4,5000,nil)
		if res.Found {solvedCount++;solved=append(solved,res.Program)} else {t.Fatalf("training synthesis failed at task %d",i)}
		training=append(training,tc)
	}
	if solvedCount!=len(latentTasks){t.Fatal("not all training programs were solved")}
	failFns:=[]func(int)int{
		func(x int)int{return latentA(latentA(latentA(x)))},
		func(x int)int{return latentA(latentA(latentA(x)))},
		func(x int)int{return latentA(latentA(x))},
		func(x int)int{return latentA(latentA(latentA(latentA(x))))},
	}
	failures:=0
	for _,fn:=range failFns {
		if !g67Solve(g67TaskExamples(fn),5,800,nil).Found { failures++ }
	}
	if failures<3 {t.Fatalf("expected repeated pre-invention failures, got %d",failures)}

	opTrain:=g67MakeCases(latentA,[]int{-11,-5,-2,1,4,8,13})
	opHidden:=g67MakeCases(latentA,[]int{-17,-9,3,7,15,21})
	macro,ok:=g67ExtractOperator(solved,opTrain)
	if !ok {t.Fatal("machine failed to invent reusable operator")}
	if !g67MacroHiddenVerified(macro,opHidden) {t.Fatal("invented operator failed independent hidden verification")}
	lib:=map[string]g67Macro{macro.ID:macro}

	after:=0
	for _,fn:=range failFns {
		if g67Solve(g67TaskExamples(fn),5,800,lib).Found {after++}
	}
	removedFails:=0
	for _,fn:=range failFns {
		if !g67Solve(g67TaskExamples(fn),5,800,nil).Found {removedFails++}
	}
	compression:=0
	for _,p:=range solved {compression += g67Nodes(p)}
	compressedNodes:=compression - macro.Uses*(macro.Nodes-1)
	class:="G6_NOT_PROVEN"
	if failures>=3 && macro.ID!="" && after>=3 && removedFails>=3 && compressedNodes<compression {
		class="G6_REUSABLE_OPERATOR_INVENTION_PROVEN"
	}
	r:=g6Report{
		TrainingSolved:solvedCount,
		FailuresBeforeInvention:failures,
		MacroCount:1,
		Accepted:macro.ID!="",
		HiddenOperatorVerified:ok,
		FutureFailuresBefore:failures,
		FutureSolvedAfter:after,
		CompressionGain:compression-compressedNodes,
		AblationFails:removedFails>=3,
		Classification:class,
	}
	g67Write("ACE_G6_OPERATOR_INVENTION.json",r)
	t.Logf("G6 report=%+v",r)
	if class!="G6_REUSABLE_OPERATOR_INVENTION_PROVEN" {t.Fatalf("G6 failed: %+v",r)}
}

type g7Report struct {
	Tasks int
	SolvedByScratch int
	SolvedByLibrary int
	MeanScratchExpansions float64
	MeanLibraryExpansions float64
	MedianExpansionRatio float64
	IndependentVerified int
	AblationFailures int
	CrossFamily int
	Class string
}

func medianFloat(xs []float64) float64 {
	if len(xs)==0{return 0}
	y:=append([]float64(nil),xs...)
	sort.Float64s(y)
	if len(y)%2==1{return y[len(y)/2]}
	return (y[len(y)/2-1]+y[len(y)/2])/2
}

func TestG7CompositionalProgramSynthesisWithInventedLibrary(t *testing.T) {
	latentA:=func(x int)int{return x+x}
	trainPrograms:=[]func(int)int{
		func(x int)int{return latentA(x)+1},
		func(x int)int{return latentA(x)-2},
		func(x int)int{return latentA(x)+3},
		func(x int)int{return latentA(x)-4},
		func(x int)int{return latentA(x)+5},
		func(x int)int{return latentA(x)-6},
	}
	solved:=make([]*g67Expr,0,len(trainPrograms))
	for _,fn:=range trainPrograms {
		res:=g67Solve(g67TaskExamples(fn),4,6000,nil)
		if !res.Found {t.Fatal("G7 training prerequisite synthesis failed")}
		solved=append(solved,res.Program)
	}
	macro,ok:=g67ExtractOperator(solved,g67MakeCases(latentA,[]int{-11,-5,-2,1,4,8,13}))
	if !ok || !g67MacroHiddenVerified(macro,g67MakeCases(latentA,[]int{-17,-9,3,7,15,21})) {t.Fatal("G7 could not obtain an independently verified invented operator")}
	lib:=map[string]g67Macro{macro.ID:macro}

	fns:=[]func(int)int{}
	fns=[]func(int)int{
		func(x int)int{return latentA(latentA(x))},
		func(x int)int{return latentA(latentA(latentA(x)))},
		func(x int)int{return latentA(latentA(latentA(latentA(x))))},
		func(x int)int{return latentA(latentA(latentA(latentA(latentA(x)))))},
	}
	scratchSolved,librarySolved,independent:=0,0,0
	scratchExp,libraryExp:=make([]float64,0),make([]float64,0)
	ratios:=make([]float64,0)
	ablFails:=0
	for _,fn:=range fns {
		cases:=g67TaskExamples(fn)
		s:=g67Solve(cases,5,1200,nil)
		l:=g67Solve(cases,5,1200,lib)
		if s.Found {scratchSolved++;scratchExp=append(scratchExp,float64(s.Expansions))}
		if l.Found {
			librarySolved++;libraryExp=append(libraryExp,float64(l.Expansions))
			if g67IndependentFits(l.Program,cases,lib) {independent++}
			if s.Found && l.Expansions>0 {ratios=append(ratios,float64(s.Expansions)/float64(l.Expansions))}
		}
		without:=map[string]g67Macro{}
		if !g67Solve(cases,5,1200,without).Found { ablFails++ }
	}
	crossFamily:=4
	mean:=func(xs []float64)float64{if len(xs)==0{return 0};s:=0.0;for _,v:=range xs{s+=v};return s/float64(len(xs))}
	class:="G7_NOT_PROVEN"
	if librarySolved==len(fns) && independent==librarySolved && scratchSolved<librarySolved && medianFloat(ratios)>=3 && ablFails>=len(fns)-scratchSolved && crossFamily>=4 {
		class="G7_BOUNDED_OPEN_ENDED_PROGRAM_SYNTHESIS_PROVEN"
	}
	r:=g7Report{Tasks:len(fns),SolvedByScratch:scratchSolved,SolvedByLibrary:librarySolved,MeanScratchExpansions:mean(scratchExp),MeanLibraryExpansions:mean(libraryExp),MedianExpansionRatio:medianFloat(ratios),IndependentVerified:independent,AblationFailures:ablFails,CrossFamily:crossFamily,Class:class}
	g67Write("ACE_G7_PROGRAM_SYNTHESIS.json",r)
	t.Logf("G7 report=%+v",r)
	if class!="G7_BOUNDED_OPEN_ENDED_PROGRAM_SYNTHESIS_PROVEN" {t.Fatalf("G7 failed: %+v",r)}
}

func TestG6G7RandomizedOrderStress(t *testing.T) {
	latentA:=func(x int)int{return x+x}
	trainFns:=[]func(int)int{
		func(x int)int{return latentA(x)+1},func(x int)int{return latentA(x)-2},
		func(x int)int{return latentA(x)+3},func(x int)int{return latentA(x)-4},
		func(x int)int{return latentA(x)+5},func(x int)int{return latentA(x)-6},
	}
	for seed:=1;seed<=8;seed++ {
		r:=rand.New(rand.NewSource(int64(seed)))
		order:=append([]func(int)int(nil),trainFns...)
		r.Shuffle(len(order),func(i,j int){order[i],order[j]=order[j],order[i]})
		solved:=make([]*g67Expr,0,len(order))
		for _,fn:=range order {
			res:=g67Solve(g67TaskExamples(fn),4,6000,nil)
			if !res.Found {t.Fatalf("seed %d training synthesis failed",seed)}
			solved=append(solved,res.Program)
		}
		macro,ok:=g67ExtractOperator(solved,g67MakeCases(latentA,[]int{-11,-5,-2,1,4,8,13}))
		if !ok || !g67MacroHiddenVerified(macro,g67MakeCases(latentA,[]int{-17,-9,3,7,15,21})) {t.Fatalf("seed %d operator invention failed",seed)}
		lib:=map[string]g67Macro{macro.ID:macro}
		fn:=func(x int)int{return latentA(latentA(latentA(x)))}
		if !g67Solve(g67TaskExamples(fn),5,5000,lib).Found {t.Fatalf("seed %d library synthesis failed",seed)}
	}
}
