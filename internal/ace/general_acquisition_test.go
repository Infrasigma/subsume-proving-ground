package ace

import "testing"

type absOracle struct{}
func (absOracle) Evaluate(_ Task, in map[string]string) (map[string]string,error) { x:=0; if in["x"]!="" { for _,r:=range in["x"] { _=r } }; _,_=x,in; n,err:=parseInt(in["x"]); if err!=nil{return nil,err}; if n<0{n=-n}; return map[string]string{"y":itoa(n)},nil }

func parseInt(s string)(int,error){ var n int; sign:=1; for i,r:=range s { if i==0&&r=='-' {sign=-1;continue}; if r<'0'||r>'9' { return 0,errBadInt{} }; n=n*10+int(r-'0') }; return sign*n,nil }
type errBadInt struct{}
func (errBadInt) Error() string{return "invalid integer"}
func itoa(n int)string{ if n==0{return "0"}; sign:="";if n<0{sign="-";n=-n};b:=make([]byte,0,12);for n>0{b=append(b,byte('0'+n%10));n/=10};for i,j:=0,len(b)-1;i<j;i,j=i+1,j-1{b[i],b[j]=b[j],b[i]};return sign+string(b)}

func TestGeneralAcquisitionUsesIndependentCounterexamples(t *testing.T){
	task:=Task{ID:"general-abs",Goal:"absolute magnitude",Budget:ResourceVector{TimeMS:10000}}
	spec,err:=GeneralCapabilitySpecification(task,[]ProgramTestCase{{Input:map[string]string{"x":"-3"},Expected:map[string]string{"y":"3"}},{Input:map[string]string{"x":"4"},Expected:map[string]string{"y":"4"}}});if err!=nil{t.Fatal(err)}
	e:=GeneralAcquisitionEngine{Counterexamples:IndependentCounterexampleGenerator{},Oracle:absOracle{}}
	result,err:=e.Acquire(task,spec,[]map[string]string{{"x":"-3"},{"x":"4"}});if err!=nil{t.Fatal(err)}
	if result.Verification.Status!="verified"||!result.Verification.Independent{t.Fatalf("unverified result: %+v",result.Verification)}
	if len(result.Counterexamples)<4{t.Fatalf("expected independent boundary cases, got %d",len(result.Counterexamples))}
	if len(e.Library.Records)!=1{t.Fatalf("verified capability was not retained")}
}
