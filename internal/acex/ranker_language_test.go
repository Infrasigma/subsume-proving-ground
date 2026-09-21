//go:build v2kill

package acex

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

func makeRankerDataset(seed int64, n int) Dataset {
	r := rand.New(rand.NewSource(seed))
	latents := []string{"z-latent-0","z-latent-1","z-latent-2"}
	distractors := []string{
		"a-distractor-0","a-distractor-1","a-distractor-2",
		"a-distractor-3","a-distractor-4","a-distractor-5",
	}
	examples := make([]Observation,0,n)
	pos,neg := 0,0
	for pos<n/2 || neg<n/2 {
		label := pos<n/2 && (neg>=n/2 || r.Intn(2)==0)
		features := make([]string,0,12)
		if label {
			features=append(features,latents...)
		} else {
			missing:=neg%len(latents)
			for i,f:=range latents {
				if i!=missing { features=append(features,f) }
			}
		}
		for i,f:=range distractors {
			p := 0.80 - float64(i)*0.06
			if !label { p = 0.40 - float64(i)*0.03 }
			if p < 0.05 { p=0.05 }
			if r.Float64() < p { features=append(features,f) }
		}
		examples=append(examples,Observation{Features:features,Label:label})
		if label { pos++ } else { neg++ }
	}
	return Dataset{Examples:examples}
}

func TestV2SynthesizesNewSearchLanguage(t *testing.T) {
	train := make([]Dataset,4)
	hold := make([]Dataset,4)
	for i:=0;i<4;i++ {
		train[i]=makeRankerDataset(int64(500+i),120)
		hold[i]=makeRankerDataset(int64(700+i),120)
	}

	current := RankerProgram{Expr:&RankExpr{Kind:"metric",Value:metricID("balance")}}
	result,err:=SearchRankerProgram(train,hold,current)
	if err!=nil { t.Fatal(err) }
	if result.Improved >= result.Baseline {
		t.Fatalf("ranker did not improve visible acquisition cost: %+v",result)
	}
	if !strings.Contains(result.Program.Signature(),"add(") && !strings.Contains(result.Program.Signature(),"mul(") {
		t.Fatalf("learner retained a primitive rather than synthesizing a composite program: %s",result.Program.Signature())
	}
	if err:=EnsureFiniteRanker(result.Program); err!=nil { t.Fatal(err) }

	hiddenRatios:=make([]float64,0,8)
	for i:=0;i<8;i++ {
		tr:=makeRankerDataset(int64(900+i*11),140)
		ho:=makeRankerDataset(int64(1200+i*13),140)
		orderedBase:=candidateFeatures(tr)
		_,baseCost,baseErr:=DiscoverWithOrder(tr,ho,orderedBase,4)
		got,newCost,newErr:=DiscoverWithOrder(tr,ho,result.Program.OrderFixed(tr),4)
		if baseErr!=nil || newErr!=nil || got.Accuracy<0.90 {
			t.Fatalf("hidden transfer failed seed=%d baseErr=%v newErr=%v concept=%+v",i,baseErr,newErr,got)
		}
		ratio:=float64(newCost.Total())/float64(baseCost.Total())
		hiddenRatios=append(hiddenRatios,ratio)
		if ratio>=0.75 {
			t.Fatalf("composite search language did not generalize at seed=%d ratio=%.3f base=%d new=%d",i,ratio,baseCost.Total(),newCost.Total())
		}
	}
	t.Logf("V2 SYNTHESIZED SEARCH LANGUAGE program=%s visible=%d->%d hidden_ratios=%v",result.Program.Signature(),result.Baseline,result.Improved,hiddenRatios)

	// Ensure the learner cannot "win" by changing acceptance behavior: the
	// concept is independently checked on every hidden holdout.
	if strings.Contains(result.Program.Signature(),"task-family") {
		t.Fatal("ranker contains forbidden family-specific token")
	}
	fmt.Println("ACEX V2 NOVEL SEARCH LANGUAGE: PASS")
}
