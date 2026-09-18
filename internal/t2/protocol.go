package t2

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	mrand "math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const ProtocolVersion = "t2-hostile-adjudication/v1"

type Thresholds struct { Alpha, PowerTarget, F0MaxScore, DeltaC, DeltaK, DeltaDelete, DeltaX float64 }
type GeneratorRules struct { GeneratorFamilies []string; MaxNGramJaccard float64; NGramSize int; MaxConstantLatentFrac float64; NovelStructureFeatures []string }
type StatisticalPlan struct { OuterSimulations, PermutationsPerSimulation, AnalysisPermutations, SampleSize int; NullStd, AltStd float64; Seed int64 }
type InterpolationPlan struct { Method string; CapabilityTarget float64; ExtrapolationAllowed bool }
type Preregistration struct {
	ProtocolVersion string
	Status string
	StudyID string
	CriteriaHash string
	Thresholds Thresholds
	Generator GeneratorRules
	Statistics StatisticalPlan
	Interpolation InterpolationPlan
	SelectionRule string
	ExecutionRules []string
}

func (p Preregistration) Validate() error {
	if p.ProtocolVersion != ProtocolVersion { return fmt.Errorf("protocol_version must be %q", ProtocolVersion) }
	if strings.ToUpper(p.Status) != "LOCKED" { return errors.New("preregistration status is not LOCKED") }
	if p.StudyID == "" { return errors.New("study_id is required") }
	t:=p.Thresholds
	if !(t.Alpha>0 && t.Alpha<=0.05) || !(t.PowerTarget>=0.80 && t.PowerTarget<1) { return errors.New("invalid alpha/power thresholds") }
	if t.F0MaxScore<0 || t.DeltaC<=0 || t.DeltaK<=0 || t.DeltaDelete<=0 || t.DeltaX<=0 { return errors.New("F0 threshold and all minimum effects must be concrete positive values") }
	g:=p.Generator
	if len(g.GeneratorFamilies)<2 || g.NGramSize<1 || g.MaxNGramJaccard<0 || g.MaxNGramJaccard>=1 || g.MaxConstantLatentFrac<0 || g.MaxConstantLatentFrac>1 || len(g.NovelStructureFeatures)==0 { return errors.New("invalid generator audit preregistration") }
	s:=p.Statistics
	if s.OuterSimulations!=10000 || s.PermutationsPerSimulation<100 || s.AnalysisPermutations<1000 || s.SampleSize<4 || s.NullStd<=0 || s.AltStd<=0 { return errors.New("invalid statistical preregistration") }
	if p.Interpolation.Method!="stepwise_linear" || p.Interpolation.ExtrapolationAllowed { return errors.New("invalid acquisition-cost interpolation rule") }
	if p.SelectionRule=="" || len(p.ExecutionRules)==0 { return errors.New("selection/execution rules are required") }
	if p.CriteriaHash=="" || p.CriteriaHash=="AUTO" { return errors.New("criteria_hash must be sealed") }
	return nil
}

func canonicalJSON(v any) []byte { b,_:=json.Marshal(v); return b }
func SHA256Bytes(b []byte) string { h:=sha256.Sum256(b); return hex.EncodeToString(h[:]) }

func LoadPreregistration(path string) (Preregistration,string,error) {
	b,err:=os.ReadFile(path); if err!=nil{return Preregistration{},"",err}
	var p Preregistration
	if err:=json.Unmarshal(b,&p);err!=nil{return p,"",err}
	got:=SHA256Bytes(canonicalJSON(p))
	if got!=p.CriteriaHash{return p,got,fmt.Errorf("criteria_hash mismatch: file=%s computed=%s",p.CriteriaHash,got)}
	if err:=p.Validate();err!=nil{return p,got,err}
	return p,got,nil
}

type Task struct { ID,Family,Prompt string; Latent map[string]string; Structure map[string]string }

func ReadJSONL(path string)([]Task,error){
	f,err:=os.Open(path);if err!=nil{return nil,err};defer f.Close()
	var out []Task
	s:=bufio.NewScanner(f)
	for s.Scan(){var t Task;if err:=json.Unmarshal(s.Bytes(),&t);err!=nil{return nil,fmt.Errorf("invalid JSONL: %w",err)};if t.ID==""||t.Family==""||t.Prompt==""{return nil,errors.New("task requires id/family/prompt")};out=append(out,t)}
	return out,s.Err()
}

