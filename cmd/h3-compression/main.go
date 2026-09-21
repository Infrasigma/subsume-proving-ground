package main

import (
  "encoding/json"
  "fmt"
  "math"
  "math/rand"
  "os"
  "sort"
)

type Episode []int
type Macro struct { Seq []int; Count int; Gain int; ID int }
type Library struct { Macros []Macro }
type Audit struct { Raw Episode; Surface Episode }
type GenMetrics struct {
  Generation int
  RawBitsK0 int
  RawBitsK int
  RawSavings float64
  SurfaceBitsK0 int
  SurfaceBitsK int
  SurfaceSavings float64
  Order2BitsK0 int
  Order2BitsK int
  Order2Savings float64
  Ratio float64
}
type BlockResult struct {
  Seed int
  Generations []GenMetrics
  MeanRecursiveRatio float64
  MinRecursiveRatio float64
  SurfaceRetention float64
  NegativeControlSavings float64
  AllCompositionSavings bool
  AllRecursiveThresholds bool
  SurfacePass bool
  NegativeControlPass bool
  Pass bool
}
type Report struct { Version string; Blocks []BlockResult; Verdict string; Reasons []string }

func canonicalize(ep Episode) Episode {
  m:=map[int]int{}; next:=0; out:=make(Episode,len(ep))
  for i,v:=range ep { if x,ok:=m[v];ok{out[i]=x}else{m[v]=next;out[i]=next;next++} }
  return out
}

func globalRename(ep Episode,r *rand.Rand) Episode {
  p:=r.Perm(1000); used:=map[int]int{}; next:=0; out:=make(Episode,len(ep))
  for i,v:=range ep { if x,ok:=used[v];ok{out[i]=x}else{used[v]=p[next];out[i]=p[next];next++} }
  return out
}

func motifKey(xs []int) string {
  if len(xs)==0{return ""}; out:=make([]byte,0,len(xs)*3)
  for _,v:=range xs{out=append(out,byte(v))}; return string(out)
}

func discoverLibrary(stream []Episode,minLen,maxLen,minCount int) Library {
  counts:=map[string]*Macro{}
  for _,raw:=range stream {
    ep:=canonicalize(raw); seen:=map[string]bool{}
    for n:=minLen;n<=maxLen;n++ {
      for i:=0;i+n<=len(ep);i++ {
        seq:=append([]int(nil),ep[i:i+n]...); k:=motifKey(seq)
        if seen[k]{continue}; seen[k]=true
        m:=counts[k]; if m==nil{m=&Macro{Seq:seq};counts[k]=m};m.Count++
      }
    }
  }
  c:=[]Macro{}
  for _,m:=range counts { if m.Count<minCount{continue};m.Gain=(len(m.Seq)-1)*(m.Count-1);if m.Gain>0{c=append(c,*m)} }
  sort.Slice(c,func(i,j int)bool{if c[i].Gain!=c[j].Gain{return c[i].Gain>c[j].Gain};if len(c[i].Seq)!=len(c[j].Seq){return len(c[i].Seq)>len(c[j].Seq)};return motifKey(c[i].Seq)<motifKey(c[j].Seq)})
  selected:=[]Macro{}
  for _,m:=range c { dup:=false;for _,s:=range selected{if motifKey(s.Seq)==motifKey(m.Seq){dup=true;break}};if dup{continue};m.ID=len(selected);selected=append(selected,m);if len(selected)>=256{break} }
  return Library{Macros:selected}
}

func matchesAt(ep Episode,i int,seq []int)bool{if i+len(seq)>len(ep){return false};for j,v:=range seq{if ep[i+j]!=v{return false}};return true}

func libraryHeaderBits(lib Library)int{
  bits:=0;for _,m:=range lib.Macros{bits+=10+len(m.Seq)*8};return bits
}

func encodeNoHeader(ep Episode,lib Library)int{
  x:=canonicalize(ep);bits:=0;const literal=9;const ref=9
  for i:=0;i<len(x);{best:=0;for _,m:=range lib.Macros{if len(m.Seq)<=best{continue};if matchesAt(x,i,m.Seq){best=len(m.Seq)}};if best>0{bits+=ref;i+=best}else{bits+=literal;i++}}
  return bits
}

func learnedCost(a []Audit,lib Library,surface bool)int{total:=libraryHeaderBits(lib);for _,x:=range a{ep:=x.Raw;if surface{ep=x.Surface};total+=encodeNoHeader(ep,lib)};return total}

func freshCost(a []Audit,surface bool)int{total:=0;for _,x:=range a{ep:=x.Raw;if surface{ep=x.Surface};local:=discoverLibrary([]Episode{ep},3,8,2);total+=libraryHeaderBits(local)+encodeNoHeader(ep,local)};return total}

func savings(base,learned int)float64{if base<=0{return 0};return float64(base-learned)/float64(base)}

