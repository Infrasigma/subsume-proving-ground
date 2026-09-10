package main

// Independent executable development reference for Phase 2.1.
// It constructs tasks and interacts with the environment itself; it never reads
// Python traces, fixture answers, or attribution labels.

import (
  "crypto/sha256"
  "encoding/binary"
  "encoding/hex"
  "encoding/json"
  "fmt"
  "os"
  "sort"
)

type Edge struct{ U,V int }
type Rel struct{ U,V int }
type Task struct { Seed int; Family string; N,Goal int; Base,Deps []Edge; Token map[string]string; Loc map[int]string; E []bool; At int; Done bool }
type Obs struct { State map[string]string `json:"state"`; Available []string `json:"available_actions"`; Last *string `json:"last_action"`; Result map[string]string `json:"last_result"`; Terminated bool `json:"terminated"` }
type Ledger struct { Obs []Obs; Acts []map[string]any }
func digest(ns string, seed int, condition,purpose string, counter uint64) [32]byte { h:=sha256.New(); h.Write([]byte(ns)); h.Write([]byte{0}); var b [8]byte; binary.BigEndian.PutUint64(b[:],uint64(seed)); h.Write(b[:]); h.Write([]byte{0}); h.Write([]byte(condition)); h.Write([]byte{0}); h.Write([]byte(purpose)); h.Write([]byte{0}); binary.BigEndian.PutUint64(b[:],counter); h.Write(b[:]); var out [32]byte; copy(out[:],h.Sum(nil)); return out }
func word(ns string, seed int, c,p string, n uint64) uint64 { d:=digest(ns,seed,c,p,n); return binary.BigEndian.Uint64(d[:8]) }
func uniform(ns string,seed int,c,p string,n uint64, bound int) (int,uint64) { lim:=(^uint64(0)/uint64(bound))*uint64(bound); for { r:=word(ns,seed,c,p,n); if r<lim { return int(r%uint64(bound)),n+1 }; n++ } }
func shuffle(a []int,seed int,c,p string) []int { x:=append([]int(nil),a...); var n uint64; for i:=len(x)-1;i>0;i-- { j,k:=uniform("TASK_GENERATION",seed,c,p,n,i+1); n=k; x[i],x[j]=x[j],x[i] }; return x }
func token(seed int,c,p string,i int) string { d:=digest("LABEL_PERMUTATION",seed,c,p,uint64(i)); return hex.EncodeToString(d[:])[:16] }
func key(k,a,b,r int) string { return fmt.Sprintf("%d:%d:%d:%d",k,a,b,r) }
func NewTask(seed int, family string) Task { t:=Task{Seed:seed,Family:family,Token:map[string]string{},Loc:map[int]string{}}; if family=="A" { t.N=8;t.Goal=7;for i:=0;i<7;i++{t.Base=append(t.Base,Edge{i,i+1})};p:=shuffle([]int{0,1,2,3,4},seed,"A","dependency_sources");sort.Ints(p);for _,u:=range p[:3]{t.Deps=append(t.Deps,Edge{u,u+2})} } else {t.N=10;t.Goal=9;t.Base=[]Edge{{0,1},{1,2},{2,3},{2,4},{3,5},{5,6},{4,6},{6,7},{7,8},{6,8},{8,9}};cand:=t.Base;need:=3;if seed%2==1{need=4};order:=shuffle([]int{0,1,2,3,4,5,6,7,8,9,10},seed,"B","dependency_subset");for _,i:=range order{if len(t.Deps)==need{break};e:=cand[i];dup:=false;for _,d:=range t.Deps{if d==e{dup=true}};if !dup{t.Deps=append(t.Deps,e)}}};t.E=make([]bool,len(t.Deps));sem:=[]string{};for _,e:=range t.Base{sem=append(sem,key(0,e.U,e.V,0))};for r,e:=range t.Deps{sem=append(sem,key(1,e.U,e.U,r),key(2,e.U,e.U,r),key(3,e.U,e.V,r))};sort.Strings(sem);pool:=[]string{};for i:=range sem{pool=append(pool,token(seed,"action","action_pool",i))};pool=shuffleLabel(pool,seed,"action","action_assignment");for i,s:=range sem{t.Token[s]=pool[i]};lp:=[]string{};for i:=0;i<t.N;i++{lp=append(lp,token(seed,"location","location_pool",i))};lp=shuffleLabel(lp,seed,"location","location_assignment");for i,s:=range lp{t.Loc[i]=s};return t }
func shuffleLabel(a []string,seed int,c,p string) []string {x:=append([]string(nil),a...);var n uint64;for i:=len(x)-1;i>0;i--{j,k:=uniform("LABEL_PERMUTATION",seed,c,p,n,i+1);n=k;x[i],x[j]=x[j],x[i]};return x}
func (t *Task) sem(tok string)(int,int,int,int,bool){for s,v:=range t.Token{if v==tok{var k,a,b,r int;if _,e:=fmt.Sscanf(s,"%d:%d:%d:%d",&k,&a,&b,&r);e==nil{return k,a,b,r,true}}};return 0,0,0,0,false}
func (t *Task) available() []string {out:=[]string{};for s,tok:=range t.Token{k,a,_,r,ok:=parseSem(s);if !ok||a!=t.At{continue};if k==3&&!t.E[r]{continue};out=append(out,tok)};sort.Strings(out);return out}
func parseSem(s string)(int,int,int,int,bool){var k,a,b,r int;_,e:=fmt.Sscanf(s,"%d:%d:%d:%d",&k,&a,&b,&r);return k,a,b,r,e==nil}
func (t *Task) step(tok string) map[string]string {k,a,b,r,ok:=t.sem(tok);if !ok{return map[string]string{"status":"ILLEGAL_ACTION"}};if a!=t.At{return map[string]string{"status":"BLOCKED"}};if k==3&&!t.E[r]{return map[string]string{"status":"BLOCKED"}};switch k{case 0:t.At=b;case 1:t.E[r]=true;case 2:case 3:t.At=b};if t.At==t.Goal{t.Done=true;return map[string]string{"status":"SUCCESS"}};return map[string]string{"status":"ACCEPTED"}}
func (t *Task) observation(last *string,res map[string]string) Obs{return Obs{State:map[string]string{"location":t.Loc[t.At]},Available:t.available(),Last:last,Result:res,Terminated:t.Done}}
func run(t Task,limit int)(Ledger,string){l:=Ledger{};var last *string;l.Obs=append(l.Obs,t.observation(nil,nil));for !t.Done&&len(l.Acts)<limit{a:=t.available();if len(a)==0{break};x:=a[0];r:=t.step(x);last=&x;l.Acts=append(l.Acts,map[string]any{"index":len(l.Acts),"action":x,"result":r});l.Obs=append(l.Obs,t.observation(last,r))};term:="DECISION_CUTOFF_EXHAUSTED";if t.Done{term="SUCCESS"}else if len(l.Acts)>=limit{term="INTERACTION_CAP_EXHAUSTED"};return l,term}
func main(){if len(os.Args)>1&&os.Args[1]=="--definitive"{panic("REFUSED: definitive 100-task experiment")};a,_:=run(NewTask(0,"A"),20);b,_:=run(NewTask(0,"B"),30);out:=map[string]any{"A_observations":a.Obs,"A_actions":a.Acts,"B_observations":b.Obs,"B_actions":b.Acts};j,_:=json.Marshal(out);fmt.Println(string(j))}
