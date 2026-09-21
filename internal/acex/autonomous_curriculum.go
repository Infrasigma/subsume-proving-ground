package acex

import (
	"fmt"
	"math/rand"
	"sort"
)

type ChallengeTransform string

const (
	TransformRename       ChallengeTransform = "rename"
	TransformPermute     ChallengeTransform = "permute"
	TransformDistract    ChallengeTransform = "distract"
	TransformCountertest ChallengeTransform = "countertest"
	TransformCompose     ChallengeTransform = "compose"
	TransformHorizon     ChallengeTransform = "horizon"
)

type ChallengeSpec struct {
	ID         string
	Parent     string
	Transforms []ChallengeTransform
	Difficulty float64
	Reason     string
}

type CurriculumState struct {
	KnownSuccesses int
	KnownFailures  int
	TransferGaps   int
	ModelGaps      int
	SearchGaps     int
}

type AutonomousCurriculum struct {
	Scheduler CognitiveScheduler
}

func (c *AutonomousCurriculum) Generate(state CurriculumState, budget float64, seed int64) (ChallengeSpec,error) {
	r:=rand.New(rand.NewSource(seed))
	candidates:=make([]CognitiveOpportunity,0,6)
	transforms:=[]ChallengeTransform{
		TransformRename,TransformPermute,TransformDistract,
		TransformCountertest,TransformCompose,TransformHorizon,
	}
	for i,tr:=range transforms {
		gain:=0.4+0.05*float64(state.KnownFailures)
		transfer:=0.4
		if tr==TransformRename || tr==TransformPermute { transfer+=0.4 }
		if tr==TransformCountertest { gain+=0.2 }
		if tr==TransformCompose { gain+=0.3 }
		if tr==TransformHorizon { gain+=0.2 }
		if state.TransferGaps>0 && (tr==TransformRename || tr==TransformPermute || tr==TransformDistract) {
			transfer+=0.3
		}
		if state.ModelGaps>0 && tr==TransformCountertest { gain+=0.3 }
		if state.SearchGaps>0 && tr==TransformCompose { gain+=0.3 }
		candidates=append(candidates,CognitiveOpportunity{
			ID:fmt.Sprintf("%d-%s",i,tr),
			Kind:DecisionExperiment,
			InformationGain:gain,
			CapabilityLift:gain,
			TransferValue:transfer,
			Novelty:0.5+r.Float64()*0.5,
			Risk:0.4,
			Cost:1+float64(i%3),
		})
	}
	chosen,err:=c.Scheduler.Choose(candidates,budget)
	if err!=nil { return ChallengeSpec{},err }
	var selected ChallengeTransform
	for _,tr:=range transforms {
		if chosen.ID[2:]==string(tr) || chosen.ID==fmt.Sprintf("%d-%s",0,tr) {
			selected=tr
			break
		}
	}
	if selected=="" {
		// Deterministic fallback from the selected opportunity ID.
		for _,tr:=range transforms {
			if fmt.Sprintf("%s",tr)==chosen.ID {
				selected=tr
				break
			}
		}
	}
	if selected=="" { return ChallengeSpec{},fmt.Errorf("selected transform unavailable: %s",chosen.ID) }
	return ChallengeSpec{
		ID:fmt.Sprintf("challenge-%d-%s",seed,selected),
		Parent:"measured-capability-frontier",
		Transforms:[]ChallengeTransform{selected},
		Difficulty:1+chosen.CapabilityLift+chosen.TransferValue,
		Reason:"selected from nondominated capability-gap opportunities",
	},nil
}

type DatasetTransformer struct{}

func (DatasetTransformer) Apply(in Dataset, transform ChallengeTransform, seed int64) Dataset {
	r:=rand.New(rand.NewSource(seed))
	out:=in.Copy()
	switch transform {
	case TransformRename:
		mapping:=map[string]string{}
		n:=0
		for _,ex:=range out.Examples {
			for _,f:=range ex.Features {
				if _,ok:=mapping[f]; !ok {
					n++
					mapping[f]=fmt.Sprintf("r-%d",n)
				}
			}
		}
		for i:=range out.Examples {
			for j,f:=range out.Examples[i].Features {
				out.Examples[i].Features[j]=mapping[f]
			}
		}
	case TransformPermute:
		for i:=range out.Examples {
			r.Shuffle(len(out.Examples[i].Features),func(a,b int){out.Examples[i].Features[a],out.Examples[i].Features[b]=out.Examples[i].Features[b],out.Examples[i].Features[a]})
		}
	case TransformDistract:
		for i:=range out.Examples {
			if r.Float64()<0.5 {
				out.Examples[i].Features=append(out.Examples[i].Features,fmt.Sprintf("noise-%d",r.Intn(20)))
			}
		}
	case TransformCountertest:
		if len(out.Examples)>2 {
			idx:=r.Intn(len(out.Examples))
			out.Examples[idx].Label=!out.Examples[idx].Label
		}
	case TransformCompose:
		for i:=range out.Examples {
			if i%2==0 {
				out.Examples[i].Features=append(out.Examples[i].Features,fmt.Sprintf("compose-%d",i%5))
			}
		}
	case TransformHorizon:
		for i:=0;i<len(out.Examples);i++ {
			out.Examples[i].Features=append(out.Examples[i].Features,fmt.Sprintf("horizon-%d",i%7))
		}
	}
	for i:=range out.Examples { sort.Strings(out.Examples[i].Features) }
	return out
}