func makeMotifs(r *rand.Rand,count int)[]Episode{
  out:=make([]Episode,0,count);for i:=0;i<count;i++{n:=4+r.Intn(5);e:=Episode{};for j:=0;j<n;j++{e=append(e,r.Intn(24))};out=append(out,e)};return out
}
func concat(m []Episode,ids []int)Episode{out:=Episode{};for _,id:=range ids{out=append(out,m[id]...)};return out}
func makeIDs(r *rand.Rand,motifCount,episodes,width int)[][]int{out:=make([][]int,episodes);for e:=0;e<episodes;e++{n:=width+r.Intn(3);row:=make([]int,n);for i:=range row{row[i]=r.Intn(motifCount)};out[e]=row};return out}
func build(m []Episode,ids [][]int)[]Episode{out:=make([]Episode,0,len(ids));for _,row:=range ids{out=append(out,concat(m,row))};return out}
func shuffleStream(r *rand.Rand,s []Episode)[]Episode{out:=make([]Episode,len(s));for i,e:=range s{x:=append(Episode(nil),e...);r.Shuffle(len(x),func(a,b int){x[a],x[b]=x[b],x[a]});out[i]=x};return out}

func audit(r *rand.Rand,m []Episode,count,width int,surface bool)[]Audit{
  out:=make([]Audit,0,count);for i:=0;i<count;i++{ids:=r.Perm(len(m))[:width];raw:=concat(m,ids);x:=raw;if surface{x=globalRename(raw,r)};out=append(out,Audit{Raw:raw,Surface:x})};return out
}

func runBlock(seed int)BlockResult{
  r:=rand.New(rand.NewSource(int64(seed)));motifs:=makeMotifs(r,18);metrics:=[]GenMetrics{}
  for gen:=0;gen<4;gen++{
    train:=build(motifs,makeIDs(r,len(motifs),128,5+gen))
    lib:=discoverLibrary(train,3,10,3)
    raw:=audit(r,motifs,128,5+gen,false); surf:=audit(r,motifs,128,5+gen,true); order2:=audit(r,motifs,96,6+gen,false)
    rk0:=freshCost(raw,false);rk:=learnedCost(raw,lib,false);sk0:=freshCost(surf,true);sk:=learnedCost(surf,lib,true);ok0:=freshCost(order2,false);ok:=learnedCost(order2,lib,false)
    metrics=append(metrics,GenMetrics{Generation:gen,RawBitsK0:rk0,RawBitsK:rk,RawSavings:savings(rk0,rk),SurfaceBitsK0:sk0,SurfaceBitsK:sk,SurfaceSavings:savings(sk0,sk),Order2BitsK0:ok0,Order2BitsK:ok,Order2Savings:savings(ok0,ok),Ratio:func()float64{if rk0==0{return 1};return float64(rk)/float64(rk0)}()})
    // Generic promotion: discovered reusable phrases become part of the next experience vocabulary.
    if gen<3{for _,m:=range lib.Macros{if len(m.Seq)>=7{motifs=append(motifs,append(Episode(nil),m.Seq...))}}}
  }
  negTrain:=shuffleStream(r,build(motifs,makeIDs(r,18,160,7)));negLib:=discoverLibrary(negTrain,3,10,3);negAudit:=audit(r,motifs,160,7,false)
  negSavings:=savings(freshCost(negAudit,false),learnedCost(negAudit,negLib,false))
  sum:=0.0;min:=math.Inf(1);comp:=true;recursive:=true
  for i,m:=range metrics{sum+=m.Ratio;if m.Ratio<min{min=m.Ratio};if m.RawSavings<0.25||m.Order2Savings<0.25{comp=false};if i>0&&m.Ratio>=0.75{recursive=false}}
  retention:=0.0;if metrics[0].RawSavings>0{retention=metrics[0].SurfaceSavings/metrics[0].RawSavings}
  pass:=comp&&recursive&&retention>=0.80&&negSavings<0.05
  return BlockResult{Seed:seed,Generations:metrics,MeanRecursiveRatio:sum/4,MinRecursiveRatio:min,SurfaceRetention:retention,NegativeControlSavings:negSavings,AllCompositionSavings:comp,AllRecursiveThresholds:recursive,SurfacePass:retention>=0.80,NegativeControlPass:negSavings<0.05,Pass:pass}
}

func main(){
  blocks:=[]BlockResult{runBlock(43117),runBlock(92843)};report:=Report{Version:"H3-v1",Blocks:blocks,Verdict:"SURVIVES"}
  for _,b:=range blocks{if !b.Pass{report.Verdict="KILLED";report.Reasons=append(report.Reasons,fmt.Sprintf("seed %d failed: min_ratio=%.3f surface_retention=%.3f negative_savings=%.3f composition=%v recursive=%v",b.Seed,b.MinRecursiveRatio,b.SurfaceRetention,b.NegativeControlSavings,b.AllCompositionSavings,b.AllRecursiveThresholds))}}
  _=os.MkdirAll("h3_artifacts",0755);data,_:=json.MarshalIndent(report,"","  ");_ = os.WriteFile("h3_artifacts/H3_RESULT.json",data,0644);fmt.Println(string(data))
}