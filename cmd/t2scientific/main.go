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
	if len(os.Args)<2{fail("commands: preflight | calibrate | seal | finalize")}
	switch os.Args[1]{case "preflight":preflight(os.Args[2:]);case "calibrate":calibrate(os.Args[2:]);case "seal":seal(os.Args[2:]);case "finalize":finalize(os.Args[2:]);default:fail("unknown command")}
}
func load(path string)(t2.Preregistration,string){p,h,err:=t2.LoadPreregistration(path);if err!=nil{fail(err.Error())};return p,h}

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
	fs:=flag.NewFlagSet("finalize",flag.ExitOnError);in:=fs.String("input","","finalizer input JSON");fs.Parse(args);if *in==""{fail("finalize requires --input")}
	b,err:=os.ReadFile(*in);if err!=nil{fail(err.Error())}
	var x struct{Audits map[string]bool;Conjunction t2.T2Conjunction;Values map[string]map[string]any}
	if err:=json.Unmarshal(b,&x);err!=nil{fail(err.Error())};for k,v:=range x.Audits{if !v{fail("pre-execution audit failed: "+k)}}
	o:=t2.MechanicalOutput(x.Audits,x.Values,x.Conjunction);os.Stdout.Write(t2.MustJSON(o));os.Stdout.Write([]byte("\\n"))
}
func fail(s string){fmt.Fprintln(os.Stderr,s);os.Exit(1)}
