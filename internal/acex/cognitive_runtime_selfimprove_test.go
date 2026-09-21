//go:build v2kill

package acex

import "testing"

func TestV2RuntimePromotesSynthesizedSearchLanguage(t *testing.T) {
	r:=NewCognitiveRuntime()
	train:=make([]Dataset,4)
	hold:=make([]Dataset,4)
	hidden:=make([]Dataset,4)
	for i:=0;i<4;i++ {
		train[i]=makeRankerDataset(int64(1500+i),120)
		hold[i]=makeRankerDataset(int64(1700+i),120)
		hidden[i]=makeRankerDataset(int64(1900+i),140)
	}
	before:=r.ActiveRanker.Signature()
	res,err:=r.ImproveSearchLanguage(train,hold,hidden)
	if err!=nil { t.Fatal(err) }
	if r.Version!=1 || r.ActiveRanker.Signature()==before {
		t.Fatalf("runtime failed to promote new search language: version=%d before=%s after=%s",r.Version,before,r.ActiveRanker.Signature())
	}
	if res.Improved>=res.Baseline {
		t.Fatalf("promoted mechanism did not improve cost: %+v",res)
	}
	receipt,ok:=r.ImprovementLedger.Latest()
	if !ok || !receipt.HiddenPassed || receipt.AfterCost!=res.Improved {
		t.Fatalf("promotion receipt missing or inconsistent: %+v",receipt)
	}
	active:=r.ActiveRanker.Signature()
	if err:=r.RollbackSearchLanguage(); err!=nil {
		t.Fatal(err)
	}
	if r.ActiveRanker.Signature()==active || r.Version!=0 {
		t.Fatalf("rollback did not restore parent mechanism: active=%s version=%d",r.ActiveRanker.Signature(),r.Version)
	}
}
