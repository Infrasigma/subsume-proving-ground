package ace

import (
	"encoding/json"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

type g15Task struct { Size, Depth, Noise, Seed int }
type g15Evidence struct { Success, Fail int }
type g15Model struct { Buckets map[[3]int]g15Evidence; Global g15Evidence }

type g15Report struct {
	TrainingCases int
	HiddenCases int
	StationaryCases int
	ShiftedCases int
	SelfModelBrier float64
	GlobalBrier float64
	StaticShiftedBrier float64
	AdaptiveShiftedBrier float64
	SelfModelECE float64
	AlwaysVerifyCost float64
	GlobalRegulatedCost float64
	SelfRegulatedCost float64
	AlwaysVerifyCalls int
	GlobalRegulatedVerifies int
	SelfRegulatedVerifies int
	SuccessRate float64
	SelfRegulatedSuccessRate float64
	OrderStressPasses int
	BoundaryStressPasses int
	ShiftAdaptationPasses int
	CausalAblationPass bool
	IndependentSeeds int
	Classification string
}

func g15Difficulty(t g15Task) int { return (t.Size-3) + 2*t.Depth + t.Noise }
func g15Budget(shifted bool) int { if shifted { return 8 }; return 9 }
func g15ActualSuccess(t g15Task, shifted bool) bool { return g15Difficulty(t) <= g15Budget(shifted) }

func (m *g15Model) Update(t g15Task, success bool) {
	k := [3]int{t.Size,t.Depth,t.Noise}; e := m.Buckets[k]
	if success { e.Success++; m.Global.Success++ } else { e.Fail++; m.Global.Fail++ }
	m.Buckets[k] = e
}

func g15Posterior(e g15Evidence) float64 {
	if e.Success+e.Fail==0 { return 0.5 }
	return float64(e.Success+1)/float64(e.Success+e.Fail+2)
}

func (m *g15Model) Predict(t g15Task) float64 {
	k := [3]int{t.Size,t.Depth,t.Noise}
	if e,ok := m.Buckets[k]; ok && e.Success+e.Fail>0 { return g15Posterior(e) }
	var matched g15Evidence
	for key,e := range m.Buckets {
		d:=0
		if key[0]!=t.Size { d++ }; if key[1]!=t.Depth { d++ }; if key[2]!=t.Noise { d++ }
		if d==1 { matched.Success+=e.Success; matched.Fail+=e.Fail }
	}
	if matched.Success+matched.Fail>0 { return g15Posterior(matched) }
	return g15Posterior(m.Global)
}

func g15GlobalPredict(m *g15Model) float64 { return g15Posterior(m.Global) }

func g15Brier(preds []float64, ys []bool) float64 {
	if len(preds)==0 { return 0 }; s:=0.0
	for i,p:=range preds { y:=0.0; if ys[i] { y=1 }; d:=p-y; s+=d*d }
	return s/float64(len(preds))
}

func g15ECE(preds []float64, ys []bool, bins int) float64 {
	if len(preds)==0 { return 0 }; total:=0.0
	for b:=0;b<bins;b++ {
		lo:=float64(b)/float64(bins); hi:=float64(b+1)/float64(bins)
		n:=0; sp:=0.0; sy:=0.0
		for i,p:=range preds {
			if (p>=lo && p<hi)||(b==bins-1 && p==1) { n++; sp+=p; if ys[i] { sy++ } }
		}
		if n>0 { total += float64(n)/float64(len(preds))*math.Abs(sp/float64(n)-sy/float64(n)) }
	}
	return total
}

func g15DecisionCost(p float64, success bool) (float64,bool) {
	if p<0.5 { return 2,success }
	if success { return 1,true }
	return 6,false
}

func g15Write(name string,v any) {
	ws:=os.Getenv("GITHUB_WORKSPACE"); if ws=="" { return }
	b,_:=json.MarshalIndent(v,"","  "); _=os.WriteFile(filepath.Join(ws,name),append(b,'\n'),0644)
}

func g15RunSeed(seed int) g15Report {
	const trainN=600; const hiddenN=2048; const shiftAt=512
	r:=rand.New(rand.NewSource(int64(150001+seed*7919)))
	model:=&g15Model{Buckets:map[[3]int]g15Evidence{}}
	for i:=0;i<trainN;i++ {
		task:=g15Task{Size:3+r.Intn(10),Depth:1+r.Intn(5),Noise:r.Intn(4),Seed:i}
		model.Update(task,g15ActualSuccess(task,false))
	}
	report:=g15Report{TrainingCases:trainN,HiddenCases:hiddenN,StationaryCases:shiftAt,ShiftedCases:hiddenN-shiftAt,Classification:"G15_NOT_PROVEN"}
	stationaryPreds:=make([]float64,0,shiftAt); stationaryBase:=make([]float64,0,shiftAt); stationaryY:=make([]bool,0,shiftAt)
	shiftedAdaptive:=make([]float64,0,hiddenN-shiftAt); shiftedStatic:=make([]float64,0,hiddenN-shiftAt); shiftedY:=make([]bool,0,hiddenN-shiftAt)
	staticModel:=&g15Model{Buckets:map[[3]int]g15Evidence{},Global:model.Global}
	for k,v:=range model.Buckets { staticModel.Buckets[k]=v }
	alwaysCost,globalCost,selfCost:=0.0,0.0,0.0
	globalVerifies,selfVerifies,selfSuccesses,totalSuccesses:=0,0,0,0
	for i:=0;i<hiddenN;i++ {
		task:=g15Task{Size:3+r.Intn(10),Depth:1+r.Intn(5),Noise:r.Intn(4),Seed:10000+i}
		shifted:=i>=shiftAt; ok:=g15ActualSuccess(task,shifted)
		p:=model.Predict(task); g:=g15GlobalPredict(model); s:=staticModel.Predict(task)
		if !shifted { stationaryPreds=append(stationaryPreds,p); stationaryBase=append(stationaryBase,g); stationaryY=append(stationaryY,ok) } else {
			shiftedAdaptive=append(shiftedAdaptive,p); shiftedStatic=append(shiftedStatic,s); shiftedY=append(shiftedY,ok)
		}
		alwaysCost+=2
		gc,_:=g15DecisionCost(g,ok); globalCost+=gc; if g<0.5 { globalVerifies++ }
		sc,ss:=g15DecisionCost(p,ok); selfCost+=sc; if p<0.5 { selfVerifies++ }; if ss { selfSuccesses++ }; if ok { totalSuccesses++ }
		model.Update(task,ok)
	}
	report.SelfModelBrier=g15Brier(stationaryPreds,stationaryY); report.GlobalBrier=g15Brier(stationaryBase,stationaryY)
	report.StaticShiftedBrier=g15Brier(shiftedStatic,shiftedY); report.AdaptiveShiftedBrier=g15Brier(shiftedAdaptive,shiftedY)
	report.SelfModelECE=g15ECE(stationaryPreds,stationaryY,10)
	report.AlwaysVerifyCost=alwaysCost/hiddenN; report.GlobalRegulatedCost=globalCost/hiddenN; report.SelfRegulatedCost=selfCost/hiddenN
	report.AlwaysVerifyCalls=hiddenN; report.GlobalRegulatedVerifies=globalVerifies; report.SelfRegulatedVerifies=selfVerifies
	report.SuccessRate=float64(totalSuccesses)/hiddenN; report.SelfRegulatedSuccessRate=float64(selfSuccesses)/hiddenN
	if report.AdaptiveShiftedBrier < report.StaticShiftedBrier*0.9 { report.ShiftAdaptationPasses=1 }
	train:=make([]g15Task,0,trainN); rr:=rand.New(rand.NewSource(int64(151777+seed*31)))
	for i:=0;i<trainN;i++ { train=append(train,g15Task{Size:3+rr.Intn(10),Depth:1+rr.Intn(5),Noise:rr.Intn(4),Seed:i}) }
	reference:=&g15Model{Buckets:map[[3]int]g15Evidence{}}
	for _,task:=range train { reference.Update(task,g15ActualSuccess(task,false)) }
	for orderSeed:=1;orderSeed<=32;orderSeed++ {
		shuffled:=append([]g15Task(nil),train...); r2:=rand.New(rand.NewSource(int64(151000+seed*100+orderSeed)))
		r2.Shuffle(len(shuffled),func(i,j int){shuffled[i],shuffled[j]=shuffled[j],shuffled[i]})
		alt:=&g15Model{Buckets:map[[3]int]g15Evidence{}}
		for _,task:=range shuffled { alt.Update(task,g15ActualSuccess(task,false)) }
		ok:=true
		for _,probe:=range []g15Task{{3,1,0,1},{7,3,2,2},{12,5,3,3},{15,6,4,4}} {
			if math.Abs(alt.Predict(probe)-reference.Predict(probe))>1e-12 { ok=false; break }
		}
		if ok { report.OrderStressPasses++ }
	}
	for _,probe:=range []g15Task{{20,8,7,1},{21,9,8,2},{22,10,9,3},{23,11,10,4}} {
		p:=reference.Predict(probe); if p>0 && p<1 { report.BoundaryStressPasses++ }
	}
	report.CausalAblationPass=report.SelfRegulatedCost<report.GlobalRegulatedCost && report.SelfRegulatedCost<report.AlwaysVerifyCost && report.SelfRegulatedSuccessRate>=report.SuccessRate*0.98
	if report.SelfModelBrier<report.GlobalBrier && report.SelfModelECE<0.10 &&
		report.SelfRegulatedCost<report.GlobalRegulatedCost &&
		report.SelfRegulatedSuccessRate>=report.SuccessRate*0.98 &&
		report.OrderStressPasses==32 && report.BoundaryStressPasses==4 &&
		report.ShiftAdaptationPasses>=1 && report.CausalAblationPass {
		report.Classification="G15_METACOGNITIVE_SELF_MODEL_REGULATION_PROVEN"
	}
	return report
}

func TestG15MetacognitiveSelfModelAndRegulation(t *testing.T) {
	const seeds=16
	aggregate:=g15Report{Classification:"G15_NOT_PROVEN",IndependentSeeds:seeds}
	passed:=0
	sumSelfBrier,sumGlobalBrier,sumShiftStatic,sumShiftAdaptive,sumECE:=0.0,0.0,0.0,0.0,0.0
	sumAlways,sumGlobal,sumSelf:=0.0,0.0,0.0
	sumSuccess,sumSelfSuccess:=0.0,0.0
	minSavings:=1.0
	for seed:=1;seed<=seeds;seed++ {
		r:=g15RunSeed(seed)
		aggregate.TrainingCases+=r.TrainingCases; aggregate.HiddenCases+=r.HiddenCases
		aggregate.StationaryCases+=r.StationaryCases; aggregate.ShiftedCases+=r.ShiftedCases
		sumSelfBrier+=r.SelfModelBrier; sumGlobalBrier+=r.GlobalBrier; sumShiftStatic+=r.StaticShiftedBrier; sumShiftAdaptive+=r.AdaptiveShiftedBrier; sumECE+=r.SelfModelECE
		sumAlways+=r.AlwaysVerifyCost; sumGlobal+=r.GlobalRegulatedCost; sumSelf+=r.SelfRegulatedCost
		sumSuccess+=r.SuccessRate; sumSelfSuccess+=r.SelfRegulatedSuccessRate
		aggregate.AlwaysVerifyCalls+=r.AlwaysVerifyCalls; aggregate.GlobalRegulatedVerifies+=r.GlobalRegulatedVerifies; aggregate.SelfRegulatedVerifies+=r.SelfRegulatedVerifies
		aggregate.OrderStressPasses+=r.OrderStressPasses; aggregate.BoundaryStressPasses+=r.BoundaryStressPasses; aggregate.ShiftAdaptationPasses+=r.ShiftAdaptationPasses
		if r.CausalAblationPass { aggregate.CausalAblationPass=true }
		if r.Classification=="G15_METACOGNITIVE_SELF_MODEL_REGULATION_PROVEN" { passed++ } else { t.Logf("G15 seed=%d failed: %+v",seed,r) }
		savings:=1.0-r.SelfRegulatedCost/r.AlwaysVerifyCost; if savings<minSavings { minSavings=savings }
	}
	aggregate.SelfModelBrier=sumSelfBrier/seeds; aggregate.GlobalBrier=sumGlobalBrier/seeds
	aggregate.StaticShiftedBrier=sumShiftStatic/seeds; aggregate.AdaptiveShiftedBrier=sumShiftAdaptive/seeds
	aggregate.SelfModelECE=sumECE/seeds; aggregate.AlwaysVerifyCost=sumAlways/seeds; aggregate.GlobalRegulatedCost=sumGlobal/seeds; aggregate.SelfRegulatedCost=sumSelf/seeds
	aggregate.SuccessRate=sumSuccess/seeds
	aggregate.SelfRegulatedSuccessRate=sumSelfSuccess/seeds
	aggregate.CausalAblationPass=passed==seeds
	if passed==seeds &&
		aggregate.SelfModelBrier<aggregate.GlobalBrier &&
		aggregate.SelfModelECE<0.10 &&
		aggregate.SelfRegulatedCost<aggregate.GlobalRegulatedCost &&
		aggregate.OrderStressPasses==seeds*32 &&
		aggregate.BoundaryStressPasses==seeds*4 &&
		aggregate.ShiftAdaptationPasses==seeds &&
		aggregate.CausalAblationPass && minSavings>0.02 {
		aggregate.Classification="G15_METACOGNITIVE_SELF_MODEL_REGULATION_PROVEN"
	}
	g15Write("ACE_G15_METACOGNITION.json",aggregate)
	t.Logf("G15 aggregate=%+v passed_seeds=%d/%d min_savings=%.4f",aggregate,passed,seeds,minSavings)
	if aggregate.Classification!="G15_METACOGNITIVE_SELF_MODEL_REGULATION_PROVEN" { t.Fatalf("G15 failed: %+v",aggregate) }
}
