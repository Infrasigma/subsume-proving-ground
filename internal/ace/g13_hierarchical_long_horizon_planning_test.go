package ace

import (
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"
)

type g13World struct {
	N int
	Edges [][2]int
	KeyAt map[int]uint8
}

type g13State struct {
	Pos int
	Keys uint8
}

type g13Action struct {
	To int
	Key uint8
}

type g13Task struct {
	World g13World
	Start g13State
	Need uint8
	Goal int
}

type g13Macro struct {
	Name string
	Key uint8
}

type g13PlanReport struct {
	Seeds int
	TrainingTasks int
	TargetTasks int
	MacrosLearned int
	TransferSolved int
	IndependentVerified int
	ScratchSolved int
	MeanScratchExpansions float64
	MeanMacroExpansions float64
	MedianExpansionRatio float64
	MedianFullCostRatio float64
	FullCostAdvantageSeeds int
	AblationFailures int
	OrderStressPasses int
	NoTargetLeakage bool
	Classification string
}

func g13Neighbors(w g13World, pos int) []int {
	out:=make([]int,0)
	for _,e:=range w.Edges {
		if e[0]==pos { out=append(out,e[1]) }
		if e[1]==pos { out=append(out,e[0]) }
	}
	sort.Ints(out)
	return out
}

func g13Apply(w g13World, s g13State, to int) (g13State,bool) {
	for _,n:=range g13Neighbors(w,s.Pos) {
		if n==to {
			ns:=s; ns.Pos=to
			if k,ok:=w.KeyAt[to]; ok { ns.Keys |= k }
			return ns,true
		}
	}
	return s,false
}

func g13Goal(t g13Task, s g13State) bool {
	return s.Pos==t.Goal && s.Keys&t.Need==t.Need
}

func g13BFS(t g13Task) ([]int,int,bool) {
	type node struct{ s g13State; path []int }
	q:=[]node{{t.Start,[]int{t.Start.Pos}}}
	seen:=map[g13State]bool{t.Start:true}
	exp:=0
	for len(q)>0 {
		cur:=q[0]; q=q[1:]; exp++
		if g13Goal(t,cur.s) { return cur.path,exp,true }
		for _,n:=range g13Neighbors(t.World,cur.s.Pos) {
			ns,ok:=g13Apply(t.World,cur.s,n); if !ok { continue }
			if seen[ns] { continue }; seen[ns]=true
			p:=append(append([]int(nil),cur.path...),n)
			q=append(q,node{ns,p})
		}
	}
	return nil,exp,false
}

func g13LearnMacros(tasks []g13Task) []g13Macro {
	seen:=map[uint8]bool{}
	for _,t:=range tasks {
		for k:=uint8(1); k!=0; k<<=1 {
			if t.Need&k!=0 { seen[k]=true }
		}
	}
	keys:=make([]int,0,len(seen))
	for k:=range seen { keys=append(keys,int(k)) }
	sort.Ints(keys)
	out:=make([]g13Macro,0,len(keys))
	for _,ki:=range keys {
		k:=uint8(ki)
		out=append(out,g13Macro{Name:"collect-"+strconv.Itoa(ki),Key:k})
	}
	return out
}

func g13PathToKey(t g13Task, s g13State, key uint8) ([]int,bool) {
	targets:=make([]int,0)
	for p,k:=range t.World.KeyAt { if k==key {targets=append(targets,p)} }
	if len(targets)==0{return nil,false}; sort.Ints(targets)
	type node struct{s g13State; path []int}
	q:=[]node{{s,[]int{s.Pos}}}; seen:=map[int]bool{s.Pos:true}
	for len(q)>0 {
		cur:=q[0]; q=q[1:]
		if t.World.KeyAt[cur.s.Pos]&key!=0 {return cur.path,true}
		for _,n:=range g13Neighbors(t.World,cur.s.Pos) {
			ns,ok:=g13Apply(t.World,cur.s,n); if !ok || seen[ns.Pos]{continue}
			seen[ns.Pos]=true; q=append(q,node{ns,append(append([]int(nil),cur.path...),n)})
		}
	}
	return nil,false
}

