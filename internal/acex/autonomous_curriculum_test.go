package acex

import "testing"

func TestV2AutonomousCurriculum(t *testing.T) {
	c:=AutonomousCurriculum{}
	states:=[]CurriculumState{
		{KnownSuccesses:4,KnownFailures:3,TransferGaps:2},
		{KnownSuccesses:7,KnownFailures:1,ModelGaps:2},
		{KnownSuccesses:9,KnownFailures:2,SearchGaps:3},
	}
	for i,st:=range states {
		ch,err:=c.Generate(st,6,int64(100+i))
		if err!=nil { t.Fatal(err) }
		if ch.Parent!="measured-capability-frontier" || len(ch.Transforms)!=1 || ch.Difficulty<=1 {
			t.Fatalf("invalid endogenous challenge: %+v",ch)
		}
		base:=makeBalanced(int64(4000+i),families[i%len(families)],0,100,true)
		trans:=DatasetTransformer{}.Apply(base,ch.Transforms[0],int64(5000+i))
		if len(trans.Examples)!=len(base.Examples) { t.Fatal("curriculum changed episode count") }
	}
}
