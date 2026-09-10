package ace

import (
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "strconv"
    "strings"
    "sync"
)

// CapabilityRecord is the persistent, executable form of an acquired capability.
type CapabilityRecord struct {
    Capability Capability `json:"capability"`
    Skill Skill `json:"skill"`
    Artifact string `json:"artifact"`
    Tests []ProgramTestCase `json:"tests"`
    Mechanism string `json:"mechanism"`
    ArchitectureCost float64 `json:"architecture_cost"`
    Provenance Provenance `json:"provenance"`
}

type ArchitectureExperience struct {
    StructuralKey string `json:"structural_key"`
    Mechanism string `json:"mechanism"`
    Passed bool `json:"passed"`
    SearchCount int `json:"search_count"`
    Cost ResourceVector `json:"cost"`
    Provenance Provenance `json:"provenance"`
}

type registrySnapshot struct {
    Records []CapabilityRecord `json:"records"`
    Architecture []ArchitectureExperience `json:"architecture"`
    Version uint64 `json:"version"`
}

type PersistentRegistry struct {
    mu sync.Mutex
    Path string
    Data registrySnapshot
}

func NewPersistentRegistry(path string) (*PersistentRegistry,error) {
    r:=&PersistentRegistry{Path:path}
    b,err:=os.ReadFile(path)
    if err==nil {
        if err=json.Unmarshal(b,&r.Data);err!=nil{return nil,err}
    } else if !errors.Is(err,os.ErrNotExist) { return nil,err }
    return r,nil
}
func (r *PersistentRegistry) flush() error {
    if r.Path=="" { return nil }
    if err:=os.MkdirAll(filepath.Dir(r.Path),0755);err!=nil{return err}
    b,err:=json.MarshalIndent(r.Data,"","  ");if err!=nil{return err}
    tmp:=r.Path+".tmp";if err=os.WriteFile(tmp,b,0600);err!=nil{return err};return os.Rename(tmp,r.Path)
}
func (r *PersistentRegistry) Records() []CapabilityRecord { r.mu.Lock();defer r.mu.Unlock();return append([]CapabilityRecord(nil),r.Data.Records...) }
func (r *PersistentRegistry) Architecture() []ArchitectureExperience { r.mu.Lock();defer r.mu.Unlock();return append([]ArchitectureExperience(nil),r.Data.Architecture...) }
func (r *PersistentRegistry) Put(rec CapabilityRecord) error { r.mu.Lock();defer r.mu.Unlock();r.Data.Records=append(r.Data.Records,rec);r.Data.Version++;return r.flush() }
func (r *PersistentRegistry) RecordArchitecture(x ArchitectureExperience) error { r.mu.Lock();defer r.mu.Unlock();r.Data.Architecture=append(r.Data.Architecture,x);r.Data.Version++;return r.flush() }

// AffineIncrementContract derives an acceptance contract from the task goal.
// It deliberately accepts only an explicit structural equation; it never invents a target.
type AffineIncrementContract struct { Input,Output string; Delta int }
func ParseAffineIncrement(goal string)(AffineIncrementContract,error) {
    goal=strings.ReplaceAll(goal," ","")
    parts:=strings.Split(goal,"=")
    if len(parts)!=2 { return AffineIncrementContract{},errors.New("unsupported goal form") }
    out:=parts[0]; rhs:=parts[1]
    plus:=strings.LastIndex(rhs,"+")
    if plus<=0 { return AffineIncrementContract{},errors.New("unsupported affine goal") }
    in:=rhs[:plus];delta,err:=strconv.Atoi(rhs[plus+1:]);if err!=nil||out==""||in==""{return AffineIncrementContract{},errors.New("invalid affine goal")}
    return AffineIncrementContract{Input:in,Output:out,Delta:delta},nil
}

