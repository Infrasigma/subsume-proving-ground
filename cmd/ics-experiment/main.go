package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strings"
)

type Obj struct{ ID, Value int }
type State struct {
	Objects []Obj
	Edges   [][2]int
}
type Action struct{ Target int }

type Rule struct {
	Scope     int
	Effect    int
	Predicate int
}

const (
	ScopeTarget = iota
	ScopeOut
	ScopeIn
)
const (
	EffectInc = iota
	EffectToggle
)
const (
	PredAlways = iota
	PredEq0
	PredEq1
)

func (r Rule) ID() string { return fmt.Sprintf("s%d-e%d-p%d", r.Scope, r.Effect, r.Predicate) }

func cloneState(s State) State {
	return State{Objects: append([]Obj(nil), s.Objects...), Edges: append([][2]int(nil), s.Edges...)}
}
func objectValue(s State, id int) int {
	for _, o := range s.Objects { if o.ID == id { return o.Value } }
	return 0
}
func setObjectValue(s *State, id, v int) {
	for i := range s.Objects { if s.Objects[i].ID == id { s.Objects[i].Value = v; return } }
}
func uniqueInts(xs []int) []int {
	seen:=map[int]bool{};out:=make([]int,0,len(xs))
	for _,x:=range xs { if !seen[x] { seen[x]=true;out=append(out,x) } }
	sort.Ints(out);return out
}
func uniqueEdges(es [][2]int) [][2]int {
	seen:=map[string]bool{};out:=make([][2]int,0,len(es))
	for _,e:=range es { k:=fmt.Sprintf("%d>%d",e[0],e[1]);if !seen[k]{seen[k]=true;out=append(out,e)} }
	sort.Slice(out,func(i,j int)bool{if out[i][0]!=out[j][0]{return out[i][0]<out[j][0]};return out[i][1]<out[j][1]})
	return out
}
func targets(s State,a Action,r Rule) []int {
	switch r.Scope {
	case ScopeTarget:return []int{a.Target}
	case ScopeOut:
		out:=[]int{a.Target};for _,e:=range s.Edges{if e[0]==a.Target{out=append(out,e[1])}};return uniqueInts(out)
	case ScopeIn:
		out:=[]int{a.Target};for _,e:=range s.Edges{if e[1]==a.Target{out=append(out,e[0])}};return uniqueInts(out)
	default:return []int{a.Target}
	}
}
func apply(s State,a Action,r Rule) State {
	n:=cloneState(s);v:=objectValue(s,a.Target)
	ok:=r.Predicate==PredAlways||(r.Predicate==PredEq0&&v==0)||(r.Predicate==PredEq1&&v==1)
	if !ok{return n}
	for _,id:=range targets(s,a,r){
		old:=objectValue(s,id)
		if r.Effect==EffectInc{setObjectValue(&n,id,old+1)}else if old==0{setObjectValue(&n,id,1)}else{setObjectValue(&n,id,0)}
	}
	return n
}
func stateSig(s State) string {
	o:=append([]Obj(nil),s.Objects...);sort.Slice(o,func(i,j int)bool{return o[i].ID<o[j].ID})
	var b strings.Builder
	for _,x:=range o{fmt.Fprintf(&b,"o%d=%d;",x.ID,x.Value)}
	b.WriteString("|");e:=append([][2]int(nil),s.Edges...);sort.Slice(e,func(i,j int)bool{if e[i][0]!=e[j][0]{return e[i][0]<e[j][0]};return e[i][1]<e[j][1]})
	for _,x:=range e{fmt.Fprintf(&b,"%d>%d;",x[0],x[1])}
	return b.String()
}
func allRules() []Rule {
	var out []Rule
	for s:=0;s<3;s++{for e:=0;e<2;e++{for p:=0;p<3;p++{out=append(out,Rule{Scope:s,Effect:e,Predicate:p})}}}
	return out
}
func randomWorld(r *rand.Rand)(State,Rule){
	n:=5;s:=State{}
	for i:=0;i<n;i++{s.Objects=append(s.Objects,Obj{ID:i,Value:r.Intn(2)})}
	for i:=0;i<n;i++{j:=r.Intn(n);if j==i{j=(j+1)%n};s.Edges=append(s.Edges,[2]int{i,j})}
	s.Edges=uniqueEdges(s.Edges)
	rs:=allRules()
	return s,rs[r.Intn(len(rs))]
}

