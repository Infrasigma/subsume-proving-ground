package acex

import (
	"errors"
	"time"
)

type ImprovementReceipt struct {
	ID            string
	ParentVersion uint64
	NewVersion    uint64
	MechanismID   string
	BeforeCost    int
	AfterCost     int
	HiddenPassed  bool
	Verifier      string
	RollbackID    string
	CreatedUTC    string
}

type ImprovementLedger struct {
	Receipts []ImprovementReceipt
}

func (l *ImprovementLedger) Admit(r ImprovementReceipt) error {
	if r.ID=="" || r.MechanismID=="" || r.ParentVersion!=r.NewVersion-1 {
		return errors.New("invalid improvement receipt")
	}
	if !r.HiddenPassed || r.AfterCost>=r.BeforeCost {
		return errors.New("improvement receipt does not establish verified gain")
	}
	l.Receipts=append(l.Receipts,r)
	return nil
}

func (l ImprovementLedger) Latest() (ImprovementReceipt,bool) {
	if len(l.Receipts)==0 { return ImprovementReceipt{},false }
	return l.Receipts[len(l.Receipts)-1],true
}

func MakeImprovementReceipt(parent,newVersion uint64,mechanism string,beforeCost,afterCost int,hidden bool) ImprovementReceipt {
	id:=Hash([]any{"improvement",parent,newVersion,mechanism,beforeCost,afterCost,hidden})
	return ImprovementReceipt{
		ID:id,
		ParentVersion:parent,
		NewVersion:newVersion,
		MechanismID:mechanism,
		BeforeCost:beforeCost,
		AfterCost:afterCost,
		HiddenPassed:hidden,
		Verifier:"independent-hidden-comparison",
		RollbackID:Hash([]any{"rollback",parent,mechanism}),
		CreatedUTC:time.Now().UTC().Format(time.RFC3339Nano),
	}
}
