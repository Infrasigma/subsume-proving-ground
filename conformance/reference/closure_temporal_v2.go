package main

import (
  "bufio"
  "bytes"
  "crypto/sha256"
  "encoding/hex"
  "encoding/json"
  "fmt"
  "os"
  "regexp"
  "sort"
)

type Observation struct { State map[string]string `json:"state"`; Available []string `json:"available_actions"`; LastAction any `json:"last_action"`; LastResult any `json:"last_result"`; Terminated bool `json:"terminated"` }
type Atom struct { Predicate string `json:"predicate"`; Args []string `json:"args"` }
type Rule struct { Atoms []Atom `json:"atoms"`; Consequence string `json:"consequence"` }
type Input struct { Seed uint64 `json:"seed"`; History []Observation `json:"history"`; Candidate Rule `json:"candidate"`; Attempts int `json:"attempts"`; Terminal string `json:"terminal"`; Attribution map[string]any `json:"attribution"`; NoveltyA []map[string]any `json:"novelty_a"`; NoveltyB []map[string]any `json:"novelty_b"` }
type Fact struct { P string `json:"p"`; A []string `json:"a"`; At []int `json:"at"` }

var roles=[]string{"target","intervention","contrast","context"}
var arity=map[string]int{"BLOCKED":1,"AVAILABLE":1,"ACTION":1,"INTERVENES":2,"BEFORE":2,"AFTER":2,"OBSERVED_EFFECT":2,"ENABLES":2,"NONENABLES":2,"SAME_LOCAL_CONTEXT":2}
var tokenRE=regexp.MustCompile(`^[0-9a-f]{16}$`)
var consequences=map[string]bool{"ENABLES(target)":true,"NONENABLES(contrast,target)":true}

func canon(v any)[]byte { raw,err:=json.Marshal(v); if err!=nil {panic(err)}; dec:=json.NewDecoder(bytes.NewReader(raw)); dec.UseNumber(); var x any; if err:=dec.Decode(&x);err!=nil{panic(err)}; out,err:=json.Marshal(x);if err!=nil{panic(err)};return out }
func hash(v any)string{h:=sha256.Sum256(canon(v));return hex.EncodeToString(h[:])}
func put64(b []byte,x uint64){for i:=0;i<8;i++{b[i]=byte(x>>(56-8*i))}}
func stream(ns string,seed,counter uint64,condition,purpose string)[]byte{b:=append([]byte(ns+"\x00"),make([]byte,8)...);put64(b[len(ns)+1:],seed);b=append(b,0);b=append(b,[]byte(condition)...);b=append(b,0);b=append(b,[]byte(purpose)...);b=append(b,0);x:=make([]byte,8);put64(x,counter);return append(b,x...)}
func word(ns string,seed,counter uint64,condition,purpose string)uint64{h:=sha256.Sum256(stream(ns,seed,counter,condition,purpose));var x uint64;for _,b:=range h[:8]{x=x<<8|uint64(b)};return x}
func token(ns string,seed,index uint64,condition,purpose string)string{h:=sha256.Sum256(stream(ns,seed,index,condition,purpose));return hex.EncodeToString(h[:])[:16]}
func equal(a,b []string)bool{if len(a)!=len(b){return false};for i:=range a{if a[i]!=b[i]{return false}};return true}
func validObs(o Observation)bool{if len(o.State)!=1||!tokenRE.MatchString(o.State["location"]){return false};cp:=append([]string(nil),o.Available...);sort.Strings(cp);if !equal(cp,o.Available){return false};seen:=map[string]bool{};for _,x:=range o.Available{if !tokenRE.MatchString(x)||seen[x]{return false};seen[x]=true};if o.LastAction!=nil{if x,ok:=o.LastAction.(string);!ok||!tokenRE.MatchString(x){return false}};if o.LastResult!=nil{m,ok:=o.LastResult.(map[string]any);if !ok||len(m)!=1{return false};s,_:=m["status"].(string);if s!="BLOCKED"&&s!="ACCEPTED"&&s!="SUCCESS"&&s!="ILLEGAL_ACTION"&&s!="ENVIRONMENT_ERROR"{return false}};return true}
func atomKey(a Atom)string{return string(canon(map[string]any{"args":a.Args,"predicate":a.Predicate}))}
func canonical(r Rule)Rule{seen:=map[string]Atom{};for _,a:=range r.Atoms{seen[atomKey(a)]=a};ks:=make([]string,0,len(seen));for k:=range seen{ks=append(ks,k)};sort.Strings(ks);out:=make([]Atom,0,len(ks));for _,k:=range ks{out=append(out,seen[k])};r.Atoms=out;return r}
func validRule(r Rule)bool{if !consequences[r.Consequence]||len(r.Atoms)<1||len(r.Atoms)>6{return false};seen:=map[string]bool{};bound:=map[string]bool{};for _,a:=range r.Atoms{n,ok:=arity[a.Predicate];if !ok||len(a.Args)!=n{return false};for _,x:=range a.Args{ok=false;for _,rr:=range roles{if x==rr{ok=true;bound[x]=true;break}};if !ok{return false}};k:=atomKey(a);if seen[k]{return false};seen[k]=true};if !bound["target"]{return false};if r.Consequence=="NONENABLES(contrast,target)"&&!bound["contrast"]{return false};return true}