func render(s State,surface int,r *rand.Rand) string {
	ids:=[]int{};for _,o:=range s.Objects{ids=append(ids,o.ID)}
	r.Shuffle(len(ids),func(i,j int){ids[i],ids[j]=ids[j],ids[i]})
	rename:=map[int]int{};for i,id:=range ids{rename[id]=i+10}
	parts:=[]string{};for _,id:=range ids{parts=append(parts,fmt.Sprintf("o%d:%d",rename[id],objectValue(s,id)))}
	edges:=append([][2]int(nil),s.Edges...);r.Shuffle(len(edges),func(i,j int){edges[i],edges[j]=edges[j],edges[i]})
	for _,e:=range edges{parts=append(parts,fmt.Sprintf("r%d>%d",rename[e[0]],rename[e[1]]))}
	switch surface{
	case 0:return strings.Join(parts,";")
	case 1:return strings.Join(parts," | ")
	case 2:return strings.Join(parts,",")
	case 3:return "["+strings.Join(parts," ][ ")+"]"
	case 4:return strings.Join(parts,"\n")
	default:return strings.Join(parts,";")
	}
}
func decodeSurface(raw string)(State,error){
	clean:=strings.NewReplacer("[","","]","").Replace(raw)
	toks:=strings.FieldsFunc(clean,func(r rune)bool{return r==';'||r=='|'||r==','||r=='\n'})
	vals:=map[int]int{};edges:=[][2]int{}
	for _,t:=range toks{
		t=strings.TrimSpace(t);if t==""{continue}
		if strings.HasPrefix(t,"o"){var id,v int;if _,e:=fmt.Sscanf(t,"o%d:%d",&id,&v);e!=nil{return State{},e};vals[id]=v
		}else if strings.HasPrefix(t,"r"){var a,b int;if _,e:=fmt.Sscanf(t,"r%d>%d",&a,&b);e!=nil{return State{},e};edges=append(edges,[2]int{a,b})
		}else{return State{},fmt.Errorf("unknown token %q",t)}
	}
	ids:=make([]int,0,len(vals));for id:=range vals{ids=append(ids,id)};sort.Ints(ids)
	objs:=make([]Obj,0,len(ids));idmap:=map[int]int{}
	for i,id:=range ids{idmap[id]=i;objs=append(objs,Obj{ID:i,Value:vals[id]})}
	for i,e:=range edges{edges[i]=[2]int{idmap[e[0]],idmap[e[1]]}}
	return State{Objects:objs,Edges:uniqueEdges(edges)},nil
}

type transition struct{Before State;Action Action;After State}

func candidates(s State,obs []transition)[]Rule{
	out:=[]Rule{}
	for _,r:=range allRules(){
		ok:=true
		for _,tr:=range obs{if stateSig(apply(tr.Before,tr.Action,r))!=stateSig(tr.After){ok=false;break}}
		if ok{out=append(out,r)}
	}
	return out
}
func bestActiveAction(s State,cands []Rule)Action{
	best:=Action{Target:0};bestScore:=-1
	for id:=range s.Objects{
		sigs:=map[string]bool{}
		for _,r:=range cands{sigs[stateSig(apply(s,Action{Target:id},r))]=true}
		if len(sigs)>bestScore{bestScore=len(sigs);best=Action{Target:id}}
	}
	return best
}
func randomAction(s State,r *rand.Rand)Action{return Action{Target:r.Intn(len(s.Objects))}}