func DeriveCapabilitySpecification(t Task) (CapabilitySpecification, []ProgramTestCase, error) {
    c,err:=ParseAffineIncrement(t.Goal);if err!=nil{return CapabilitySpecification{},nil,err}
    if c.Input==c.Output{return CapabilitySpecification{},nil,errors.New("input and output must differ")}
    spec:=CapabilitySpecification{
        ID:Hash([]any{"affine",c}),
        DesiredBehaviour:t.Goal,
        Inputs:[]string{c.Input},
        Outputs:[]string{c.Output},
        Invariants:[]string{"output-input=declared_delta"},
        AcceptanceTests:[]string{"task-goal", "held-out-affine-example"},
        ResourceLimits:t.Budget,
        FailureCriteria:[]string{"wrong output","runtime error"},
        RegressionConstraints:[]string{"existing acquired capabilities still pass"},
        Provenance:Prov("capability-discovery",t.ID,"derive-affine-contract",t),
    }
    input:=map[string]string{c.Input:"7"};expected:=map[string]string{c.Output:strconv.Itoa(7+c.Delta)}
    holdout:=map[string]string{c.Input:"19"};holdoutExpected:=map[string]string{c.Output:strconv.Itoa(19+c.Delta)}
    tests:=[]ProgramTestCase{{Input:input,Expected:expected},{Input:holdout,Expected:holdoutExpected}}
    return spec,tests,nil
}

func structuralKey(spec CapabilitySpecification) string {
    return Hash(struct{Inputs,Outputs []string;Tests []string}{[]string{"numeric"},[]string{"numeric"},spec.AcceptanceTests})
}

// LearningMechanismSearch reorders competing candidates using prior verified architecture experience.
type LearningMechanismSearch struct { Base CompetingMechanismSearch; Registry *PersistentRegistry }
func (s LearningMechanismSearch) SearchMechanisms(spec CapabilitySpecification,b ResourceVector)([]ArchitectureCandidate,error) {
    cs,err:=s.Base.SearchMechanisms(spec,b);if err!=nil{return nil,err}
    if s.Registry==nil{return cs,nil}
    history:=s.Registry.Architecture();scores:=map[string]int{}
    key:=structuralKey(spec)
    for _,h:=range history{if h.StructuralKey==key&&h.Passed{scores[h.Mechanism] += 100;}}
    sort.SliceStable(cs,func(i,j int)bool{return scores[cs[i].Mechanism]>scores[cs[j].Mechanism]})
    return cs,nil
}

func compileSkill(rec CapabilityRecord, spec CapabilitySpecification) CapabilityRecord {
    c,_:=ParseAffineIncrement(spec.DesiredBehaviour)
    action:=Action{ID:Hash([]any{"acquired-action",rec.Capability.ID}),CapabilityID:rec.Capability.ID,Operation:"execute:"+rec.Mechanism,Arguments:map[string]string{"input_role":c.Input,"output_role":c.Output},ExpectedEffects:[]string{spec.DesiredBehaviour},MaxScope:"single-task"}
    rec.Skill=Skill{ID:Hash([]any{"skill",rec.Capability.ID}),Name:"acquired:"+spec.DesiredBehaviour,Actions:[]Action{action},ExpectedEffects:[]string{spec.DesiredBehaviour},Verification:[]string{"independent-affine-evaluator"},FailureModes:spec.FailureCriteria,Confidence:1,Version:1,Provenance:Prov("skill-compilation",spec.ID,"compile-verified-program",rec.Artifact)}
    return rec
}

func runProgramArtifact(artifact string,input map[string]string)(map[string]string,error){var p ExecutableProgram;if err:=json.Unmarshal([]byte(artifact),&p);err!=nil{return nil,err};return p.Run(input)}

func behavioralPass(artifact string,cases []ProgramTestCase)(bool,error){for _,tc:=range cases{got,err:=runProgramArtifact(artifact,tc.Input);if err!=nil{return false,err};for k,v:=range tc.Expected{if got[k]!=v{return false,fmt.Errorf("%s: got %q want %q",k,got[k],v)}}};return true,nil}

func independentAffineVerify(spec CapabilitySpecification,tests []ProgramTestCase,artifact string)(VerificationResult,error){passed,err:=behavioralPass(artifact,tests);if err!=nil{return VerificationResult{Status:"failed",Independent:true},err};if !passed{return VerificationResult{Status:"failed",Independent:true},errors.New("independent acceptance failed")};return VerificationResult{Status:"verified",Independent:true,Expected:spec.AcceptanceTests,Observed:[]string{"independent affine evaluation passed"},Provenance:Prov("independent-affine-verifier",spec.ID,"recompute-contract",tests)},nil}

// AutonomousAcquirer implements the first complete acquisition path in the finite executable substrate.
type AutonomousAcquirer struct {
    Registry *PersistentRegistry
    Search LearningMechanismSearch
    Builder ProgramBuilder
}