func NGrams(s string,n int)map[string]struct{}{
	s=strings.ToLower(strings.Join(strings.Fields(s)," "));out:=map[string]struct{}{}
	for i:=0;i+n<=len(s);i++{out[s[i:i+n]]=struct{}{}}
	return out
}
func Jaccard(a,b map[string]struct{})float64{if len(a)==0&&len(b)==0{return 1};inter:=0;for x:=range a{if _,ok:=b[x];ok{inter++}};u:=len(a)+len(b)-inter;if u==0{return 0};return float64(inter)/float64(u)}

func AuditGenerator(tasks []Task,p Preregistration)error{
	allowed:=map[string]bool{};for _,f:=range p.Generator.GeneratorFamilies{allowed[f]=true}
	seen:=map[string]bool{};family:=map[string]bool{};latent:=map[string]map[string]int{}
	for _,t:=range tasks{
		if !allowed[t.Family]{return fmt.Errorf("generator integrity failed: unexpected family %q",t.Family)}
		if seen[t.ID]{return errors.New("generator integrity failed: duplicate task id")};seen[t.ID]=true;family[t.Family]=true
		for k,v:=range t.Latent{if latent[k]==nil{latent[k]=map[string]int{}};latent[k][v]++}
	}
	if len(family)!=len(allowed){return errors.New("generator integrity failed: missing generator family")}
	for k,c:=range latent{mx:=0;for _,n:=range c{if n>mx{mx=n}};if float64(mx)/float64(len(tasks))>p.Generator.MaxConstantLatentFrac{return fmt.Errorf("generator integrity failed: latent field %q is too constant",k)}}
	g:=make([]map[string]struct{},len(tasks));for i,t:=range tasks{g[i]=NGrams(t.Prompt,p.Generator.NGramSize)}
	for i:=0;i<len(g);i++{for j:=i+1;j<len(g);j++{if Jaccard(g[i],g[j])>p.Generator.MaxNGramJaccard{return fmt.Errorf("generator integrity failed: n-gram overlap between %s and %s exceeds threshold",tasks[i].ID,tasks[j].ID)}}}
	return nil
}

func Encrypt(plain,key []byte,aad string)([]byte,error){
	block,err:=aes.NewCipher(key);if err!=nil{return nil,err};gcm,err:=cipher.NewGCM(block);if err!=nil{return nil,err}
	nonce:=make([]byte,gcm.NonceSize());if _,err:=rand.Read(nonce);err!=nil{return nil,err}
	ct:=gcm.Seal(nil,nonce,plain,[]byte(aad));out:=append([]byte(aad+"\\n"),nonce...);out=append(out,ct...);return out,nil
}
func Decrypt(blob,key []byte,aad string)([]byte,error){
	p:=[]byte(aad+"\\n");if !bytes.HasPrefix(blob,p){return nil,errors.New("ciphertext aad mismatch")};blob=blob[len(p):]
	block,err:=aes.NewCipher(key);if err!=nil{return nil,err};gcm,err:=cipher.NewGCM(block);if err!=nil{return nil,err};if len(blob)<gcm.NonceSize(){return nil,errors.New("ciphertext truncated")}
	return gcm.Open(nil,blob[:gcm.NonceSize()],blob[gcm.NonceSize():],[]byte(aad))
}

type SealManifest struct { ProtocolVersion,PreregHash,DiscoverHash,SelectHash,ValidateHash,CiphertextHash string; FamilyManifest []string }

func encodeTasks(tasks []Task)[]byte{var b bytes.Buffer;e:=json.NewEncoder(&b);for _,t:=range tasks{_ = e.Encode(t)};return b.Bytes()}