func infer(s State,hidden Rule,active bool,seed int)(Rule,int,bool){
	r:=rand.New(rand.NewSource(int64(seed)));cur:=s;obs:=[]transition{};const max=10
	for step:=0;step<max;step++{
		c:=candidates(cur,obs);if len(c)==1{return c[0],step,true}
		var a Action;if active{a=bestActiveAction(cur,c)}else{a=randomAction(cur,r)}
		n:=apply(cur,a,hidden);obs=append(obs,transition{cur,a,n});cur=n
	}
	c:=candidates(cur,obs);for _,x:=range c{if x==hidden{return x,max,false}}
	return Rule{},max,false
}

func predictTransfer(rule Rule,base State,hidden Rule)bool{
	for surface:=0;surface<5;surface++{
		r:=rand.New(rand.NewSource(int64(1000+surface)))
		raw:=render(base,surface,r);decoded,e:=decodeSurface(raw);if e!=nil{return false}
		for a:=range decoded.Objects{if stateSig(apply(decoded,Action{Target:a},rule))!=stateSig(apply(decoded,Action{Target:a},hidden)){return false}}
	}
	return true
}

type Result struct {
	Seeds int
	Worlds int
	ActiveIdentified int
	RandomIdentified int
	ActiveRate float64
	RandomRate float64
	ActiveMeanInteractions float64
	RandomMeanInteractions float64
	CrossSurfaceTransfer int
	CrossSurfaceTotal int
	CrossSurfaceRate float64
	TransferCostRatio float64
	Verdict string
	KillReasons []string
}
func mean(xs []int)float64{if len(xs)==0{return 0};t:=0;for _,x:=range xs{t+=x};return float64(t)/float64(len(xs))}

func main(){
	const seeds=64
	const worldsPerSeed=32
	activeOK,randomOK,transferOK:=0,0,0
	var activeCosts,randomCosts []int
	total:=seeds*worldsPerSeed
	for seed:=0;seed<seeds;seed++{
		r:=rand.New(rand.NewSource(int64(900000+seed)))
		for w:=0;w<worldsPerSeed;w++{
			base,hidden:=randomWorld(r)
			arule,acost,aident:=infer(base,hidden,true,seed*10000+w)
			rrule,rcost,rident:=infer(base,hidden,false,seed*10000+w+777)
			if aident&&arule==hidden{activeOK++}
			if rident&&rrule==hidden{randomOK++}
			activeCosts=append(activeCosts,acost);randomCosts=append(randomCosts,rcost)
			if arule==hidden&&predictTransfer(arule,base,hidden){transferOK++}
		}
	}
	activeRate:=float64(activeOK)/float64(total)
	randomRate:=float64(randomOK)/float64(total)
	transferRate:=float64(transferOK)/float64(total)
	activeMean,randomMean:=mean(activeCosts),mean(randomCosts)
	ratio:=activeMean/randomMean
	kill:=[]string{}
	if activeRate<0.90{kill=append(kill,fmt.Sprintf("active identification %.3f < 0.90",activeRate))}
	if transferRate<0.95{kill=append(kill,fmt.Sprintf("cross-surface schema transfer %.3f < 0.95",transferRate))}
	if ratio>0.70{kill=append(kill,fmt.Sprintf("active intervention cost ratio %.3f > 0.70",ratio))}
	if activeRate<randomRate-0.10{kill=append(kill,fmt.Sprintf("active underperforms random: %.3f vs %.3f",activeRate,randomRate))}
	verdict:="SURVIVES_INITIAL_FALSIFICATION";if len(kill)>0{verdict="KILLED"}
	res:=Result{Seeds:seeds,Worlds:total,ActiveIdentified:activeOK,RandomIdentified:randomOK,ActiveRate:activeRate,RandomRate:randomRate,ActiveMeanInteractions:activeMean,RandomMeanInteractions:randomMean,CrossSurfaceTransfer:transferOK,CrossSurfaceTotal:total,CrossSurfaceRate:transferRate,TransferCostRatio:ratio,Verdict:verdict,KillReasons:kill}
	_ = os.MkdirAll("ics_artifacts",0755)
	b,_:=json.MarshalIndent(res,"","  ");_ = os.WriteFile("ics_artifacts/result.json",b,0644);fmt.Println(string(b))
}
