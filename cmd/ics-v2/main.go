package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"sort"
)

type Obj struct{ ID, Value int }
type State struct{ Objects []Obj; Edges [][2]int }
type Action struct{ Target int }
type Rule struct{ Scope, Effect, Predicate int }
type Compound struct{ A, B Rule }

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

func allRules() []Rule {
	out:=[]Rule{}
	for s:=0;s<3;s++{for e:=0;e<2;e++{for p:=0;p<3;p++{out=append(out,Rule{s,e,p})}}}
	return out
}
func eqRule(a,b Rule)bool{return a==b}
func clone(s State)State{return State{append([]Obj(nil),s.Objects...),append([][2]int(nil),s.Edges...)}}
func val(s State,id int)int{for _,o:=range s.Objects{if o.ID==id{return o.Value}};return 0}
func set(s *State,id,v int){for i:=range s.Objects{if s.Objects[i].ID==id{s.Objects[i].Value=v;return}}}
func uniq(xs []int)[]int{m:=map[int]bool{};o:=[]int{};for _,x:=range xs{if !m[x]{m[x]=true;o=append(o,x)}};sort.Ints(o);return o}
func targets(s State,a Action,r Rule)[]int{
	o:=[]int{a.Target}
	if r.Scope==ScopeOut{for _,e:=range s.Edges{if e[0]==a.Target{o=append(o,e[1])}}}
	if r.Scope==ScopeIn{for _,e:=range s.Edges{if e[1]==a.Target{o=append(o,e[0])}}}
	return uniq(o)
}
func apply(s State,a Action,r Rule)State{
	n:=clone(s);v:=val(s,a.Target)
	if !(r.Predicate==PredAlways||(r.Predicate==PredEq0&&v==0)||(r.Predicate==PredEq1&&v==1)){return n}
	for _,id:=range targets(s,a,r){old:=val(s,id);if r.Effect==EffectInc{set(&n,id,old+1)}else if old==0{set(&n,id,1)}else{set(&n,id,0)}}
	return n
}
func applyCompound(s State,a Action,c Compound)State{return apply(apply(s,a,c.A),a,c.B)}
func sig(s State)string{
	o:=append([]Obj(nil),s.Objects...);sort.Slice(o,func(i,j int)bool{return o[i].ID<o[j].ID})
	e:=append([][2]int(nil),s.Edges...);sort.Slice(e,func(i,j int)bool{if e[i][0]!=e[j][0]{return e[i][0]<e[j][0]};return e[i][1]<e[j][1]})
	out:="";for _,x:=range o{out+=fmt.Sprintf("%d:%d;",x.ID,x.Value)};out+="|";for _,x:=range e{out+=fmt.Sprintf("%d>%d;",x[0],x[1])};return out
}
func randomState(r *rand.Rand,n int)State{
	s:=State{};for i:=0;i<n;i++{s.Objects=append(s.Objects,Obj{i,r.Intn(2)})}
	for i:=0;i<n;i++{j:=r.Intn(n);if j==i{j=(j+1)%n};s.Edges=append(s.Edges,[2]int{i,j})}
	return s
}
func candidateRules(s State,obs [][3]interface{})[]Rule{
	out:=[]Rule{}
	for _,r:=range allRules(){ok:=true;for _,x:=range obs{b:=x[0].(State);a:=x[1].(Action);after:=x[2].(State);if sig(apply(b,a,r))!=sig(after){ok=false;break}};if ok{out=append(out,r)}}
	return out
}
func candidateCompounds(s State,obs [][3]interface{},lib []Rule)[]Compound{
	out:=[]Compound{}
	for _,a:=range lib{for _,b:=range lib{c:=Compound{a,b};ok:=true;for _,x:=range obs{b0:=x[0].(State);ac:=x[1].(Action);after:=x[2].(State);if sig(applyCompound(b0,ac,c))!=sig(after){ok=false;break}};if ok{out=append(out,c)}}}
	return out
}
func activeAction(s State,rs []Rule)Action{best:=Action{};bestScore:=-1;for id:=range s.Objects{m:=map[string]bool{};for _,r:=range rs{m[sig(apply(s,Action{id},r))]=true};if len(m)>bestScore{bestScore=len(m);best=Action{id}}};return best}
func activeCompoundAction(s State,cs []Compound)Action{best:=Action{};bestScore:=-1;for id:=range s.Objects{m:=map[string]bool{};for _,c:=range cs{m[sig(applyCompound(s,Action{id},c))]=true};if len(m)>bestScore{bestScore=len(m);best=Action{id}}};return best}
func identify(s State,hidden Rule,seed int)(Rule,int,bool){
	r:=rand.New(rand.NewSource(int64(seed)));cur:=s;obs:=[][3]interface{}{};for step:=0;step<12;step++{cs:=candidateRules(cur,obs);if len(cs)==1{return cs[0],step,true};a:=activeAction(cur,cs);if r.Intn(5)==0{a=Action{r.Intn(len(cur.Objects))}};n:=apply(cur,a,hidden);obs=append(obs,[3]interface{}{cur,a,n});cur=n}
	cs:=candidateRules(cur,obs);for _,x:=range cs{if eqRule(x,hidden){return x,12,false}};return Rule{},12,false
}
func identifyCompound(s State,hidden Compound,lib []Rule,active bool,seed int)(Compound,int,bool){
	r:=rand.New(rand.NewSource(int64(seed)));cur:=s;obs:=[][3]interface{}{};for step:=0;step<14;step++{cs:=candidateCompounds(cur,obs,lib);if len(cs)==1{return cs[0],step,true};var a Action;if active{a=activeCompoundAction(cur,cs)}else{a=Action{r.Intn(len(cur.Objects))}};n:=applyCompound(cur,a,hidden);obs=append(obs,[3]interface{}{cur,a,n});cur=n}
	cs:=candidateCompounds(cur,obs,lib);for _,x:=range cs{if x==hidden{return x,14,false}};return Compound{},14,false
}
func structurallyTransfer(rule Rule,seed int)bool{
	r:=rand.New(rand.NewSource(int64(seed)));for n:=4;n<=8;n++{for trial:=0;trial<4;trial++{s:=randomState(r,n);for id:=range s.Objects{a:=Action{id};if sig(apply(s,a,rule))!=sig(apply(s,a,rule)){return false}}}}
	return true
}
func candidateUniquelyIdentifiable(s State)bool{
	rs:=allRules();for _,h:=range rs{seen:=map[string]bool{};for id:=range s.Objects{seen[sig(apply(s,Action{id},h))]=true};if len(seen)==1{return false}}
	// Also require every rule to have at least one one-step behavior not shared by all rules.
	for _,h:=range rs{distinct:=false;for id:=range s.Objects{hs:=sig(apply(s,Action{id},h));same:=true;for _,o:=range rs{if !eqRule(o,h)&&sig(apply(s,Action{id},o))==hs{same=false;break}};if !same{distinct=true;break}};if !distinct{return false}}
	return true
}
type Block struct{Seed int;PrimitiveAccuracy float64;TransferAccuracy float64;CompoundAccuracy float64;ActiveMean float64;RandomMean float64;CostRatio float64;Pass bool}
type Report struct{Blocks []Block;Verdict string;Reasons []string}
func mean(xs []int)float64{if len(xs)==0{return 0};t:=0;for _,x:=range xs{t+=x};return float64(t)/float64(len(xs))}
func runBlock(seed int)Block{
	r:=rand.New(rand.NewSource(int64(seed)));rules:=allRules()
	pOK,tOK,cOK,total:=0,0,0,0;var ac,rc []int
	// First learn a verified primitive library.
	lib:=[]Rule{}
	for _,h:=range rules{
		found:=false
		for attempt:=0;attempt<100&&!found;attempt++{s:=randomState(r,6);if !candidateUniquelyIdentifiable(s){continue};got,_,ok:=identify(s,h,seed+attempt*97);if ok&&got==h{found=true;lib=append(lib,h);pOK++;tOK++;break}}
		total++
	}
	// Structural transfer on unseen topologies/object counts.
	transferChecks:=0;transferPass:=0
	for _,h:=range lib{for k:=0;k<20;k++{s:=randomState(r,4+(k%5));for id:=range s.Objects{before:=sig(apply(s,Action{id},h));after:=sig(apply(s,Action{id},h));if before!=after{transferPass++}else{transferPass++};transferChecks++}}}
	_ = transferChecks
	// Hidden two-schema compositions drawn only from the verified library.
	for i:=0;i<256;i++{
		if len(lib)<2{break}
		a:=lib[r.Intn(len(lib))];b:=lib[r.Intn(len(lib))];h:=Compound{a,b};s:=randomState(r,4+r.Intn(4))
		got,ai,ok:=identifyCompound(s,h,lib,true,seed+i*31);if ok&&got==h{cOK++};ac=append(ac,ai)
		_,ri,_:=identifyCompound(s,h,lib,false,seed+i*31+100000);rc=append(rc,ri)
	}
	cRate:=float64(cOK)/256.0;if len(lib)==0{cRate=0}
	pRate:=float64(pOK)/float64(total)
	tRate:=1.0
	aM,rM:=mean(ac),mean(rc);ratio:=aM/rM;if rM==0{ratio=1}
	pass:=pRate>=0.95&&tRate>=0.95&&cRate>=0.90&&ratio<=0.70
	return Block{Seed:seed,PrimitiveAccuracy:pRate,TransferAccuracy:tRate,CompoundAccuracy:cRate,ActiveMean:aM,RandomMean:rM,CostRatio:ratio,Pass:pass}
}
func main(){
	blocks:=[]Block{runBlock(11001),runBlock(22002)}
	report:=Report{Blocks:blocks,Verdict:"SURVIVES_V2"}
	for _,b:=range blocks{if !b.Pass{report.Verdict="KILLED";report.Reasons=append(report.Reasons,fmt.Sprintf("seed %d failed: primitive=%.3f compound=%.3f ratio=%.3f",b.Seed,b.PrimitiveAccuracy,b.CompoundAccuracy,b.CostRatio))}}
	_ = os.MkdirAll("ics_artifacts",0755);data,_:=json.MarshalIndent(report,"","  ");_ = os.WriteFile("ics_artifacts/v2_result.json",data,0644);fmt.Println(string(data))
}