func SplitAndSeal(tasks []Task,p Preregistration,outDir string)(SealManifest,[]byte,error){
	if err:=p.Validate();err!=nil{return SealManifest{},nil,err};if err:=AuditGenerator(tasks,p);err!=nil{return SealManifest{},nil,err};sort.Slice(tasks,func(i,j int)bool{return tasks[i].ID<tasks[j].ID})
	if len(tasks)<12{return SealManifest{},nil,errors.New("task pool too small")}
	a:=len(tasks)/4;b:=len(tasks)/2;c:=(3*len(tasks))/4
	d,s,v:=tasks[:a],tasks[a:b],tasks[c:]
	db,sb,vb:=encodeTasks(d),encodeTasks(s),encodeTasks(v)
	key:=make([]byte,32);if _,err:=rand.Read(key);err!=nil{return SealManifest{},nil,err}
	ct,err:=Encrypt(vb,key,p.StudyID+":D_validate");if err!=nil{return SealManifest{},nil,err}
	if err:=os.MkdirAll(outDir,0700);err!=nil{return SealManifest{},nil,err}
	for name,data:=range map[string][]byte{"D_discover.jsonl":db,"D_select.jsonl":sb,"D_validate.enc":ct}{if err:=os.WriteFile(filepath.Join(outDir,name),data,0600);err!=nil{return SealManifest{},nil,err}}
	m:=SealManifest{ProtocolVersion:ProtocolVersion,PreregHash:p.CriteriaHash,DiscoverHash:SHA256Bytes(db),SelectHash:SHA256Bytes(sb),ValidateHash:SHA256Bytes(vb),CiphertextHash:SHA256Bytes(ct),FamilyManifest:append([]string(nil),p.Generator.GeneratorFamilies...)}
	mb,_:=json.MarshalIndent(m,"","  ");if err:=os.WriteFile(filepath.Join(outDir,"SEAL_MANIFEST.json"),mb,0600);err!=nil{return SealManifest{},nil,err}
	return m,key,nil
}

type AuditScores struct { Scores []float64 }
func AuditF0(scores AuditScores,maxScore float64)error{
	if len(scores.Scores)==0{return errors.New("F0 integrity failed: no FM scores")}
	for i,s:=range scores.Scores{if math.IsNaN(s)||s<0{return errors.New("F0 integrity failed: invalid score")};if s>maxScore{return fmt.Errorf("FM-assisted ACE: score %d exceeds threshold",i)}}
	return nil
}

type LockProof struct { StudyID,PreregHash,FreshStateHash,ResourceHash string }
func(p LockProof)Hash()string{return SHA256Bytes(canonicalJSON(p))}
type KMSClient interface{ReleaseValidateKey(LockProof)([]byte,error)}
type LocalKMS struct{Key []byte}
func(k LocalKMS)ReleaseValidateKey(p LockProof)([]byte,error){if p.StudyID==""||p.PreregHash==""||p.FreshStateHash==""||p.ResourceHash==""{return nil,errors.New("invalid lock proof")};return append([]byte(nil),k.Key...),nil}

type Measurement struct { TaskID string; Capability,Cost,StructuralX,ResourceCost float64 }
type ArmResult struct { Arm string; Results []Measurement; StateHash string }

func ValidateArm(r ArmResult,ids []string,arm string)error{if r.Arm!=arm{return fmt.Errorf("unexpected arm %q",r.Arm)};if len(r.Results)!=len(ids){return fmt.Errorf("%s result count mismatch",arm)};seen:=map[string]bool{};for _,x:=range r.Results{if x.TaskID==""||seen[x.TaskID]{return fmt.Errorf("%s duplicate/empty task id",arm)};seen[x.TaskID]=true};for _,id:=range ids{if !seen[id]{return fmt.Errorf("%s missing task %s",arm,id)}};return nil}
func SignFlipPValue(a,b []float64,permutations int,seed int64)(float64,float64,error){if len(a)==0||len(a)!=len(b){return 0,0,errors.New("invalid paired sample")};d:=make([]float64,len(a));obs:=0.0;for i:=range a{d[i]=b[i]-a[i];obs+=d[i]};obs/=float64(len(d));r:=mrand.New(mrand.NewSource(seed));ext:=0;for i:=0;i<permutations;i++{m:=0.0;for _,x:=range d{if r.Intn(2)==0{m+=x}else{m-=x}};m/=float64(len(d));if math.Abs(m)>=math.Abs(obs){ext++}};return obs,float64(ext+1)/float64(permutations+1),nil}
func randNorm(r *mrand.Rand)float64{u1:=r.Float64();if u1<1e-12{u1=1e-12};u2:=r.Float64();return math.Sqrt(-2*math.Log(u1))*math.Cos(2*math.Pi*u2)}