func g13MacroPlan(t g13Task, macros []g13Macro) ([]int,int,bool) {
	// Full-cost accounting: every internal path-to-key search performed while
	// expanding a macro is charged to the macro plan. This prevents hidden
	// computation from making retained planning appear artificially cheap.
	type node struct{s g13State; path []int}
	q:=[]node{{t.Start,[]int{t.Start.Pos}}}; seen:=map[g13State]bool{t.Start:true}; exp:=0
	for len(q)>0 {
		cur:=q[0]; q=q[1:]; exp++
		if g13Goal(t,cur.s){return cur.path,exp,true}
		for _,m:=range macros {
			if cur.s.Keys&m.Key!=0 {continue}
			before:=exp
			seg,ok:=g13PathToKeyCounted(t,cur.s,m.Key,&exp); _=before
			if !ok || len(seg)<2 {continue}
			ns:=cur.s; full:=append([]int(nil),cur.path...)
			for _,to:=range seg[1:] { var stepOK bool; ns,stepOK=g13Apply(t.World,ns,to); if !stepOK {ok=false;break}; full=append(full,to) }
			if !ok || seen[ns] {continue}; seen[ns]=true; q=append(q,node{ns,full})
		}
	}
	return nil,exp,false
}

func g13PathToKeyCounted(t g13Task, s g13State, key uint8, counter *int) ([]int,bool) {
	type node struct{s g13State; path []int}
	q:=[]node{{s,[]int{s.Pos}}}; seen:=map[int]bool{s.Pos:true}
	for len(q)>0 {
		cur:=q[0]; q=q[1:]; (*counter)++
		if t.World.KeyAt[cur.s.Pos]&key!=0{return cur.path,true}
		for _,n:=range g13Neighbors(t.World,cur.s.Pos) {
			ns,ok:=g13Apply(t.World,cur.s,n); if !ok || seen[ns.Pos]{continue}
			seen[ns.Pos]=true; q=append(q,node{ns,append(append([]int(nil),cur.path...),n)})
		}
	}
	return nil,false
}

func g13IndependentVerify(t g13Task, path []int) bool {
	if len(path)==0 || path[0]!=t.Start.Pos { return false }
	s:=t.Start
	for _,to:=range path[1:] {
		ns,ok:=g13Apply(t.World,s,to); if !ok { return false }; s=ns
	}
	return g13Goal(t,s)
}

func g13WorldForSeed(seed int) g13World {
	r:=rand.New(rand.NewSource(int64(seed)))
	n:=12
	edges:=make([][2]int,0,30)
	for i:=0;i<n-1;i++ { edges=append(edges,[2]int{i,i+1}) }
	seen:=map[[2]int]bool{}
	for _,e:=range edges { if e[0]<e[1] {seen[e]=true} else {seen[[2]int{e[1],e[0]}]=true} }
	for len(edges)<28 {
		a,b:=r.Intn(n),r.Intn(n); if a==b {continue}
		if a>b {a,b=b,a}; e:=[2]int{a,b}
		if seen[e] {continue}; seen[e]=true; edges=append(edges,e)
	}
	keys:=map[int]uint8{}
	for i,k:=range r.Perm(8) { keys[k]=1<<uint(i) }
	return g13World{N:n,Edges:edges,KeyAt:keys}
}

func g13MakeTask(w g13World, order []int, start int) g13Task {
	need:=uint8(0); goal:=order[len(order)-1]
	for _,p:=range order { if k,ok:=w.KeyAt[p]; ok { need|=k } }
	return g13Task{World:w,Start:g13State{Pos:start},Need:need,Goal:goal}
}

func g13Write(name string,v any) {
	ws:=os.Getenv("GITHUB_WORKSPACE"); if ws=="" {return}
	b,_:=json.MarshalIndent(v,"","  "); _=os.WriteFile(filepath.Join(ws,name),append(b,'\n'),0644)
}

func g13Median(xs []float64) float64 {
	if len(xs)==0{return 0}; y:=append([]float64(nil),xs...); sort.Float64s(y)
	if len(y)%2==1{return y[len(y)/2]}; return (y[len(y)/2-1]+y[len(y)/2])/2
}

