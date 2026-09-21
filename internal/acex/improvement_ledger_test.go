package acex

import "testing"

func TestV2ImprovementLedger(t *testing.T) {
	l:=ImprovementLedger{}
	r:=MakeImprovementReceipt(0,1,"ranker:add(balance,frequency)",100,40,true)
	if err:=l.Admit(r); err!=nil { t.Fatal(err) }
	latest,ok:=l.Latest()
	if !ok || latest.ID!=r.ID || latest.RollbackID=="" {
		t.Fatalf("bad improvement ledger state: %+v %v",latest,ok)
	}
	bad:=MakeImprovementReceipt(1,2,"bad",40,50,true)
	if err:=l.Admit(bad); err==nil {
		t.Fatal("ledger accepted non-improvement")
	}
}
