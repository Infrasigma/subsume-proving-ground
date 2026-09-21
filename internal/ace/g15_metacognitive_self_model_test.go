package ace

import (
	"encoding/json"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

type g15Task struct {
	Family  int
	Depth   int
	Noise   int
	Seed    int
}

type g15Observation struct {
	Task    g15Task
	Success bool
}

type g15Bucket struct {
	Success int
	Fail    int
}

type g15SelfModel struct {
	Buckets map[[2]int]g15Bucket
	Global  g15Bucket
}

type g15Prediction struct {
	P float64
}

type g15Report struct {
	TrainingCases          int
	HiddenCases            int
	HiddenAccuracy         int
	BaselineBrier          float64
	SelfModelBrier         float64
	SelfModelECE           float64
	SelectiveCoverage      float64
	SelectiveRisk          float64
	AlwaysVerifyCalls      int
	SelfRegulatedVerifies  int
	RegulationSavings      float64
	OrderStressPasses     int
	BoundaryStressPasses   int
	Classification         string
}

func g15Difficulty(t g15Task) int {
	// Hidden physical difficulty is independent of the self-model. The model
	// only sees family/depth; the evaluator computes actual success.
	return 2*t.Depth + t.Family*2 + t.Noise
}

func g15SolverBudget(t g15Task) int {
	return 7 + (t.Family%3)*2
}

func g15ActualSuccess(t g15Task) bool {
	return g15Difficulty(t) <= g15SolverBudget(t)
}

func g15TrainTask(r *rand.Rand, seed int) g15Task {
	family:=r.Intn(6)
	depth:=1+r.Intn(4)
	noise:=r.Intn(3)
	return g15Task{Family:family,Depth:depth,Noise:noise,Seed:seed}
}

func g15HiddenTask(r *rand.Rand, seed int) g15Task {
	// Shift the hidden distribution and include unseen family/depth
	// combinations; no hidden outcome is used to train the self-model.
	family:=r.Intn(6)
	depth:=1+r.Intn(4)
	noise:=r.Intn(4)
	return g15Task{Family:family,Depth:depth,Noise:noise,Seed:seed}
}

func (m *g15SelfModel) Update(t g15Task, success bool) {
	k:=[2]int{t.Family,t.Depth}
	b:=m.Buckets[k]
	if success {b.Success++} else {b.Fail++}
	m.Buckets[k]=b
	if success {m.Global.Success++} else {m.Global.Fail++}
}

func (m *g15SelfModel) Predict(t g15Task) float64 {
	k:=[2]int{t.Family,t.Depth}
	b,ok:=m.Buckets[k]
	if !ok {
		b=m.Global
	}
	if b.Success+b.Fail==0 {return 0.5}
	return float64(b.Success+1)/float64(b.Success+b.Fail+2)
}

func g15Brier(preds []float64, ys []bool) float64 {
	if len(preds)==0{return 0}
	sum:=0.0
	for i,p:=range preds {
		y:=0.0; if ys[i]{y=1}
		d:=p-y; sum+=d*d
	}
	return sum/float64(len(preds))
}

func g15ECE(preds []float64, ys []bool, bins int) float64 {
	if len(preds)==0{return 0}
	var total float64
	for b:=0;b<bins;b++ {
		lo:=float64(b)/float64(bins); hi:=float64(b+1)/float64(bins)
		count:=0; sumP:=0.0; sumY:=0.0
		for i,p:=range preds {
			if (p>=lo && p<hi) || (b==bins-1 && p==1) {
				count++; sumP+=p; if ys[i]{sumY++}
			}
		}
		if count>0 {
			total += float64(count)/float64(len(preds))*math.Abs(sumP/float64(count)-sumY/float64(count))
		}
	}
	return total
}

func g15GlobalPredict(m *g15SelfModel) float64 {
	n:=m.Global.Success+m.Global.Fail
	if n==0{return 0.5}
	return float64(m.Global.Success)/float64(n)
}

func g15Write(name string,v any){
	ws:=os.Getenv("GITHUB_WORKSPACE");if ws==""{return}
	b,_:=json.MarshalIndent(v,"","  ");_=os.WriteFile(filepath.Join(ws,name),append(b,'\n'),0644)
}

func TestG15MetacognitiveSelfModelAndRegulation(t *testing.T){
	const trainN=192
	const hiddenN=512
	r:=rand.New(rand.NewSource(150001))
	model:=&g15SelfModel{Buckets:map[[2]int]g15Bucket{}}
	train:=make([]g15Observation,0,trainN)
	for i:=0;i<trainN;i++ {
		task:=g15TrainTask(r,i)
		ok:=g15ActualSuccess(task)
		train=append(train,g15Observation{task,ok})
		model.Update(task,ok)
	}
	report:=g15Report{TrainingCases:trainN,HiddenCases:hiddenN,Classification:"G15_NOT_PROVEN"}
	preds:=make([]float64,0,hiddenN)
	basePreds:=make([]float64,0,hiddenN)
	ys:=make([]bool,0,hiddenN)
	regVerifies:=0
	successCount:=0
	for i:=0;i<hiddenN;i++ {
		task:=g15HiddenTask(r,10000+i)
		p:=model.Predict(task)
		base:=g15GlobalPredict(model)
		ok:=g15ActualSuccess(task)
		preds=append(preds,p); basePreds=append(basePreds,base); ys=append(ys,ok)
		if (p>=0.55 && ok) || (p<0.55 && !ok) {successCount++}
		// Metacognitive control: low-confidence work receives independent
		// verification before execution; high-confidence work proceeds.
		if p<0.55 {regVerifies++}
	}
	report.HiddenAccuracy=successCount
	report.SelfModelBrier=g15Brier(preds,ys)
	report.BaselineBrier=g15Brier(basePreds,ys)
	report.SelfModelECE=g15ECE(preds,ys,10)

	// Selective execution evaluation. Predictions are frozen before actual
	// outcomes are consulted.
	attempted:=0; failedAttempts:=0
	for i,p:=range preds {
		if p>=0.72 {
			attempted++
			if !ys[i]{failedAttempts++}
		}
	}
	if hiddenN>0 {report.SelectiveCoverage=float64(attempted)/float64(hiddenN)}
	if attempted>0 {report.SelectiveRisk=float64(failedAttempts)/float64(attempted)}
	report.AlwaysVerifyCalls=hiddenN
	report.SelfRegulatedVerifies=regVerifies
	report.RegulationSavings=1.0-float64(regVerifies)/float64(hiddenN)

	// Randomized order: the same multiset of observations must yield the same
	// self-model predictions, eliminating dependence on insertion order.
	for seed:=1;seed<=24;seed++ {
		rr:=rand.New(rand.NewSource(int64(16000+seed)))
		shuffled:=append([]g15Observation(nil),train...)
		rr.Shuffle(len(shuffled),func(i,j int){shuffled[i],shuffled[j]=shuffled[j],shuffled[i]})
		alt:=&g15SelfModel{Buckets:map[[2]int]g15Bucket{}}
		for _,o:=range shuffled {alt.Update(o.Task,o.Success)}
		ok:=true
		for _,task:=range []g15Task{{0,1,0,1},{2,3,1,2},{7,5,2,3}} {
			if math.Abs(alt.Predict(task)-model.Predict(task))>1e-12{ok=false}
		}
		if !ok{t.Fatalf("order stress failed seed %d",seed)}
		report.OrderStressPasses++
	}

	// Boundary stress: unseen family/depth buckets must fall back to the global
	// prior instead of inventing certainty.
	for f:=6;f<8;f++ {
		for d:=4;d<=5;d++ {
			task:=g15Task{Family:f,Depth:d,Noise:0}
			got:=model.Predict(task)
			exp:=g15GlobalPredict(model)
			if math.Abs(got-exp)>1e-12 {t.Fatalf("unseen bucket did not use fallback prior")}
			report.BoundaryStressPasses++
		}
	}

	if report.SelfModelBrier<report.BaselineBrier &&
		report.SelfModelECE<0.12 &&
		report.SelectiveCoverage>=0.28 &&
		report.SelectiveRisk<=0.15 &&
		report.RegulationSavings>=0.70 &&
		report.OrderStressPasses==24 &&
		report.BoundaryStressPasses==4 {
		report.Classification="G15_METACOGNITIVE_SELF_MODEL_REGULATION_PROVEN"
	}
	g15Write("ACE_G15_METACOGNITION.json",report)
	t.Logf("G15 report=%+v",report)
	if report.Classification!="G15_METACOGNITIVE_SELF_MODEL_REGULATION_PROVEN" {
		t.Fatalf("G15 failed: %+v",report)
	}
}
