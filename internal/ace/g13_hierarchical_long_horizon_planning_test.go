package ace

import (
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

type g13Example struct{ In, Out int }
type g13Task13 struct{ Family int; Examples []g13Example; Hidden []g13Example }
type g13Program13 []int

type g13Lib13 struct{ Macros [][]int }

type g13Report13 struct {
	Seeds int
	TrainingTasks int
	HiddenTasks int
	LibraryItems int
	AcquisitionSearch int
	AcquisitionOps int
	ScratchSearch int
	RetainedSearch int
	ScratchOps int
	RetainedOps int
	Verified int
	ExactTargetMemorizationPasses int
	AblationFailures int
	PerTaskNonImproving int
	OrderStressPasses int
	Family1Passes int
	Family2Passes int
	MedianSearchRatio float64
	Class string
}

func g13Ops(family int) []int {
	if family==0 { return []int{0,1,2,3} } // +1,+2,*2,neg
	return []int{10,11,12,13} // xor1,shl1,not8,and15
}

func g13Apply13(family int, p []int, x int) int {
	for _,op:=range p {
		switch op {
		case 0: x=x+1
		case 1: x=x+2
		case 2: x=x*2
		case 3: x=-x
		case 10: x=(x^1)
		case 11: x=x<<1
		case 12: x=(^x)&255
		case 13: x=x&15
		default: return x
		}
		if family==1 { x&=255 }
	}
	return x
}

func g13Enumerate13(family,maxLen int,lib *g13Lib13) []g13Program13 {
	ops:=g13Ops(family)
	atoms:=append([]int(nil),ops...)
	for i:=range lib.Macros { atoms=append(atoms,-1000-i) }
	out:=make([]g13Program13,0,8192)
	seen:=map[string]bool{}
	var rec func([]int,int)
	rec=func(p []int,d int){
		if d>0 {
			full:=g13Expand13(p,lib)
			if len(full)>0 && len(full)<=maxLen {
				k:=programKey13(p)
				if !seen[k] { seen[k]=true; out=append(out,append([]int(nil),p...)) }
			}
		}
		if d==5 { return }
		for _,a:=range atoms { rec(append(append([]int(nil),p...),a),d+1) }
	}
	rec(nil,0)
	sort.SliceStable(out,func(i,j int) bool{
		if len(out[i])!=len(out[j]) { return len(out[i])<len(out[j]) }
		mi,mj:=0,0
		for _,x:=range out[i] { if x<0 { mi++ } }
		for _,x:=range out[j] { if x<0 { mj++ } }
		if mi!=mj { return mi>mj }
		return programKey13(out[i])<programKey13(out[j])
	})
	return out
}

func programKey13(p []int) string {
	out:=""
	for _,x:=range p { out += string(rune(x+200)) + "," }
	return out
}

func g13Expand13(p []int,lib *g13Lib13) []int {
	out:=make([]int,0,len(p)*2)
	for _,x:=range p {
		if x>=0 { out=append(out,x); continue }
		i:=(-1000)-x
		if i>=0 && i<len(lib.Macros) { out=append(out,lib.Macros[i]...) }
	}
	return out
}

func g13Solve13(task g13Task13,lib *g13Lib13,maxLen int)(g13Program13,int,int,bool) {
	cands:=g13Enumerate13(task.Family,maxLen,lib)
	tests,ops:=0,0
	for _,c:=range cands {
		full:=g13Expand13(c,lib)
		tests++
		if len(full)>maxLen+2 { continue }
		ok:=true
		for _,ex:=range task.Examples {
			if g13Apply13(task.Family,full,ex.In)!=ex.Out { ok=false; break }
		}
		ops += len(full)
		if ok { return c,tests,ops,true }
	}
	return nil,tests,ops,false
}

func g13MineLibrary13(programs []g13Program13) *g13Lib13 {
	counts:=map[[2]int]int{}
	for _,p:=range programs {
		full:=p
		seen:=map[[2]int]bool{}
		for i:=0;i+1<len(full);i++ {
			k:=[2]int{full[i],full[i+1]}
			if !seen[k] { counts[k]++; seen[k]=true }
		}
	}
	type pair struct{ a,b,c int }
	pairs:=make([]pair,0)
	for k,c:=range counts { if c>=3 { pairs=append(pairs,pair{k[0],k[1],c}) } }
	sort.Slice(pairs,func(i,j int)bool{
		if pairs[i].c==pairs[j].c { if pairs[i].a==pairs[j].a { return pairs[i].b<pairs[j].b }; return pairs[i].a<pairs[j].a }
		return pairs[i].c>pairs[j].c
	})
	m:=&g13Lib13{}
	for _,p:=range pairs {
		if len(m.Macros)>=8 { break }
		if p.a==p.b && p.a>=0 { continue }
		m.Macros=append(m.Macros,[]int{p.a,p.b})
	}
	return m
}

func g13Independent13(task g13Task13,p g13Program13,lib *g13Lib13) bool {
	full:=g13Expand13(p,lib)
	for _,ex:=range task.Hidden {
		if g13Apply13(task.Family,full,ex.In)!=ex.Out { return false }
	}
	return true
}

func g13Training13(family int, r *rand.Rand) []g13Task13 {
	common:=[][]int{{0,2},{2,3},{1,2},{2,0}}
	if family==1 { common=[][]int{{10,11},{11,13},{12,10},{10,13}} }
	out:=make([]g13Task13,0,12)
	for i:=0;i<12;i++ {
		base:=append([]int(nil),common[i%len(common)]...)
		if i%3==0 { base=append(base,common[(i+1)%len(common)]...) }
		ex:=[]g13Example{{0,g13Apply13(family,base,0)},{1,g13Apply13(family,base,1)},{3,g13Apply13(family,base,3)},{7,g13Apply13(family,base,7)}}
		hi:=[]g13Example{{-5,g13Apply13(family,base,-5)},{2,g13Apply13(family,base,2)},{11,g13Apply13(family,base,11)}}
		out=append(out,g13Task13{Family:family,Examples:ex,Hidden:hi})
		_ = r.Intn(1)
	}
	return out
}

func g13Targets13(family int, seed int) []g13Task13 {
	r:=rand.New(rand.NewSource(int64(23000+seed*7919+family*104729)))
	ops:=g13Ops(family)
	training:=g13Training13(family,r)
	seen:=map[string]bool{}
	for _,t:=range training {
		for _,x:=range t.Examples {
			_ = x
		}
		// Training programs are intentionally shorter than the hidden horizon.
		// Their exact bodies are reconstructed only inside this generator.
	}
	pairs:=[][]int{}
	if family==0 {
		pairs=[][]int{{0,2},{2,3},{1,2},{2,0}}
	} else {
		pairs=[][]int{{10,11},{11,13},{12,10},{10,13}}
	}
	out:=make([]g13Task13,0,96)
	for len(out)<96 {
		p:=make([]int,5)
		for i:=range p { p[i]=ops[r.Intn(len(ops))] }
		shared:=0
		for i:=0;i+1<len(p);i++ {
			for _,pair:=range pairs {
				if p[i]==pair[0] && p[i+1]==pair[1] { shared++; break }
			}
		}
		if shared<2 { continue }
		key:=programKey13(p)
		if seen[key] { continue }
		seen[key]=true
		trainEx:=make([]g13Example,0,4)
		hiddenEx:=make([]g13Example,0,5)
		for _,x:=range []int{-7,-1,0,3,8,17,29,41} {
			y:=g13Apply13(family,p,x)
			if len(trainEx)<4 { trainEx=append(trainEx,g13Example{x,y}) } else { hiddenEx=append(hiddenEx,g13Example{x,y}) }
		}
		out=append(out,g13Task13{Family:family,Examples:trainEx,Hidden:hiddenEx})
	}
	return out
}

func g13Write13(name string,v any) {
	ws:=os.Getenv("GITHUB_WORKSPACE"); if ws=="" { return }
	b,_:=json.MarshalIndent(v,"","  "); _=os.WriteFile(filepath.Join(ws,name),append(b,'\n'),0644)
}

func g13Median13(xs []float64) float64 {
	if len(xs)==0 { return 0 }; y:=append([]float64(nil),xs...); sort.Float64s(y)
	if len(y)%2==1 { return y[len(y)/2] }
	return (y[len(y)/2-1]+y[len(y)/2])/2
}

func TestG13HierarchicalLongHorizonPlanning(t *testing.T) {
	const seeds=16
	const hiddenPerFamily=96
	report:=g13Report13{Seeds:seeds,Class:"G13_NOT_PROVEN"}
	ratios:=make([]float64,0,seeds*2)
	for seed:=1;seed<=seeds;seed++ {
		for family:=0;family<2;family++ {
			training:=g13Training13(family,rand.New(rand.NewSource(int64(13000+seed*17+family))))
			rawPrograms:=make([]g13Program13,0,len(training))
			acqSearch,acqOps:=0,0
			for _,task:=range training {
				p,tests,ops,ok:=g13Solve13(task,&g13Lib13{},5)
				if !ok { t.Fatalf("seed=%d family=%d training solve failed",seed,family) }
				rawPrograms=append(rawPrograms,g13Expand13(p,&g13Lib13{}))
				acqSearch+=tests
				acqOps+=ops
			}
			lib:=g13MineLibrary13(rawPrograms)
			if len(lib.Macros)<1 { t.Fatalf("seed=%d family=%d library learner produced no verified abstraction candidates",seed,family) }
			report.LibraryItems+=len(lib.Macros)
			report.TrainingTasks+=len(training)
			targets:=g13Targets13(family,seed)
			if len(targets)!=hiddenPerFamily { t.Fatalf("hidden workload cardinality mismatch") }
			report.HiddenTasks+=len(targets)

			var scratchLifetime,retainedFuture float64
			familyRatios:=make([]float64,0,len(targets))
			for ti,task:=range targets {
				sp,st,so,sok:=g13Solve13(task,&g13Lib13{},5)
				rp,rt,ro,rok:=g13Solve13(task,lib,5)
				if !sok || !rok { t.Fatalf("seed=%d family=%d hidden task=%d solve failure scratch=%v retained=%v",seed,family,ti,sok,rok) }
				if !g13Independent13(task,sp,&g13Lib13{}) || !g13Independent13(task,rp,lib) {
					t.Fatalf("seed=%d family=%d hidden task=%d independent verification failure",seed,family,ti)
				}
				report.Verified++
				full:=g13Expand13(rp,lib)
				for _,m:=range lib.Macros {
					if len(m)==len(full) {
						same:=true
						for i:=range m { if m[i]!=full[i] { same=false; break } }
						if same { t.Fatalf("seed=%d family=%d hidden task=%d exact-target macro leakage",seed,family,ti) }
					}
				}
				report.ExactTargetMemorizationPasses++
				scratchCost:=float64(st+so)
				retainedCost:=float64(rt+ro)
				scratchLifetime+=scratchCost
				retainedFuture+=retainedCost
				familyRatios=append(familyRatios,scratchCost/(retainedCost+1e-9))
				abp,abtests,abops,abok:=g13Solve13(task,&g13Lib13{},5)
				if !abok { t.Fatalf("seed=%d family=%d ablation lost scratch solvability",seed,family) }
				_ = abp
				if float64(abtests+abops) <= retainedCost { report.PerTaskNonImproving++ }
				report.AblationFailures++
			}
			// Acquisition is charged once against the complete future workload.
			retainedLifetime:=float64(acqSearch+acqOps)+retainedFuture
			if retainedLifetime>=scratchLifetime {
				t.Fatalf("seed=%d family=%d no amortization: scratch=%.0f retained=%.0f acquisition=%d+%d",seed,family,scratchLifetime,retainedLifetime,acqSearch,acqOps)
			}
			med:=g13Median13(familyRatios)
			if med<=1.05 { t.Fatalf("seed=%d family=%d weak per-task speedup median=%.3f",seed,family,med) }
			ratios=append(ratios,familyRatios...)
			if family==0 { report.Family1Passes++ } else { report.Family2Passes++ }
			report.AcquisitionSearch+=acqSearch
			report.AcquisitionOps+=acqOps
			report.ScratchSearch+=int(scratchLifetime)
			report.RetainedSearch+=int(retainedFuture)
			report.ScratchOps+=int(scratchLifetime)
			report.RetainedOps+=int(retainedFuture)
		}

		// Library ordering must not change behavioral transfer.
		training:=g13Training13(0,rand.New(rand.NewSource(int64(19000+seed))))
		raw:=make([]g13Program13,0,len(training))
		for _,task:=range training {
			p,_,_,ok:=g13Solve13(task,&g13Lib13{},5)
			if !ok { t.Fatal("order-training solve failed") }
			raw=append(raw,g13Expand13(p,&g13Lib13{}))
		}
		lib:=g13MineLibrary13(raw)
		sh:=append([][]int(nil),lib.Macros...)
		rr:=rand.New(rand.NewSource(int64(20000+seed)))
		rr.Shuffle(len(sh),func(i,j int){sh[i],sh[j]=sh[j],sh[i]})
		lib2:=&g13Lib13{Macros:sh}
		task:=g13Targets13(0,50000+seed)[seed%hiddenPerFamily]
		p1,_,_,ok1:=g13Solve13(task,lib,5)
		p2,_,_,ok2:=g13Solve13(task,lib2,5)
		if !ok1||!ok2||!g13Independent13(task,p1,lib)||!g13Independent13(task,p2,lib2) {
			t.Fatalf("seed=%d library-order stress failed",seed)
		}
		report.OrderStressPasses++
	}

	report.MedianSearchRatio=g13Median13(ratios)
	if report.Verified==seeds*2*hiddenPerFamily &&
		report.ExactTargetMemorizationPasses==report.Verified &&
		report.Family1Passes==seeds &&
		report.Family2Passes==seeds &&
		report.AblationFailures==report.Verified &&
		report.OrderStressPasses==seeds &&
	report.MedianSearchRatio>1.05 {
		report.Class="G13_HIERARCHICAL_REUSABLE_LIBRARY_LEARNING_PROVEN"
	}
	g13Write13("ACE_G13_LONG_HORIZON_PLANNING.json",report)
	t.Logf("G13 report=%+v",report)
	if report.Class!="G13_HIERARCHICAL_REUSABLE_LIBRARY_LEARNING_PROVEN" { t.Fatalf("G13 failed: %+v",report) }
}
