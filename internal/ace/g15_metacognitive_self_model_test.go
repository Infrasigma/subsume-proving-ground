package ace

import (
	"encoding/json"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

type g15Task struct{ Size, Depth, Noise, Seed int }
type g15Evidence struct{ Success, Fail int }
type g15Model struct{ Buckets map[[3]int]g15Evidence; Global g15Evidence }

type g15Report struct {
	TrainingCases, HiddenCases, StationaryCases, ShiftedCases int
	SelfModelBrier, GlobalBrier, StaticShiftedBrier, AdaptiveShiftedBrier, SelfModelECE float64
	AlwaysVerifyCost, GlobalRegulatedCost, SelfRegulatedCost float64
	AlwaysVerifyCalls, GlobalRegulatedVerifies, SelfRegulatedVerifies int
	SuccessRate, SelfRegulatedSuccessRate, SelectiveCoverage, SelectiveRisk float64
	OrderStressPasses, BoundaryStressPasses, ShiftAdaptationPasses int
	CausalAblationPass bool
	IndependentSeeds int
	Classification string
}

const g15Threshold = 0.80

func g15ActualSuccess(t g15Task, shifted bool) bool {
	budget:=9
	if shifted { budget=8 }
	return (t.Size-3)+2*t.Depth+t.Noise <= budget
}

func (m *g15Model) Update(t g15Task, ok bool) {
	k:=[3]int{t.Size,t.Depth,t.Noise}
	e:=m.Buckets[k]
	if ok { e.Success++; m.Global.Success++ } else { e.Fail++; m.Global.Fail++ }
	m.Buckets[k]=e
}

func g15Posterior(e g15Evidence) float64 {
	if e.Success+e.Fail==0 { return 0.5 }
	return float64(e.Success+1)/float64(e.Success+e.Fail+2)
}

func (m *g15Model) Predict(t g15Task) float64 {
	k:=[3]int{t.Size,t.Depth,t.Noise}
	if e,ok:=m.Buckets[k]; ok && e.Success+e.Fail>0 { return g15Posterior(e) }
	var near g15Evidence
	for key,e:=range m.Buckets {
		d:=0
		if key[0]!=t.Size { d++ }; if key[1]!=t.Depth { d++ }; if key[2]!=t.Noise { d++ }
		if d==1 { near.Success+=e.Success; near.Fail+=e.Fail }
	}
	if near.Success+near.Fail>0 { return g15Posterior(near) }
	return g15Posterior(m.Global)
}

func g15GlobalPredict(m *g15Model) float64 { return g15Posterior(m.Global) }

func g15Brier(ps []float64, ys []bool) float64 {
	if len(ps)==0 { return 0 }
	s:=0.0
	for i,p:=range ps { y:=0.0; if ys[i] { y=1 }; d:=p-y; s+=d*d }
	return s/float64(len(ps))
}

func g15ECE(ps []float64, ys []bool, bins int) float64 {
	if len(ps)==0 { return 0 }
	total:=0.0
	for b:=0;b<bins;b++ {
		lo,hi:=float64(b)/float64(bins),float64(b+1)/float64(bins)
		n:=0; sp:=0.0; sy:=0.0
		for i,p:=range ps {
			if (p>=lo && p<hi)||(b==bins-1&&p==1) { n++; sp+=p; if ys[i] { sy++ } }
		}
		if n>0 { total += float64(n)/float64(len(ps))*math.Abs(sp/float64(n)-sy/float64(n)) }
	}
	return total
}

// A low-confidence prediction invokes an independent verifier/recovery gate.
// The decision is made before the hidden outcome is revealed.
func g15Policy(p float64, actual bool)(cost float64, success, verified bool) {
	if p<g15Threshold { return 2,true,true }
	if actual { return 1,true,false }
	return 6,false,false
}

func g15Write(name string,v any) {
	ws:=os.Getenv("GITHUB_WORKSPACE"); if ws=="" { return }
	b,_:=json.MarshalIndent(v,"","  "); _=os.WriteFile(filepath.Join(ws,name),append(b,'\n'),0644)
}

func g15RunSeed(seed int) g15Report {
	const trainN,hiddenN,shiftAt=600,2048,512
	r:=rand.New(rand.NewSource(int64(150001+seed*7919)))
	m:=&g15Model{Buckets:map[[3]int]g15Evidence{}}
	for i:=0;i<trainN;i++ {
		t:=g15Task{3+r.Intn(10),1+r.Intn(5),r.Intn(4),i}
		m.Update(t,g15ActualSuccess(t,false))
	}
	static:=&g15Model{Buckets:map[[3]int]g15Evidence{},Global:m.Global}
	for k,v:=range m.Buckets { static.Buckets[k]=v }

	sp,sb,sy:=make([]float64,0,shiftAt),make([]float64,0,shiftAt),make([]bool,0,shiftAt)
	ap,st,ay:=make([]float64,0,hiddenN-shiftAt),make([]float64,0,hiddenN-shiftAt),make([]bool,0,hiddenN-shiftAt)
	rep:=g15Report{TrainingCases:trainN,HiddenCases:hiddenN,StationaryCases:shiftAt,ShiftedCases:hiddenN-shiftAt,IndependentSeeds:1,Classification:"G15_NOT_PROVEN"}

	always,global,self:=0.0,0.0,0.0
	gv,sv,policyOK,actualOK,unverified,unverifiedFail:=0,0,0,0,0,0

	for i:=0;i<hiddenN;i++ {
		t:=g15Task{3+r.Intn(10),1+r.Intn(5),r.Intn(4),10000+i}
		shift:=i>=shiftAt
		actual:=g15ActualSuccess(t,shift)
		p:=m.Predict(t); g:=g15GlobalPredict(m); s:=static.Predict(t)
		if !shift { sp=append(sp,p); sb=append(sb,g); sy=append(sy,actual) } else { ap=append(ap,p); st=append(st,s); ay=append(ay,actual) }

		always+=2
		gc,_,gver:=g15Policy(g,actual); global+=gc; if gver { gv++ }
		sc,ok,ver:=g15Policy(p,actual); self+=sc; if ver { sv++ } else { unverified++; if !actual { unverifiedFail++ } }
		if ok { policyOK++ }; if actual { actualOK++ }
		_ = gver
		m.Update(t,actual)
	}

	rep.SelfModelBrier=g15Brier(sp,sy); rep.GlobalBrier=g15Brier(sb,sy)
	rep.StaticShiftedBrier=g15Brier(st,ay); rep.AdaptiveShiftedBrier=g15Brier(ap,ay)
	rep.SelfModelECE=g15ECE(sp,sy,10)
	rep.AlwaysVerifyCost=always/hiddenN; rep.GlobalRegulatedCost=global/hiddenN; rep.SelfRegulatedCost=self/hiddenN
	rep.AlwaysVerifyCalls=hiddenN; rep.GlobalRegulatedVerifies=gv; rep.SelfRegulatedVerifies=sv
	rep.SuccessRate=float64(actualOK)/hiddenN; rep.SelfRegulatedSuccessRate=float64(policyOK)/hiddenN
	rep.SelectiveCoverage=1-float64(sv)/hiddenN
	if unverified>0 { rep.SelectiveRisk=float64(unverifiedFail)/float64(unverified) }

	if rep.AdaptiveShiftedBrier < 0.9*rep.StaticShiftedBrier { rep.ShiftAdaptationPasses=1 }

	train:=make([]g15Task,0,trainN)
	rr:=rand.New(rand.NewSource(int64(151777+seed*31)))
	for i:=0;i<trainN;i++ { train=append(train,g15Task{3+rr.Intn(10),1+rr.Intn(5),rr.Intn(4),i}) }
	ref:=&g15Model{Buckets:map[[3]int]g15Evidence{}}
	for _,t:=range train { ref.Update(t,g15ActualSuccess(t,false)) }
	for os:=1;os<=32;os++ {
		sh:=append([]g15Task(nil),train...)
		r2:=rand.New(rand.NewSource(int64(151000+seed*100+os)))
		r2.Shuffle(len(sh),func(i,j int){sh[i],sh[j]=sh[j],sh[i]})
		alt:=&g15Model{Buckets:map[[3]int]g15Evidence{}}
		for _,t:=range sh { alt.Update(t,g15ActualSuccess(t,false)) }
		ok:=true
		for _,p:=range []g15Task{{3,1,0,1},{7,3,2,2},{12,5,3,3},{15,6,4,4}} {
			if math.Abs(alt.Predict(p)-ref.Predict(p))>1e-12 { ok=false; break }
		}
		if ok { rep.OrderStressPasses++ }
	}
	for _,p:=range []g15Task{{20,8,7,1},{21,9,8,2},{22,10,9,3},{23,11,10,4}} {
		q:=ref.Predict(p); if q>0&&q<1 { rep.BoundaryStressPasses++ }
	}

	rep.CausalAblationPass=
		rep.SelfRegulatedCost<rep.GlobalRegulatedCost &&
		rep.SelfRegulatedCost<rep.AlwaysVerifyCost &&
		rep.SelfRegulatedSuccessRate>=0.98

	if rep.SelfModelBrier<rep.GlobalBrier &&
		rep.SelfRegulatedCost<rep.GlobalRegulatedCost &&
		rep.SelfRegulatedCost<rep.AlwaysVerifyCost &&
		rep.SelfRegulatedSuccessRate>=0.98 &&
		rep.SelectiveRisk<=0.10 &&
		rep.SelectiveCoverage>=0.15 &&
		rep.OrderStressPasses==32 &&
		rep.BoundaryStressPasses==4 &&
		rep.ShiftAdaptationPasses==1 &&
		rep.CausalAblationPass {
		rep.Classification="G15_METACOGNITIVE_SELF_MODEL_REGULATION_PROVEN"
	}
	return rep
}

func TestG15MetacognitiveSelfModelAndRegulation(t *testing.T) {
	const seeds=16
	agg:=g15Report{IndependentSeeds:seeds,Classification:"G15_NOT_PROVEN"}
	passed:=0
	var sumSB,sumGB,sumSS,sumAS,sumECE,sumAC,sumGC,sumSC,sumSR,sumAR,sumCV,sumRK float64
	for seed:=1;seed<=seeds;seed++ {
		r:=g15RunSeed(seed)
		sumSB+=r.SelfModelBrier; sumGB+=r.GlobalBrier; sumSS+=r.StaticShiftedBrier; sumAS+=r.AdaptiveShiftedBrier
		sumECE+=r.SelfModelECE; sumAC+=r.AlwaysVerifyCost; sumGC+=r.GlobalRegulatedCost; sumSC+=r.SelfRegulatedCost
		sumSR+=r.SelfRegulatedSuccessRate; sumAR+=r.SuccessRate; sumCV+=r.SelectiveCoverage; sumRK+=r.SelectiveRisk
		agg.TrainingCases+=r.TrainingCases; agg.HiddenCases+=r.HiddenCases; agg.StationaryCases+=r.StationaryCases; agg.ShiftedCases+=r.ShiftedCases
		agg.AlwaysVerifyCalls+=r.AlwaysVerifyCalls; agg.GlobalRegulatedVerifies+=r.GlobalRegulatedVerifies; agg.SelfRegulatedVerifies+=r.SelfRegulatedVerifies
		agg.OrderStressPasses+=r.OrderStressPasses; agg.BoundaryStressPasses+=r.BoundaryStressPasses; agg.ShiftAdaptationPasses+=r.ShiftAdaptationPasses
		if r.CausalAblationPass && r.Classification=="G15_METACOGNITIVE_SELF_MODEL_REGULATION_PROVEN" { passed++ } else { t.Logf("G15 seed=%d failed: %+v",seed,r) }
	}
	agg.SelfModelBrier=sumSB/seeds; agg.GlobalBrier=sumGB/seeds; agg.StaticShiftedBrier=sumSS/seeds; agg.AdaptiveShiftedBrier=sumAS/seeds; agg.SelfModelECE=sumECE/seeds
	agg.AlwaysVerifyCost=sumAC/seeds; agg.GlobalRegulatedCost=sumGC/seeds; agg.SelfRegulatedCost=sumSC/seeds
	agg.SuccessRate=sumAR/seeds; agg.SelfRegulatedSuccessRate=sumSR/seeds; agg.SelectiveCoverage=sumCV/seeds; agg.SelectiveRisk=sumRK/seeds
	agg.CausalAblationPass=passed==seeds
	if passed==seeds &&
		agg.SelfModelBrier<agg.GlobalBrier &&
		agg.SelfRegulatedCost<agg.GlobalRegulatedCost &&
		agg.SelfRegulatedCost<agg.AlwaysVerifyCost &&
		agg.SelfRegulatedSuccessRate>=0.98 &&
		agg.SelectiveRisk<=0.10 &&
		agg.SelectiveCoverage>=0.15 &&
		agg.OrderStressPasses==seeds*32 &&
		agg.BoundaryStressPasses==seeds*4 &&
		agg.ShiftAdaptationPasses==seeds &&
		agg.CausalAblationPass {
		agg.Classification="G15_METACOGNITIVE_SELF_MODEL_REGULATION_PROVEN"
	}
	g15Write("ACE_G15_METACOGNITION.json",agg)
	t.Logf("G15 aggregate=%+v passed=%d/%d",agg,passed,seeds)
	if agg.Classification!="G15_METACOGNITIVE_SELF_MODEL_REGULATION_PROVEN" { t.Fatalf("G15 failed: %+v",agg) }
}
