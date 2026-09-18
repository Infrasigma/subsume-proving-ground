package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/Infrasigma/subsume-proving-ground/internal/t2"
)

func main(){
	if len(os.Args)<2{fail("commands: preregister | preflight | calibrate | generate-audit | generate-live | run-f0 | audit-f0 | seal | finalize")}
	switch os.Args[1]{case "preregister":preregister(os.Args[2:]);case "preflight":preflight(os.Args[2:]);case "calibrate":calibrate(os.Args[2:]);case "generate-audit":generateAudit(os.Args[2:]);case "generate-live":generateLive(os.Args[2:]);case "run-f0":runF0(os.Args[2:]);case "audit-f0":auditF0(os.Args[2:]);case "seal":seal(os.Args[2:]);case "finalize":finalize(os.Args[2:]);default:fail("unknown command")}
}
func load(path string)(t2.Preregistration,string){p,h,err:=t2.LoadPreregistration(path);if err!=nil{fail(err.Error())};return p,h}

func preregister(args []string){
	fs:=flag.NewFlagSet("preregister",flag.ExitOnError);in:=fs.String("input","","draft prereg JSON");out:=fs.String("out","","locked prereg JSON");fs.Parse(args)
	if *in==""||*out==""{fail("preregister requires --input --out")}
	b,err:=os.ReadFile(*in);if err!=nil{fail(err.Error())}
	var p t2.Preregistration;if err:=json.Unmarshal(b,&p);err!=nil{fail(err.Error())}
	p,err=t2.SealPreregistration(p);if err!=nil{fail(err.Error())}
	j:=t2.MustJSON(p);if err:=os.WriteFile(*out,j,0600);err!=nil{fail(err.Error())};fmt.Println(t2.SHA256Bytes(j))
}

func runF0(args []string){
	fs:=flag.NewFlagSet("run-f0",flag.ExitOnError);prereg:=fs.String("prereg","","locked prereg JSON");scoredPath:=fs.String("scored","","D_audit_scored.jsonl");exe:=fs.String("executable","","immutable F0 executable");sha:=fs.String("sha256","","expected executable SHA256");study:=fs.String("study","","study ID");fs.Parse(args)
	if *prereg==""||*scoredPath==""||*exe==""||*sha==""{fail("run-f0 requires --prereg --scored --executable --sha256")}
	p,_:=load(*prereg);scored,err:=t2.ReadScoredJSONL(*scoredPath);if err!=nil{fail(err.Error())};public:=make([]t2.PublicTask,0,len(scored));for _,x:=range scored{public=append(public,x.Public)}
	if *study==""{*study=p.StudyID}
	ev,err:=t2.RunImmutableArm(context.Background(),t2.ImmutableArmConfig{Arm:t2.ArmF0,StudyID:*study,Executable:*exe,ExpectedSHA256:*sha,Tasks:public,Budget:t2.ResourceBudget{TokenBudget:1<<60,CPUTimeMS:1<<60,WallTimeMS:1<<60},CostModel:p.CostModel},scored);if err!=nil{fail(err.Error())}
	if err:=t2.AuditF0(t2.AuditScores{Scores:ev.TaskScores},p.Thresholds.F0MaxScore);err!=nil{fail(err.Error())}
	fmt.Println(string(t2.MustJSON(map[string]any{"f0_integrity_passed":true,"evidence":ev})))
}

func auditF0(args []string){
	fs:=flag.NewFlagSet("audit-f0",flag.ExitOnError);in:=fs.String("scores","","D_audit F0 score JSON");prereg:=fs.String("prereg","","locked prereg JSON");fs.Parse(args)
	if *in==""||*prereg==""{fail("audit-f0 requires --scores --prereg")}
	p,_:=load(*prereg);b,err:=os.ReadFile(*in);if err!=nil{fail(err.Error())};var s t2.AuditScores;if err:=json.Unmarshal(b,&s);err!=nil{fail(err.Error())}
	if err:=t2.AuditF0(s,p.Thresholds.F0MaxScore);err!=nil{fail(err.Error())};fmt.Println("{\"f0_integrity_passed\":true}")
}

func generateAudit(args []string){
	fs:=flag.NewFlagSet("generate-audit",flag.ExitOnError);prereg:=fs.String("prereg","","locked prereg JSON");out:=fs.String("out","","audit directory");seed:=fs.Int64("seed",20260918,"independent audit seed");fs.Parse(args)
	if *prereg==""||*out==""{fail("generate-audit requires --prereg --out")}
	p,_:=load(*prereg);plan:=t2.DefaultGeneratorPlan(*seed);plan.Novelty=p.Generator.Novelty;tasks,err:=t2.GenerateAuditPool(plan);if err!=nil{fail(err.Error())};if err:=t2.AuditGeneratedPool(tasks,p);err!=nil{fail(err.Error())};if err:=t2.WriteAuditPool(tasks,*out);err!=nil{fail(err.Error())};fmt.Println(t2.SHA256Bytes(t2.MustJSON(tasks)))
}

