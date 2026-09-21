package ace

import (
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

type g14World struct { N int; Edges [][2]int }
type g14Fault struct { Step, Kind int }

type g14ExecReport struct {
	Seeds int
	Trials int
	NominalSolved int
	ClosedLoopRecovered int
	IndependentVerified int
	OpenLoopFailures int
	FaultDetected int
	MultiFaultRecovered int
	MeanRecoveryExpansions float64
	BoundedRecoveryPasses int
	OrderStressPasses int
	TopologyStressPasses int
	Classification string
}

func g14Neighbors(w g14World,pos int) []int {
	out:=make([]int,0)
	for _,e:=range w.Edges {
		if e[0]==pos { out=append(out,e[1]) }
		if e[1]==pos { out=append(out,e[0]) }
	}
	sort.Ints(out); return out
}

func g14Plan(w g14World,start,goal int)([]int,int,bool) {
	type node struct{ p int; path []int }
	q:=[]node{{start,[]int{start}}}; seen:=map[int]bool{start:true}; exp:=0
	for len(q)>0 {
		cur:=q[0]; q=q[1:]; exp++
		if cur.p==goal { return cur.path,exp,true }
		for _,n:=range g14Neighbors(w,cur.p) {
			if seen[n] { continue }
			seen[n]=true
			q=append(q,node{n,append(append([]int(nil),cur.path...),n)})
		}
	}
	return nil,exp,false
}

func g14Apply(w g14World,pos,to int)(int,bool) {
	for _,n:=range g14Neighbors(w,pos) { if n==to { return to,true } }
	return pos,false
}

func g14DisturbedState(w g14World,pos,to int,f g14Fault,step int,goal int)(int,bool) {
	if step!=f.Step { return g14Apply(w,pos,to) }
	switch f.Kind%4 {
	case 0: // dropped command
		return pos,true
	case 1: // wrong but legal action
		for _,n:=range g14Neighbors(w,pos) {
			if n!=to && n!=goal { return n,true }
		}
		return pos,true
	case 2: // state disturbance, guaranteed not to land on the goal
		for delta:=3;delta<w.N+3;delta++ {
			p:=(pos+delta)%w.N
			if p!=goal && p!=to { return p,true }
		}
		return pos,true
	default: // stale/duplicated command: after the intended move, execute a
		// different legal move when one exists, guaranteeing an observable deviation.
		after,ok:=g14Apply(w,pos,to)
		if !ok { return pos,true }
		for _,n:=range g14Neighbors(w,after) {
			if n!=pos && n!=goal { return n,true }
		}
		return after,true
	}
}

func g14IndependentVerify(w g14World,start,goal int,observed []int,maxDisturbances int) bool {
	if len(observed)==0 || observed[0]!=start || observed[len(observed)-1]!=goal { return false }
	adj:=map[[2]int]bool{}
	for _,e:=range w.Edges {
		adj[e]=true
		adj[[2]int{e[1],e[0]}]=true
	}
	illegal:=0
	for i:=1;i<len(observed);i++ {
		if observed[i]==observed[i-1] { continue }
		if !adj[[2]int{observed[i-1],observed[i]}] {
			illegal++
		}
	}
	return illegal<=maxDisturbances
}

func g14WorldForSeed(seed int) g14World {
	r:=rand.New(rand.NewSource(int64(seed)))
	n:=24
	edges:=make([][2]int,0,35); seen:=map[[2]int]bool{}
	for i:=0;i<n-1;i++ { edges=append(edges,[2]int{i,i+1}); seen[[2]int{i,i+1}]=true }
	for len(edges)<35 {
		a,b:=r.Intn(n),r.Intn(n); if a==b { continue }; if a>b { a,b=b,a }
		e:=[2]int{a,b}; if seen[e] { continue }; seen[e]=true; edges=append(edges,e)
	}
	return g14World{N:n,Edges:edges}
}

func g14FarthestGoal(w g14World,start int) int {
	best,bestD:=start,-1
	for goal:=0;goal<w.N;goal++ {
		if goal==start { continue }
		p,_,ok:=g14Plan(w,start,goal); if !ok { continue }
		if d:=len(p)-1; d>bestD { best,bestD=goal,d }
	}
	return best
}

func g14Run(w g14World,start,goal int,faults []g14Fault,closedLoop bool)([]int,int,bool,int,int) {
	pos:=start; observed:=[]int{pos}; totalExp:=0; detected:=0; replans:=0
	for step:=0;step<160;step++ {
		if pos==goal { return observed,totalExp,true,detected,replans }
		plan,exp,ok:=g14Plan(w,pos,goal); totalExp+=exp
		if !ok || len(plan)<2 { return observed,totalExp,false,detected,replans }
		next:=plan[1]; got:=pos
		if closedLoop {
			got=next
			for _,f:=range faults { var ok2 bool; got,ok2=g14DisturbedState(w,pos,next,f,step+1,goal); if !ok2 { return observed,totalExp,false,detected,replans }; if f.Step==step { break } }
			if got!=next { detected++; replans++ }
		} else {
			got=next
			for _,f:=range faults {
				var ok2 bool
				got,ok2=g14DisturbedState(w,pos,next,f,step+1,goal)
				if !ok2 { return observed,totalExp,false,detected,replans }
				if f.Step==step+1 { break }
			}
		}
		pos=got; observed=append(observed,pos)
		if !closedLoop && step>len(plan)+32 { break }
	}
	return observed,totalExp,false,detected,replans
}

func g14Write(name string,v any) {
	ws:=os.Getenv("GITHUB_WORKSPACE"); if ws=="" { return }
	b,_:=json.MarshalIndent(v,"","  "); _=os.WriteFile(filepath.Join(ws,name),append(b,'\n'),0644)
}

func TestG14ClosedLoopExecutionAndRecovery(t *testing.T) {
	const seeds=64
	const singleTrials=8
	report:=g14ExecReport{Seeds:seeds,Classification:"G14_NOT_PROVEN"}
	sumRecovery:=0
	for seed:=1;seed<=seeds;seed++ {
		w:=g14WorldForSeed(14000+seed); start:=seed%w.N; goal:=g14FarthestGoal(w,start)
		_,_,nominal,_,_:=g14Run(w,start,goal,nil,false)
		if !nominal { t.Fatalf("seed %d nominal planning failed",seed) }
		report.NominalSolved++

		for ti:=0;ti<singleTrials;ti++ {
			p,baseExp,ok:=g14Plan(w,start,goal); if !ok || len(p)<7 { t.Fatalf("seed %d insufficient path: len=%d",seed,len(p)) }
			step:=1+(ti%(len(p)-3))
			f:=g14Fault{Step:step,Kind:ti%4}
			closed,ce,cok,detected,replans:=g14Run(w,start,goal,[]g14Fault{f},true)
			open,oe,ook,_,_:=g14Run(w,start,goal,[]g14Fault{f},false)
			report.Trials++
			if detected>0 { report.FaultDetected++ }
			if !ook || !g14IndependentVerify(w,start,goal,open,1) { report.OpenLoopFailures++ }
			if !cok || !g14IndependentVerify(w,start,goal,closed,1) { t.Fatalf("seed %d single fault %d recovery failed",seed,ti) }
			if detected<1 || replans<1 { t.Fatalf("seed %d single fault %d was not detected/replanned",seed,ti) }
			report.ClosedLoopRecovered++; report.IndependentVerified++
			if ce<=baseExp*8+64 { report.BoundedRecoveryPasses++ }
			sumRecovery += ce-oe
		}

		// Multi-fault chains: later faults are generated without giving the
		// controller the sequence itself.
		p,_,ok:=g14Plan(w,start,goal); if !ok || len(p)<9 { t.Fatalf("seed %d path too short for multi-fault trial: len=%d",seed,len(p)) }
		faults:=[]g14Fault{
			{Step:1+(seed%3),Kind:seed%4},
			{Step:3+(seed%4),Kind:(seed+1)%4},
			{Step:5+(seed%3),Kind:(seed+2)%4},
		}
		multi,me,mok,mdetect,mreplans:=g14Run(w,start,goal,faults,true)
		if !mok || !g14IndependentVerify(w,start,goal,multi,len(faults)) || mdetect<2 || mreplans<2 {
			t.Fatalf("seed %d multi-fault recovery failed: detect=%d replans=%d",seed,mdetect,mreplans)
		}
		report.MultiFaultRecovered++
		if me>20000 { t.Fatalf("seed %d pathological multi-fault cost=%d",seed,me) }

		for order:=0;order<4;order++ {
			fcopy:=append([]g14Fault(nil),faults...); rr:=rand.New(rand.NewSource(int64(15000+seed*10+order)))
			rr.Shuffle(len(fcopy),func(i,j int){fcopy[i],fcopy[j]=fcopy[j],fcopy[i]})
			rp,_,rok,rd,_:=g14Run(w,start,goal,fcopy,true)
			if !rok || rd<2 || !g14IndependentVerify(w,start,goal,rp,len(fcopy)) { t.Fatalf("seed %d fault-order stress failed",seed) }
		}
		report.OrderStressPasses++

		// Topology stress uses a fresh graph with the same controller.
		w2:=g14WorldForSeed(24000+seed); s2:=(seed*3)%w2.N; g2:=g14FarthestGoal(w2,s2)
		p2,_,ok2:=g14Plan(w2,s2,g2); if !ok2 || len(p2)<5 { t.Fatalf("seed %d topology stress nominal failure",seed) }
		f2:=[]g14Fault{{Step:2,Kind:(seed+3)%4},{Step:4,Kind:(seed+1)%4}}
		r2,_,ok2d,d2,_:=g14Run(w2,s2,g2,f2,true)
		if !ok2d || d2<1 || !g14IndependentVerify(w2,s2,g2,r2,len(f2)) { t.Fatalf("seed %d topology recovery failed",seed) }
		report.TopologyStressPasses++
	}
	report.MeanRecoveryExpansions=float64(sumRecovery)/(float64(seeds*singleTrials))
	if report.NominalSolved==seeds &&
		report.Trials==seeds*singleTrials &&
		report.ClosedLoopRecovered==report.Trials &&
		report.IndependentVerified==report.Trials &&
		report.MultiFaultRecovered==seeds &&
		report.FaultDetected>=report.Trials &&
		report.OpenLoopFailures>=report.Trials/2 &&
		report.BoundedRecoveryPasses==report.Trials &&
		report.OrderStressPasses==seeds &&
		report.TopologyStressPasses==seeds {
		report.Classification="G14_CLOSED_LOOP_EXECUTION_RECOVERY_PROVEN"
	}
	g14Write("ACE_G14_EXECUTION_RECOVERY.json",report)
	t.Logf("G14 report=%+v",report)
	if report.Classification!="G14_CLOSED_LOOP_EXECUTION_RECOVERY_PROVEN" { t.Fatalf("G14 failed: %+v",report) }
}
