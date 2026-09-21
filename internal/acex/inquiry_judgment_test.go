package acex

import "testing"

func TestV2InquiryJudgmentAndCritiqueMemory(t *testing.T) {
	m:=InquiryManager{}
	candidates:=[]InquiryCandidate{
		{ID:"cheap-weak",Question:"reuse-only",ExpectedGain:.6,Novelty:.3,Falsifiability:.9,Transfer:.3,Cost:1,Feasibility:1},
		{ID:"broad",Question:"cross-surface causal test",ExpectedGain:.9,Novelty:.9,Falsifiability:.9,Transfer:.9,Cost:4,Feasibility:.9},
		{ID:"dominated",Question:"small benchmark",ExpectedGain:.7,Novelty:.5,Falsifiability:.7,Transfer:.5,Cost:5,Feasibility:1},
	}
	chosen,err:=m.Select(candidates,5)
	if err!=nil { t.Fatal(err) }
	if chosen.ID!="broad" {
		t.Fatalf("pareto inquiry selector chose dominated/low-value candidate: %+v",chosen)
	}

	cr:=m.CritiqueCandidate(chosen,nil,[]string{"prediction-mismatch"})
	if len(cr.Weaknesses)<2 || len(cr.CounterTests)<2 || cr.Severity<=0 {
		t.Fatalf("critique failed to surface missing evidence/transfer tests: %+v",cr)
	}
	revised:=m.ReviseAfterCritique(chosen,cr)
	if revised.Falsifiability<=chosen.Falsifiability || revised.Transfer<=chosen.Transfer {
		t.Fatalf("critique did not change inquiry design: before=%+v after=%+v",chosen,revised)
	}
	if len(m.Judgments)!=1 || len(m.Critiques)!=1 {
		t.Fatalf("judgment/critique provenance not retained: %+v %+v",m.Judgments,m.Critiques)
	}
}