// Fact identity is predicate + arguments. Temporal multiplicity is provenance,
// represented as a sorted unique timestamp set. Identical occurrences at the
// same timestamp are duplicates; occurrences at different timestamps are
// retained in the provenance set.
func facts(h []Observation)[]Fact{
 grouped:=map[string]Fact{}
 add:=func(p string,a []string,t int){k:=string(canon(map[string]any{"p":p,"a":a}));f,ok:=grouped[k];if !ok{f=Fact{P:p,A:append([]string(nil),a...),At:[]int{}}};for _,v:=range f.At{if v==t{return}};f.At=append(f.At,t);sort.Ints(f.At);grouped[k]=f}
 for i,o:=range h{for _,x:=range o.Available{add("AVAILABLE",[]string{x},i)};if o.LastAction!=nil{x:=o.LastAction.(string);add("ACTION",[]string{x},i)};if i>0&&o.LastAction!=nil{x:=o.LastAction.(string);prev:=map[string]bool{};cur:=map[string]bool{};for _,t:=range h[i-1].Available{prev[t]=true};for _,t:=range o.Available{cur[t]=true};all:=map[string]bool{};for t:=range prev{all[t]=true};for t:=range cur{all[t]=true};ks:=make([]string,0,len(all));for t:=range all{ks=append(ks,t)};sort.Strings(ks);for _,t:=range ks{if !prev[t]&&cur[t]{add("INTERVENES",[]string{x,t},i-1);add("OBSERVED_EFFECT",[]string{x,"ENABLES("+t+")"},i-1)}else if !prev[t]&&!cur[t]{add("NONENABLES",[]string{x,t},i-1)}};if m,ok:=o.LastResult.(map[string]any);ok{if s,_:=m["status"].(string);s=="BLOCKED"{add("BLOCKED",[]string{x},i)}}}}
 out:=make([]Fact,0,len(grouped));for _,f:=range grouped{out=append(out,f)};sort.Slice(out,func(i,j int)bool{return string(canon(out[i]))<string(canon(out[j]))});return out
}
func match(r Rule,b map[string]string,fs []Fact)bool{for _,a:=range r.Atoms{args:=make([]string,len(a.Args));for i,x:=range a.Args{args[i]=b[x]};ok:=false;for _,f:=range fs{if f.P==a.Predicate&&equal(f.A,args){ok=true;break}};if !ok{return false}};return true}
func retrieve(r Rule,fs []Fact)map[string]any{ents:=map[string]bool{};for _,f:=range fs{for _,x:=range f.A{if tokenRE.MatchString(x){ents[x]=true}}};es:=make([]string,0,len(ents));for x:=range ents{es=append(es,x)};sort.Strings(es);rs:=map[string]bool{};for _,a:=range r.Atoms{for _,x:=range a.Args{rs[x]=true}};rv:=make([]string,0,len(rs));for x:=range rs{rv=append(rv,x)};sort.Slice(rv,func(i,j int)bool{return roleIndex(rv[i])<roleIndex(rv[j])});var rec func(int,map[string]string)bool;rec=func(i int,b map[string]string)bool{if i==len(rv){return match(r,b,fs)};for _,x:=range es{b[rv[i]]=x;if rec(i+1,b){return true}};return false};b:=map[string]string{};if rec(0,b){return map[string]any{"status":"RETRIEVED","binding":b,"rule_hash":hash(canonical(r))}};return map[string]any{"status":"NONE"}}
func roleIndex(x string)int{for i,r:=range roles{if x==r{return i}};return 99}
func attr(a map[string]any)string{if b,_:=a["outside_domain"].(bool);b{return "PROTOCOL_VIOLATION"};set:=map[string]bool{};if v,ok:=a["evidence"].([]any);ok{for _,x:=range v{if s,ok:=x.(string);ok{set[s]=true}}};if set["invalid"]||len(set)==0{return "UNATTRIBUTABLE"};if set["conflict"]{return "CONFLICT"};if set["retrieval"]&&!set["prediction"]{return "RETRIEVAL_ONLY"};if set["prediction"]&&!set["intervention"]{return "PREDICTION_ONLY"};if set["coincidental"]{return "COINCIDENTAL"};need:=[]string{"retrieval","grounding","prediction","decisive_action","intervention","consequence","verification","counterfactual"};for _,x:=range need{if !set[x]{return "UNATTRIBUTABLE"}};return "KA_TRANSFER"}
func novelty(a,b []map[string]any)float64{ca:=map[string]int{};cb:=map[string]int{};for _,x:=range a{ca[string(canon(x))]++};for _,x:=range b{cb[string(canon(x))]++};u,i:=0,0;keys:=map[string]bool{};for k:=range ca{keys[k]=true};for k:=range cb{keys[k]=true};for k:=range keys{aa,bb:=ca[k],cb[k];if aa>bb{u+=aa}else{u+=bb};if aa<bb{i+=aa}else{i+=bb}};if u==0{return 1};return 1-float64(i)/float64(u)}
func controls(h []Observation)map[string]any{if len(h)==0{return map[string]any{"K0":nil,"KR":nil,"KS":nil,"KP":nil,"REPLAY":nil}};a:=append([]string(nil),h[len(h)-1].Available...);sort.Strings(a);var first any;if len(a)>0{first=a[0]};return map[string]any{"K0":first,"KR":first,"KS":first,"KP":first,"REPLAY":nil}}
func main(){sc:=bufio.NewScanner(os.Stdin);sc.Scan();var in Input;if json.Unmarshal(sc.Bytes(),&in)!=nil{panic("bad input")};for _,o:=range in.History{if !validObs(o){panic("invalid observation")}};r:=canonical(in.Candidate);if !validRule(r){panic("invalid rule")};fs:=facts(in.History);ret:=retrieve(r,fs);var pred any;if ret["status"]=="RETRIEVED"{pred=r.Consequence};out:=map[string]any{"input_hash":hash(in),"facts":fs,"candidate":r,"candidate_valid":true,"retrieval":ret,"prediction":map[string]any{"consequence":pred,"valid":pred!=nil},"cost":func()any{if in.Terminal=="SUCCESS"||in.Terminal=="INTERACTION_CAP_EXHAUSTED"||in.Terminal=="DECISION_CUTOFF_EXHAUSTED"{return in.Attempts};return nil}(),"attribution":attr(in.Attribution),"novelty":novelty(in.NoveltyA,in.NoveltyB),"controls":controls(in.History),"tokens":map[string]string{"action":token("LABEL_PERMUTATION",in.Seed,0,"","action_token_pool"),"location":token("LABEL_PERMUTATION",in.Seed,0,"","location_token_pool")},"rng":[]uint64{word("TASK_GENERATION",in.Seed,0,"","family_b_dependency_subset"),word("TASK_GENERATION",in.Seed,1,"","family_b_dependency_subset"),word("TASK_GENERATION",in.Seed,2,"","family_b_dependency_subset"),word("TASK_GENERATION",in.Seed,3,"","family_b_dependency_subset")}};fmt.Println(string(canon(out)))}