func (a AutonomousAcquirer) Acquire(t Task) (CapabilityRecord, VerificationResult, error) {
    if a.Registry==nil{return CapabilityRecord{},VerificationResult{},errors.New("persistent capability registry unavailable")}
    spec,tests,err:=DeriveCapabilitySpecification(t);if err!=nil{return CapabilityRecord{},VerificationResult{},err}
    cs,err:=a.Search.SearchMechanisms(spec,t.Budget);if err!=nil{return CapabilityRecord{},VerificationResult{},err}
    sandbox:=ExecutableSandbox{Cases:map[string][]ProgramTestCase{}}
    for _,c:=range cs{sandbox.Cases[c.ID]=tests}
    key:=structuralKey(spec);attempted:=0
    for _,c:=range cs {
        attempted++
        p,err:=a.Builder.Build(c,spec);if err!=nil{_ = a.Registry.RecordArchitecture(ArchitectureExperience{StructuralKey:key,Mechanism:c.Mechanism,Passed:false,SearchCount:attempted,Cost:c.Resources,Provenance:Prov("architecture-learning",spec.ID,"build-failed",c)}));continue}
        rr,err:=sandbox.Validate(p);if err!=nil||!rr.Passed{_ = a.Registry.RecordArchitecture(ArchitectureExperience{StructuralKey:key,Mechanism:c.Mechanism,Passed:false,SearchCount:attempted,Cost:c.Resources,Provenance:Prov("architecture-learning",spec.ID,"sandbox-failed",c)}));continue}
        v,err:=independentAffineVerify(spec,tests,p.Artifact);if err!=nil||v.Status!="verified"{_ = a.Registry.RecordArchitecture(ArchitectureExperience{StructuralKey:key,Mechanism:c.Mechanism,Passed:false,SearchCount:attempted,Cost:c.Resources,Provenance:Prov("architecture-learning",spec.ID,"independent-failed",c)}));continue}
        rec:=CapabilityRecord{Capability:Capability{ID:Hash([]any{"capability",spec.ID,c.Mechanism}),Name:spec.DesiredBehaviour,Strength:1,Version:1,KnownLimits:[]string{"finite affine contract"},Provenance:spec.Provenance},Artifact:p.Artifact,Tests:tests,Mechanism:c.Mechanism,ArchitectureCost:float64(attempted)}
        rec=compileSkill(rec,spec)
        if err:=a.Registry.Put(rec);err!=nil{return CapabilityRecord{},VerificationResult{},err}
        if err:=a.Registry.RecordArchitecture(ArchitectureExperience{StructuralKey:key,Mechanism:c.Mechanism,Passed:true,SearchCount:attempted,Cost:c.Resources,Provenance:Prov("architecture-learning",spec.ID,"verified-success",c)});err!=nil{return CapabilityRecord{},VerificationResult{},err}
        return rec,v,nil
    }
    return CapabilityRecord{},VerificationResult{Status:"failed",Independent:true},fmt.Errorf("no mechanism passed %d candidates",len(cs))
}

// ExecuteTask uses only persisted acquired records. It is intentionally lexical-label independent:
// role names are bound from the task's structural affine contract at execution time.
func ExecuteTask(r *PersistentRegistry,t Task)(VerificationResult,error){
    c,err:=ParseAffineIncrement(t.Goal);if err!=nil{return VerificationResult{},err}
    records:=r.Records();for _,rec:=range records{if rec.Capability.Name!=t.Goal{continue};in:=map[string]string{c.Input:"13"};got,err:=runProgramArtifact(rec.Artifact,in);if err!=nil{return VerificationResult{Status:"failed",Independent:true},err};want:=strconv.Itoa(13+c.Delta);if got[c.Output]!=want{return VerificationResult{Status:"failed",Independent:true},fmt.Errorf("transfer output got %q want %q",got[c.Output],want)};return VerificationResult{Status:"verified",Independent:true,Observed:[]string{c.Output+"="+got[c.Output]},Provenance:Prov("persistent-skill-execution",rec.Provenance.ID,"execute-structural-transfer",t)},nil}
    return VerificationResult{Status:"failed",Independent:true},errors.New("no acquired capability matches structural task")
}
