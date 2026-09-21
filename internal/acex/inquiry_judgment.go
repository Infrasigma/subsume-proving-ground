package acex

import (
	"errors"
	"sort"
)

type InquiryCandidate struct {
	ID            string
	Question      string
	ExpectedGain  float64
	Novelty       float64
	Falsifiability float64
	Transfer      float64
	Cost          float64
	Feasibility   float64
}

type Critique struct {
	ID          string
	Target      string
	Claims      []string
	Weaknesses  []string
	CounterTests []string
	Severity    float64
	Provenance  string
}

type JudgmentMemoryItem struct {
	ID          string
	CandidateID string
	Selected    bool
	Reasons     []string
	Outcome     string
}

type InquiryManager struct {
	Judgments []JudgmentMemoryItem
	Critiques []Critique
}

func dominatesInquiry(a,b InquiryCandidate) bool {
	atLeast:=false
	for _,p:=range []struct{a,b float64}{
		{a.ExpectedGain,b.ExpectedGain},
		{a.Novelty,b.Novelty},
		{a.Falsifiability,b.Falsifiability},
		{a.Transfer,b.Transfer},
		{a.Feasibility,b.Feasibility},
	} {
		if p.a<p.b { return false }
		if p.a>p.b { atLeast=true }
	}
	if a.Cost>b.Cost { return false }
	if a.Cost<b.Cost { atLeast=true }
	return atLeast
}

// Select returns a Pareto-frontier inquiry, then breaks ties only by
// information gain per unit cost. No single hard-coded task score decides
// scientific direction.
func (m *InquiryManager) Select(candidates []InquiryCandidate, budget float64) (InquiryCandidate,error) {
	if len(candidates)==0 { return InquiryCandidate{},errors.New("no inquiry candidates") }
	front:=make([]InquiryCandidate,0)
	for _,c:=range candidates {
		if c.Cost>budget || c.Feasibility<=0 { continue }
		dominated:=false
		for _,o:=range candidates {
			if o.ID==c.ID || o.Cost>budget || o.Feasibility<=0 { continue }
			if dominatesInquiry(o,c) { dominated=true; break }
		}
		if !dominated { front=append(front,c) }
	}
	if len(front)==0 { return InquiryCandidate{},errors.New("no feasible nondominated inquiry") }
	sort.SliceStable(front,func(i,j int)bool{
		ri:=front[i].ExpectedGain/front[i].Cost
		rj:=front[j].ExpectedGain/front[j].Cost
		if ri!=rj { return ri>rj }
		return front[i].ID<front[j].ID
	})
	chosen:=front[0]
	m.Judgments=append(m.Judgments,JudgmentMemoryItem{
		ID:"judgment-"+chosen.ID,CandidateID:chosen.ID,Selected:true,
		Reasons:[]string{"nondominated","high-information-per-cost"},
	})
	return chosen,nil
}

func (m *InquiryManager) CritiqueCandidate(c InquiryCandidate, evidence []string, knownFailures []string) Critique {
	weak:=make([]string,0)
	tests:=make([]string,0)
	if c.Falsifiability<0.5 {
		weak=append(weak,"weak-falsifiability")
		tests=append(tests,"generate-discriminating-counterexample")
	}
	if c.Transfer<0.5 {
		weak=append(weak,"weak-transfer-basis")
		tests=append(tests,"rename-and-topology-shift")
	}
	if c.Feasibility<0.5 {
		weak=append(weak,"resource-or-execution-risk")
		tests=append(tests,"bounded-resource-replay")
	}
	if len(evidence)==0 {
		weak=append(weak,"no-independent-evidence")
		tests=append(tests,"independent-heldout-verification")
	}
	for _,f:=range knownFailures {
		if f!="" {
			weak=append(weak,"known-failure:"+f)
		}
	}
	sev:=float64(len(weak))/4.0
	if sev>1 { sev=1 }
	cr:=Critique{
		ID:"critique-"+c.ID,
		Target:c.ID,
		Claims:[]string{c.Question},
		Weaknesses:weak,
		CounterTests:tests,
		Severity:sev,
		Provenance:"inquiry-manager",
	}
	m.Critiques=append(m.Critiques,cr)
	return cr
}

func (m *InquiryManager) ReviseAfterCritique(candidate InquiryCandidate, critique Critique) InquiryCandidate {
	out:=candidate
	if critique.Severity>0 {
		out.Falsifiability+=0.20*float64(len(critique.CounterTests))
		if out.Falsifiability>1 { out.Falsifiability=1 }
		out.Transfer+=0.10
		if out.Transfer>1 { out.Transfer=1 }
		out.Cost+=float64(len(critique.CounterTests))
	}
	return out
}