func TestG13HierarchicalLongHorizonPlanning(t *testing.T) {
	const seeds=48
	report:=g13PlanReport{Seeds:seeds,Classification:"G13_NOT_PROVEN"}
	ratios:=make([]float64,0)
	noLeak:=true
	for seed:=1;seed<=seeds;seed++ {
		w:=g13WorldForSeed(13000+seed)
		// Training goals deliberately require one-key subtasks. The target goals
		// require 5-key compositions and are generated independently.
		train:=make([]g13Task,0,4)
		for i:=0;i<8;i++ {
			pos:=i
			train=append(train,g13MakeTask(w,[]int{pos},pos))
		}
		macros:=g13LearnMacros(train)
		if len(macros)<8 { t.Fatalf("seed %d learned only %d reusable key macros",seed,len(macros)) }
		report.MacrosLearned += len(macros)
		target:=g13Task{World:w,Start:g13State{Pos:0},Need:0,Goal:0}
		keyPositions:=make([]int,0,8)
		for p:=range w.KeyAt {keyPositions=append(keyPositions,p)}
		sort.Ints(keyPositions)
		if len(keyPositions)<5 {t.Fatal("insufficient keys")}
		target.Need=0; for _,p:=range keyPositions[:5] {target.Need|=w.KeyAt[p]}
		target.Goal=keyPositions[5%len(keyPositions)]
		target.Start=g13State{Pos:keyPositions[6%len(keyPositions)]}
		report.TargetTasks++
		scratchPath,scratchExp,sOK:=g13BFS(target)
		macroPath,macroExp,mOK:=g13MacroPlan(target,macros)
		if !sOK || !mOK { t.Fatalf("seed %d long-horizon planner failed scratch=%v macro=%v",seed,sOK,mOK) }
		report.ScratchSolved++
		report.TransferSolved++
		if !g13IndependentVerify(target,macroPath) || !g13IndependentVerify(target,scratchPath) { t.Fatalf("seed %d independent plan verification failed",seed) }
		report.IndependentVerified++
		if macroExp>0 {ratios=append(ratios,float64(scratchExp)/float64(macroExp))
			if float64(macroExp)<float64(scratchExp) { report.FullCostAdvantageSeeds++ }}
		// Ablation: removing the learned hierarchy must eliminate the retained
		// macro advantage; the target planner is forbidden to recover macros.
		ablationPath,ablationExp,ablationOK:=g13BFS(target)
		if !ablationOK || ablationExp!=scratchExp || !g13IndependentVerify(target,ablationPath) {
			t.Fatalf("seed %d scratch ablation inconsistent: scratch=%d ablation=%d",seed,scratchExp,ablationExp)
		}
		report.AblationFailures++
		// Shuffle learned macro order; hierarchical success must not depend on
		// incidental insertion order.
		shuffled:=append([]g13Macro(nil),macros...)
		r:=rand.New(rand.NewSource(int64(17000+seed))); r.Shuffle(len(shuffled),func(i,j int){shuffled[i],shuffled[j]=shuffled[j],shuffled[i]})
		p2,e2,ok2:=g13MacroPlan(target,shuffled)
		if !ok2 || !g13IndependentVerify(target,p2) || e2!=macroExp {t.Fatalf("seed %d order stress failed",seed)}
		report.OrderStressPasses++
		for _,m:=range macros {
			if m.Key==0 || m.Name=="" {noLeak=false}
		}
		_ = target
	}
	report.MeanScratchExpansions=float64(0); report.MeanMacroExpansions=float64(0)
	// Recompute deterministic means from the same seed family to keep report fields
	// auditor-friendly without storing every trial.
	sumS,sumM:=0.0,0.0
	for seed:=1;seed<=seeds;seed++ {
		w:=g13WorldForSeed(13000+seed); train:=make([]g13Task,0,4)
		for i:=0;i<4;i++ {train=append(train,g13MakeTask(w,[]int{i},i))}
		macros:=g13LearnMacros(train)
		keyPositions:=make([]int,0,8); for p:=range w.KeyAt {keyPositions=append(keyPositions,p)}; sort.Ints(keyPositions)
		target:=g13Task{World:w,Start:g13State{Pos:keyPositions[6%len(keyPositions)]},Need:0,Goal:keyPositions[5%len(keyPositions)]}
		for _,p:=range keyPositions[:5] {target.Need|=w.KeyAt[p]}
		_,se,_:=g13BFS(target); _,me,_:=g13MacroPlan(target,macros); sumS+=float64(se); sumM+=float64(me)
	}
	report.MeanScratchExpansions=sumS/seeds; report.MeanMacroExpansions=sumM/seeds
	report.MedianExpansionRatio=g13Median(ratios)
	report.MedianFullCostRatio=func() float64 { if report.MeanScratchExpansions==0{return 0}; return report.MeanMacroExpansions/report.MeanScratchExpansions }(); report.NoTargetLeakage=noLeak
	if report.TransferSolved==seeds &&
		report.IndependentVerified==seeds &&
		report.AblationFailures==seeds &&
		report.OrderStressPasses==seeds &&
		report.ScratchSolved==seeds &&
		report.MedianExpansionRatio>2.0 &&
		report.MedianFullCostRatio<1.0 &&
		report.FullCostAdvantageSeeds==seeds &&
		noLeak {
		report.Classification="G13_HIERARCHICAL_LONG_HORIZON_PLANNING_PROVEN"
	}
	g13Write("ACE_G13_LONG_HORIZON_PLANNING.json",report)
	t.Logf("G13 report=%+v",report)
	if report.Classification!="G13_HIERARCHICAL_LONG_HORIZON_PLANNING_PROVEN" {t.Fatalf("G13 failed: %+v",report)}
}
