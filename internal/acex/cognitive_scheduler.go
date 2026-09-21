package acex

import (
	"errors"
	"sort"
)

type CognitiveOpportunity struct {
	ID             string
	Kind           ExecutiveDecision
	Uncertainty    float64
	InformationGain float64
	CapabilityLift float64
	TransferValue  float64
	Novelty        float64
	Risk           float64
	Cost           float64
}

type OpportunityRecord struct {
	ID        string
	Chosen    bool
	Outcome   float64
	Cost      float64
}

type CognitiveScheduler struct {
	History []OpportunityRecord
}

func dominatesOpportunity(a,b CognitiveOpportunity) bool {
	atLeast:=false
	for _,x:=range []struct{a,b float64}{
		{a.InformationGain,b.InformationGain},
		{a.CapabilityLift,b.CapabilityLift},
		{a.TransferValue,b.TransferValue},
		{a.Novelty,b.Novelty},
		{a.Risk,b.Risk},
	} {
		if x.a < x.b { return false }
		if x.a > x.b { atLeast=true }
	}
	if a.Cost>b.Cost { return false }
	if a.Cost<b.Cost { atLeast=true }
	return atLeast
}

func (s *CognitiveScheduler) Choose(candidates []CognitiveOpportunity, budget float64) (CognitiveOpportunity,error) {
	if len(candidates)==0 { return CognitiveOpportunity{},errors.New("no cognitive opportunities") }
	front:=make([]CognitiveOpportunity,0)
	for _,c:=range candidates {
		if c.Cost<=0 || c.Cost>budget { continue }
		dominated:=false
		for _,o:=range candidates {
			if o.ID==c.ID || o.Cost<=0 || o.Cost>budget { continue }
			if dominatesOpportunity(o,c) { dominated=true; break }
		}
		if !dominated { front=append(front,c) }
	}
	if len(front)==0 { return CognitiveOpportunity{},errors.New("no feasible nondominated opportunity") }
	sort.SliceStable(front,func(i,j int)bool{
		ri:=(front[i].InformationGain+front[i].CapabilityLift+front[i].TransferValue+front[i].Novelty+front[i].Risk)/front[i].Cost
		rj:=(front[j].InformationGain+front[j].CapabilityLift+front[j].TransferValue+front[j].Novelty+front[j].Risk)/front[j].Cost
		if ri!=rj { return ri>rj }
		return front[i].ID<front[j].ID
	})
	chosen:=front[0]
	s.History=append(s.History,OpportunityRecord{ID:chosen.ID,Chosen:true,Cost:chosen.Cost})
	return chosen,nil
}

func (s *CognitiveScheduler) Record(id string,outcome float64) {
	for i:=len(s.History)-1;i>=0;i-- {
		if s.History[i].ID==id && s.History[i].Chosen {
			s.History[i].Outcome=outcome
			return
		}
	}
}
