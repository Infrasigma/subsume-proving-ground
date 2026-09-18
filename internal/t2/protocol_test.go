package t2

import (
	"os"
	"path/filepath"
	"testing"
)

func testPrereg() Preregistration {
	p:=Preregistration{ProtocolVersion:ProtocolVersion,Status:"LOCKED",StudyID:"unit-test"}
	p.Thresholds=Thresholds{Alpha:0.05,PowerTarget:0.80,F0MaxScore:0.10,DeltaC:1,DeltaK:1,DeltaDelete:1,DeltaX:0.2}
	p.Generator=GeneratorRules{GeneratorFamilies:[]string{"g1","g2"},MaxNGramJaccard:0.90,NGramSize:5,MaxConstantLatentFrac:0.99,NovelStructureFeatures:[]string{"depth","transition"}}
	p.Statistics=StatisticalPlan{OuterSimulations:10000,PermutationsPerSimulation:100,AnalysisPermutations:1000,SampleSize:8,NullStd:1,AltStd:1,Seed:1}
	p.Interpolation=InterpolationPlan{Method:"stepwise_linear",ExtrapolationAllowed:false}
	p.SelectionRule="fixed";p.ExecutionRules=[]string{"identical validate tasks"}
	p.CriteriaHash=SHA256Bytes(canonicalJSON(p));return p
}

func TestSplitIsAuthenticated(t *testing.T){
	p:=testPrereg();tasks:=make([]Task,0,20)
	for i:=0;i<20;i++{f:="g1";if i%2==1{f="g2"};tasks=append(tasks,Task{ID:string(rune('a'+i)),Family:f,Prompt:"unique prompt "+string(rune('a'+i)),Latent:map[string]string{"seed":string(rune('A'+i))},Structure:map[string]string{"depth":"d"}})}
	d:=t.TempDir();m,key,err:=SplitAndSeal(tasks,p,d);if err!=nil{t.Fatal(err)}
	if m.CiphertextHash==m.ValidateHash{t.Fatal("ciphertext and plaintext hashes must differ")}
	ct,_:=os.ReadFile(filepath.Join(d,"D_validate.enc"))
	if _,err:=Decrypt(ct,key,p.StudyID+":D_validate");err!=nil{t.Fatal(err)}
	if _,err:=Decrypt(ct,make([]byte,32),p.StudyID+":D_validate");err==nil{t.Fatal("wrong key accepted")}
}

func TestMechanicalConjunction(t *testing.T){
	p:=testPrereg()
	a:=map[string]bool{"null_simulation_calibrated":true,"power_evaluation_passed":true,"F0_integrity_passed":true,"generator_integrity_passed":true}
	in:=map[string]EstimandInput{
		"causality_established":{Estimand:"ΔC_delete",Value:1.1,PValue:0.01},
		"structural_transfer_established":{Estimand:"Δ_X",Value:0.3,PValue:0.01},
		"capability_advantage_established":{Estimand:"ΔC_MA",Value:-1.1,PValue:0.01},
		"cost_advantage_established":{Estimand:"ΔK_MA",Value:1.1,PValue:0.01},
	}
	o,err:=FinalizeFromPrereg(p,a,in);if err!=nil{t.Fatal(err)}
	if o["VERDICT"]!="T2_DEMONSTRATED"{t.Fatalf("unexpected verdict %v",o["VERDICT"])}
}
