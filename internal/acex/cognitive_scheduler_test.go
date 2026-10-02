package acex

import "testing"

func TestV2CognitiveComputationScheduler(t *testing.T) {
	s:=CognitiveScheduler{}
	opps:=[]CognitiveOpportunity{
		{ID:"cheap-low",Kind:DecisionSearch,Uncertainty:.2,InformationGain:.2,CapabilityLift:.2,TransferValue:.1,Novelty:.1,Risk:.1,Cost:1},
		{ID:"dominated",Kind:DecisionSearch,Uncertainty:.3,InformationGain:.4,CapabilityLift:.3,TransferValue:.2,Novelty:.1,Risk:.2,Cost:4},
		{ID:"deep",Kind:DecisionExperiment,Uncertainty:.8,InformationGain:.9,CapabilityLift:.8,TransferValue:.9,Novelty:.8,Risk:.7,Cost:4},
	}
	chosen,err:=s.Choose(opps,4)
	if err!=nil { t.Fatal(err) }
	if chosen.ID!="deep" {
		t.Fatalf("scheduler chose dominated/low-value computation: %+v",chosen)
	}
	s.Record(chosen.ID,1)
	if len(s.History)!=1 || s.History[0].Outcome!=1 {
		t.Fatalf("scheduler failed to record outcome: %+v",s.History)
	}
}
