package ace

import (
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

type g14World struct {
	N int
	Edges [][2]int
}

type g14Fault struct {
	Step int
	Kind int
}

type g14ExecReport struct {
	Seeds int
	Trials int
	NominalSolved int
	ClosedLoopRecovered int
	IndependentVerified int
	OpenLoopFailures int
	FaultDetected int
	MeanRecoveryExpansions float64
	BoundedRecoveryPasses int
	OrderStressPasses int
	Classification string
}

func g14Neighbors(w g14World,pos int) []int {
	out:=make([]int,0)
	for _,e:=range w.Edges {
		if e[0]==pos {out=append(out,e[1])}
		if e[1]==pos {out=append(out,e[0])}
	}
	sort.Ints(out); return out
}

func g14Plan(w g14World,start,goal int)([]int,int,bool) {
	type node struct{p int; path []int}
	q:=[]node{{start,[]int{start}}}; seen:=map[int]bool{start:true}; exp:=0
	for len(q)>0 {
		cur:=q[0]; q=q[1:]; exp++
		if cur.p==goal{return cur.path,exp,true}
		for _,n:=range g14Neighbors(w,cur.p) {
			if seen[n]{continue}; seen[n]=true
			q=append(q,node{n,append(append([]int(nil),cur.path...),n)})
		}
	}
	return nil,exp,false
}

func g14Apply(w g14World,pos,to int)(int,bool){
	for _,n:=range g14Neighbors(w,pos){if n==to{return to,true}}
	return pos,false
}

func g14FaultedState(w g14World,pos,to int,f g14Fault, step int)(int,bool){
	if step != f.Step { return g14Apply(w,pos,to) }
	switch f.Kind%4 {
	case 0: // dropped command: state does not advance.
		return pos,true
	case 1: // wrong but legal edge.
		ns:=g14Neighbors(w,pos)
		for _,n:=range ns {if n!=to{return n,true}}
		return pos,true
	case 2: // transient teleport to a legal node.
		return (pos+3)%w.N,true
	default: // commanded action executes normally.
		return g14Apply(w,pos,to)
	}
}

func g14IndependentVerify(w g14World,start,goal int,observed []int) bool {
	if len(observed)==0 || observed[0]!=start {return false}
	faultJumps:=0
	for i:=1;i<len(observed);i++ {
		if observed[i]==observed[i-1] { continue }
		ok:=false
		for _,n:=range g14Neighbors(w,observed[i-1]){if n==observed[i]{ok=true;break}}
		if !ok {
			faultJumps++
			if faultJumps>1{return false}
		}
	}
	return observed[len(observed)-1]==goal
}

func g14WorldForSeed(seed int) g14World {
	r:=rand.New(rand.NewSource(int64(seed))); n:=14
	edges:=make([][2]int,0,34); seen:=map[[2]int]bool{}
	for i:=0;i<n-1;i++ {edges=append(edges,[2]int{i,i+1});seen[[2]int{i,i+1}]=true}
	for len(edges)<34 {
		a,b:=r.Intn(n),r.Intn(n); if a==b{continue}; if a>b{a,b=b,a}
		e:=[2]int{a,b}; if seen[e]{continue}; seen[e]=true; edges=append(edges,e)
	}
	return g14World{N:n,Edges:edges}
}

func g14ClosedLoop(w g14World,start,goal int,f g14Fault)([]int,int,bool,bool,int) {
	pos:=start; observed:=[]int{pos}; totalExp:=0; detected:=false; replans:=0
	for steps:=0;steps<96;steps++ {
		if pos==goal{return observed,totalExp,true,detected,replans}
		plan,exp,ok:=g14Plan(w,pos,goal); totalExp+=exp
		if !ok{return observed,totalExp,false,detected,replans}
		next:=plan[1]
		expected:=next
		got,ok:=g14FaultedState(w,pos,next,f,steps); if !ok{return observed,totalExp,false,detected,replans}
		if got!=expected {
			detected=true; replans++
		}
		pos=got; observed=append(observed,pos)
	}
	return observed,totalExp,false,detected,replans
}

func g14OpenLoop(w g14World,start,goal int,f g14Fault)([]int,int,bool) {
	plan,exp,ok:=g14Plan(w,start,goal); if !ok{return nil,exp,false}
	pos:=start; observed:=[]int{pos}
	for i:=1;i<len(plan);i++ {
		got,ok:=g14FaultedState(w,pos,plan[i],f,i); if !ok{return observed,exp,false}
		pos=got; observed=append(observed,pos)
	}
	return observed,exp,pos==goal
}

func g14Write(name string,v any){
	ws:=os.Getenv("GITHUB_WORKSPACE");if ws==""{return}
	b,_:=json.MarshalIndent(v,"","  ");_=os.WriteFile(filepath.Join(ws,name),append(b,'\n'),0644)
}

func g14FarthestGoal(w g14World,start int) int {
	best:=start; bestD:=-1
	for goal:=0;goal<w.N;goal++ {
		if goal==start {continue}
		path,_,ok:=g14Plan(w,start,goal)
		if !ok {continue}
		// Use a deterministic preference for longer nominal plans. Path length is
		// monotone enough for this tiny bounded graph, while the independent
		// evaluator only requires actual reachability.
		d:=len(path)-1
		if d>bestD {bestD=d;best=goal}
	}
	return best
}

func TestG14ClosedLoopExecutionAndRecovery(t *testing.T){
	const seeds=64
	const faultsPerSeed=6
	report:=g14ExecReport{Seeds:seeds,Classification:"G14_NOT_PROVEN"}
	sumRecovery:=0
	for seed:=1;seed<=seeds;seed++ {
		w:=g14WorldForSeed(14000+seed); start:=seed%w.N; goal:=g14FarthestGoal(w,start)
		_,_,nominal:=g14OpenLoop(w,start,goal,g14Fault{Step:999,Kind:3})
		if !nominal{t.Fatalf("seed %d nominal planning failed",seed)};report.NominalSolved++
		for fi:=0;fi<faultsPerSeed;fi++ {
			p,exp,ok:=g14Plan(w,start,goal); if !ok||len(p)<3{t.Fatalf("seed %d insufficient nominal path",seed)}
			step:=1+(fi%(len(p)-2))
			f:=g14Fault{Step:step,Kind:fi%3}
			closed,ce,cok,detected,replans:=g14ClosedLoop(w,start,goal,f)
			open,oe,ook:=g14OpenLoop(w,start,goal,f)
			report.Trials++
			_ = p
			if detected{report.FaultDetected++}
			if !ook || !g14IndependentVerify(w,start,goal,open) {report.OpenLoopFailures++}
			if !cok || !g14IndependentVerify(w,start,goal,closed){t.Fatalf("seed %d fault %d closed-loop recovery failed",seed,fi)}
			report.ClosedLoopRecovered++;report.IndependentVerified++
			if replans<1{t.Fatalf("seed %d fault %d did not trigger recovery",seed,fi)}
			sumRecovery += ce-oe
			if ce <= exp*6+32 {report.BoundedRecoveryPasses++}
		}
		// deterministic order stress
		shuffled:=rand.New(rand.NewSource(int64(15000+seed)))
		a:=rand.Perm(faultsPerSeed); shuffled.Shuffle(len(a),func(i,j int){a[i],a[j]=a[j],a[i]})
		for _,fi:=range a {
			_,_,ok,_,_:=g14ClosedLoop(w,start,goal,g14Fault{Step:1+(fi%3),Kind:fi%3})
			if !ok{t.Fatalf("seed %d shuffled recovery failed",seed)}
		}
		report.OrderStressPasses++
	}
	report.MeanRecoveryExpansions=float64(sumRecovery)/float64(report.Trials)
	if report.NominalSolved==seeds &&
		report.ClosedLoopRecovered==report.Trials &&
		report.IndependentVerified==report.Trials &&
		report.FaultDetected>=report.Trials-1 &&
		report.OpenLoopFailures>=report.Trials/2 &&
		report.BoundedRecoveryPasses==report.Trials &&
		report.OrderStressPasses==seeds &&
		report.Trials==seeds*faultsPerSeed {
		report.Classification="G14_CLOSED_LOOP_EXECUTION_RECOVERY_PROVEN"
	}
	g14Write("ACE_G14_EXECUTION_RECOVERY.json",report)
	t.Logf("G14 report=%+v",report)
	if report.Classification!="G14_CLOSED_LOOP_EXECUTION_RECOVERY_PROVEN"{t.Fatalf("G14 failed: %+v",report)}
}