func SimulateCalibration(p Preregistration)(map[string]float64,error){
	s:=p.Statistics;r:=mrand.New(mrand.NewSource(s.Seed));nullFP,altPass:=0,0
	for i:=0;i<s.OuterSimulations;i++{
		na,nb,aa,ab:=make([]float64,s.SampleSize),make([]float64,s.SampleSize),make([]float64,s.SampleSize),make([]float64,s.SampleSize)
		for j:=0;j<s.SampleSize;j++{na[j]=randNorm(r)*s.NullStd;nb[j]=randNorm(r)*s.NullStd;aa[j]=randNorm(r)*s.AltStd;ab[j]=aa[j]+p.Thresholds.DeltaC+randNorm(r)*s.AltStd*0.15}
		_,pn,_:=SignFlipPValue(na,nb,s.PermutationsPerSimulation,r.Int63());_,pa,_:=SignFlipPValue(aa,ab,s.PermutationsPerSimulation,r.Int63())
		if pn<=p.Thresholds.Alpha{nullFP++};if pa<=p.Thresholds.Alpha{altPass++}
	}
	fpr,pow:=float64(nullFP)/float64(s.OuterSimulations),float64(altPass)/float64(s.OuterSimulations)
	if fpr>p.Thresholds.Alpha||pow<p.Thresholds.PowerTarget{return map[string]float64{"false_positive_rate":fpr,"power":pow},errors.New("statistical calibration failed")}
	return map[string]float64{"false_positive_rate":fpr,"power":pow},nil
}

type EstimandInput struct { Estimand string; Value float64; PValue float64 }

func FinalizeFromPrereg(p Preregistration, audits map[string]bool, in map[string]EstimandInput)(map[string]any,error){
	required:=[]string{"null_simulation_calibrated","power_evaluation_passed","F0_integrity_passed","generator_integrity_passed"}
	for _,k:=range required{if !audits[k]{return nil,fmt.Errorf("pre-execution audit failed: %s",k)}}
	type rule struct{name,estimand string;effect float64;direction int}
	rules:=[]rule{
		{"causality_established","ΔC_delete",p.Thresholds.DeltaDelete,1},
		{"structural_transfer_established","Δ_X",p.Thresholds.DeltaX,1},
		{"capability_advantage_established","ΔC_MA",p.Thresholds.DeltaC,-1},
		{"cost_advantage_established","ΔK_MA",p.Thresholds.DeltaK,1},
	}
	out:=map[string]map[string]any{};all:=true
	for _,r:=range rules{
		x,ok:=in[r.name];if !ok{return nil,fmt.Errorf("missing estimand %s",r.name)}
		if x.Estimand!=r.estimand{return nil,fmt.Errorf("estimand label mismatch for %s",r.name)}
		if math.IsNaN(x.Value)||math.IsInf(x.Value,0)||math.IsNaN(x.PValue)||math.IsInf(x.PValue,0)||x.PValue<0||x.PValue>1{return nil,fmt.Errorf("invalid statistical result for %s",r.name)}
		passed:=x.PValue<=p.Thresholds.Alpha
		if r.direction>0{passed=passed&&x.Value>=r.effect}else{passed=passed&&x.Value<=-r.effect}
		out[r.name]=map[string]any{"estimand":r.estimand,"value":x.Value,"p_value":x.PValue,"passed_threshold":passed}
		all=all&&passed
	}
	verdict:="MECHANICAL_EVALUATION_PENDING";if all{verdict="T2_DEMONSTRATED"}
	return map[string]any{"PRE_EXECUTION_AUDITS":audits,"T2_CAUSAL_CONJUNCTION":out,"VERDICT":verdict},nil
}

type T2Conjunction struct { Causality,StructuralTransfer,CapabilityAdvantage,CostAdvantage bool }
func MechanicalOutput(audits map[string]bool,values map[string]map[string]any,c T2Conjunction)map[string]any{
	all:=c.Causality&&c.StructuralTransfer&&c.CapabilityAdvantage&&c.CostAdvantage;v:="MECHANICAL_EVALUATION_PENDING";if all{v="T2_DEMONSTRATED"}
	return map[string]any{"PRE_EXECUTION_AUDITS":audits,"T2_CAUSAL_CONJUNCTION":values,"VERDICT":v}
}
func MustJSON(v any)[]byte{b,_:=json.Marshal(v);return b}