func generateLive(args []string){
	fs:=flag.NewFlagSet("generate-live",flag.ExitOnError);prereg:=fs.String("prereg","","locked prereg JSON");out:=fs.String("out","","dataset directory");seed:=fs.Int64("seed",20260918,"independent generation seed");fs.Parse(args)
	if *prereg==""||*out==""{fail("generate-live requires --prereg --out")}
	p,_:=load(*prereg);kms,err:=t2.NewRemoteKMSFromEnv();if err!=nil{fail(err.Error())}
	plan:=t2.DefaultGeneratorPlan(*seed);plan.Novelty=p.Generator.Novelty;plan.NonNovelPerFamily=8;plan.NovelPerFamily=8
	if len(p.Generator.GeneratorFamilies)!=len(plan.Generators){fail("preregistered generator family count does not match implementation")}
	m,key,err:=t2.GenerateAndSplit(plan,p,*out);if err!=nil{fail(err.Error())}
	if err:=kms.StoreValidateKey(p.StudyID,m.ValidateCipherHash,key);err!=nil{os.RemoveAll(*out);fail("KMS key registration failed; generated validation material was removed")}
	for i:=range key{key[i]=0}
	fmt.Printf("MANIFEST_HASH=%s\n",t2.SHA256Bytes(t2.MustJSON(m)))
}

func preflight(args []string){
	fs:=flag.NewFlagSet("preflight",flag.ExitOnError);prereg:=fs.String("prereg","","locked prereg JSON");f0:=fs.String("f0","","D_audit F0 scores JSON");generated:=fs.String("generated","","task JSONL");fs.Parse(args)
	if *prereg==""||*f0==""||*generated==""{fail("preflight requires --prereg --f0 --generated")}
	p,_:=load(*prereg);b,err:=os.ReadFile(*f0);if err!=nil{fail(err.Error())};var scores t2.AuditScores;if err:=json.Unmarshal(b,&scores);err!=nil{fail(err.Error())}
	if err:=t2.AuditF0(scores,p.Thresholds.F0MaxScore);err!=nil{fail(err.Error())}
	tasks,err:=t2.ReadJSONL(*generated);if err!=nil{fail(err.Error())};if err:=t2.AuditGenerator(tasks,p);err!=nil{fail(err.Error())}
	cal,err:=t2.SimulateCalibration(p);if err!=nil{fail(err.Error())}
	fmt.Println(string(t2.MustJSON(map[string]any{"prereg_hash":p.CriteriaHash,"f0_integrity_passed":true,"generator_integrity_passed":true,"calibration":cal})))
}
func calibrate(args []string){
	fs:=flag.NewFlagSet("calibrate",flag.ExitOnError);prereg:=fs.String("prereg","","locked prereg JSON");out:=fs.String("out","","result JSON");fs.Parse(args);if *prereg==""||*out==""{fail("calibrate requires --prereg --out")}
	p,h:=load(*prereg);r,err:=t2.SimulateCalibration(p);if err!=nil{fail(err.Error())};v:=map[string]any{"protocol":t2.ProtocolVersion,"prereg_hash":h,"result":r};b:=t2.MustJSON(v);if err:=os.WriteFile(*out,b,0600);err!=nil{fail(err.Error())};os.Stdout.Write(b);os.Stdout.Write([]byte("\\n"))
}
func seal(args []string){
	fs:=flag.NewFlagSet("seal",flag.ExitOnError)
	prereg:=fs.String("prereg","","locked prereg JSON")
	tasks:=fs.String("tasks","","task JSONL")
	out:=fs.String("out","","split dir")
	offline:=fs.Bool("offline-test",false,"emit test-only local data key; never use for production")
	fs.Parse(args)
	if *prereg==""||*tasks==""||*out==""{fail("seal requires --prereg --tasks --out")}
	p,_:=load(*prereg);ts,err:=t2.ReadJSONL(*tasks);if err!=nil{fail(err.Error())}
	m,key,err:=t2.SplitAndSeal(ts,p,*out);if err!=nil{fail(err.Error())}
	fmt.Printf("SEAL_MANIFEST_HASH=%s\n",t2.SHA256Bytes(t2.MustJSON(m)))
	if *offline {fmt.Printf("OFFLINE_TEST_KEY_BASE64=%s\n",base64.StdEncoding.EncodeToString(key))}
	if !*offline {fmt.Println("D_validate key intentionally withheld; production release must come from the remote KMS after Phase-4 lock attestation.")}
}
func finalize(args []string){
	fs:=flag.NewFlagSet("finalize",flag.ExitOnError)
	in:=fs.String("input","","finalizer input JSON")
	fs.Parse(args)
	if *in==""{fail("finalize requires --input")}
	b,err:=os.ReadFile(*in);if err!=nil{fail(err.Error())}
	var x struct{
		Prereg string
		Audits map[string]bool
		Estimands map[string]t2.EstimandInput
	}
	if err:=json.Unmarshal(b,&x);err!=nil{fail(err.Error())}
	if x.Prereg==""{fail("finalizer requires locked prereg path")}
	p,_:=load(x.Prereg)
	o,err:=t2.FinalizeFromPrereg(p,x.Audits,x.Estimands);if err!=nil{fail(err.Error())}
	os.Stdout.Write(t2.MustJSON(o));os.Stdout.Write([]byte("\n"))
}
func fail(s string){fmt.Fprintln(os.Stderr,s);os.Exit(1)}
