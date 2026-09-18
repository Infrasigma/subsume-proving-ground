package t2

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"crypto/rand"
	mrand "math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

type StructuralVector struct {
	ASTDepth int
	GraphNodeCount int
	CycleRank int
	DependencyPathLength int
	BranchingFactor int
	OperatorCardinality int
}

type NoveltyRule struct {
	ASTDepthMin int
	GraphNodeCountMin int
	CycleRankMin int
	DependencyPathMin int
}

func (r NoveltyRule) IsNovel(s StructuralVector) bool {
	return (r.ASTDepthMin > 0 && s.ASTDepth >= r.ASTDepthMin) ||
		(r.GraphNodeCountMin > 0 && s.GraphNodeCount >= r.GraphNodeCountMin) ||
		(r.CycleRankMin > 0 && s.CycleRank >= r.CycleRankMin) ||
		(r.DependencyPathMin > 0 && s.DependencyPathLength >= r.DependencyPathMin)
}

func (r NoveltyRule) Formula() string {
	return "N(s)=I[ASTDepth>=ASTDepthMin] OR I[GraphNodeCount>=GraphNodeCountMin] OR I[CycleRank>=CycleRankMin] OR I[DependencyPathLength>=DependencyPathMin]"
}

type PublicTask struct {
	ID string
	Family string
	Prompt string
	Examples []ExamplePair
	Query json.RawMessage
	Structure StructuralVector
	Novel bool
}

type ExamplePair struct {
	Input json.RawMessage
	Output json.RawMessage
}

type ScoredTask struct {
	Public PublicTask
	Oracle json.RawMessage
}

type TaskGenerator interface {
	Name() string
	Generate(seed int64, novel bool) (ScoredTask, error)
}

type ArithmeticASTGenerator struct{}
func (ArithmeticASTGenerator) Name() string { return "G1_arithmetic_ast" }
func (ArithmeticASTGenerator) Generate(seed int64, novel bool) (ScoredTask, error) {
	r:=mrand.New(mrand.NewSource(seed)); depth:=2; if novel {depth=3}
	a,b,c,d,e:=r.Intn(5)+1,r.Intn(4)+2,r.Intn(3)+1,r.Intn(4)+1,r.Intn(5)-2
	f:=func(x int)int{v:=x+a;v=v*b+c;if depth>=3{v=v*d+e};return v}
	inputs:=[]int{seededInt(r),seededInt(r),seededInt(r)}
	ex:=make([]ExamplePair,0,3);for _,x:=range inputs{ex=append(ex,ExamplePair{mustRaw(x),mustRaw(f(x))})}
	q:=seededInt(r)
	s:=StructuralVector{ASTDepth:depth,OperatorCardinality:depth+1,BranchingFactor:2}
	pub:=PublicTask{ID:fmt.Sprintf("g1-%d-%t",seed,novel),Family:"G1_arithmetic_ast",Prompt:"Infer the deterministic numeric transformation from the examples, then return the output for the query input.",Examples:ex,Query:mustRaw(q),Structure:s,Novel:novel}
	return ScoredTask{Public:pub,Oracle:mustRaw(f(q))},nil
}

type FSMGenerator struct{}
func (FSMGenerator) Name() string { return "G2_finite_state" }
func (FSMGenerator) Generate(seed int64, novel bool) (ScoredTask, error) {
	r:=rand.New(rand.NewSource(seed)); n:=4;if novel{n=6};cycle:=1;if novel{cycle=2}
	trans:=make([]int,n);for i:=0;i<n;i++{trans[i]=(i+1)%n};for j:=0;j<cycle;j++{from:=r.Intn(n);to:=r.Intn(n);trans[from]=to}
	seqs:=[][]int{{0,1,2},{1,2,3,4},{2,0,1,3}}
	ex:=make([]ExamplePair,0,len(seqs));for _,seq:=range seqs{st:=0;for _,step:=range seq{st=(trans[(st+step)%n]+step)%n};ex=append(ex,ExamplePair{mustRaw(seq),mustRaw(st)})}
	q:=[]int{r.Intn(n),r.Intn(n)+1,r.Intn(n)+2,r.Intn(n)+3};st:=0;for _,step:=range q{st=(trans[(st+step)%n]+step)%n}
	s:=StructuralVector{GraphNodeCount:n,CycleRank:cycle,BranchingFactor:1}
	pub:=PublicTask{ID:fmt.Sprintf("g2-%d-%t",seed,novel),Family:"G2_finite_state",Prompt:"Infer the deterministic state transition rule from the observed sequences, then return the terminal state for the query sequence.",Examples:ex,Query:mustRaw(q),Structure:s,Novel:novel}
	return ScoredTask{Public:pub,Oracle:mustRaw(st)},nil
}

type BooleanASTGenerator struct{}
func (BooleanASTGenerator) Name() string { return "G3_boolean_ast" }
func (BooleanASTGenerator) Generate(seed int64, novel bool) (ScoredTask, error) {
	r:=rand.New(rand.NewSource(seed));depth:=2;if novel{depth=3};a,b,c:=r.Intn(2),r.Intn(2),r.Intn(2)
	f:=func(x,y,z int)int{v:=x^a;v&=(y|b);v^=c;if depth>=3{v=(v|x)&(z|1)};return v&1}
	exInputs:=[][3]int{{0,0,0},{0,1,0},{1,0,1},{1,1,0}};ex:=make([]ExamplePair,0,len(exInputs))
	for _,in:=range exInputs{ex=append(ex,ExamplePair{mustRaw(in),mustRaw(f(in[0],in[1],in[2]))})}
	q:=[3]int{r.Intn(2),r.Intn(2),r.Intn(2)}
	s:=StructuralVector{ASTDepth:depth,OperatorCardinality:4,BranchingFactor:2}
	pub:=PublicTask{ID:fmt.Sprintf("g3-%d-%t",seed,novel),Family:"G3_boolean_ast",Prompt:"Infer the deterministic boolean transformation from the examples, then return 0 or 1 for the query triple.",Examples:ex,Query:mustRaw(q),Structure:s,Novel:novel}
	return ScoredTask{Public:pub,Oracle:mustRaw(f(q[0],q[1],q[2]))},nil
}

type ListRewriteGenerator struct{}
func (ListRewriteGenerator) Name() string { return "G4_list_rewrite" }
func (ListRewriteGenerator) Generate(seed int64, novel bool) (ScoredTask, error) {
	r:=rand.New(rand.NewSource(seed));path:=3;if novel{path=5}
	f:=func(xs []int)[]int{ys:=append([]int(nil),xs...);for i:=0;i<path;i++{if i%2==0{for j:=range ys{ys[j]+=i+1}}else{for l,r:=0,len(ys)-1;l<r;l,r=l+1,r-1{ys[l],ys[r]=ys[r],ys[l]}}};return ys}
	inputs:=[][]int{{1,2,3},{2,4,6},{-1,0,2}};ex:=make([]ExamplePair,0,len(inputs));for _,in:=range inputs{ex=append(ex,ExamplePair{mustRaw(in),mustRaw(f(in))})}
	q:=[]int{r.Intn(7)-3,r.Intn(7)-3,r.Intn(7)-3,r.Intn(7)-3}
	s:=StructuralVector{DependencyPathLength:path,BranchingFactor:1}
	pub:=PublicTask{ID:fmt.Sprintf("g4-%d-%t",seed,novel),Family:"G4_list_rewrite",Prompt:"Infer the ordered list-rewrite procedure from the examples, then return the transformed query list.",Examples:ex,Query:mustRaw(q),Structure:s,Novel:novel}
	return ScoredTask{Public:pub,Oracle:mustRaw(f(q))},nil
}

func seededInt(r *mrand.Rand) int { return r.Intn(19)-9 }
func mustRaw(v any) json.RawMessage {b,_:=json.Marshal(v);return b}

type GeneratorPlan struct { Generators []TaskGenerator; NonNovelPerFamily int; NovelPerFamily int; Seed int64; Novelty NoveltyRule }
func DefaultGeneratorPlan(seed int64) GeneratorPlan {
	return GeneratorPlan{Generators:[]TaskGenerator{ArithmeticASTGenerator{},FSMGenerator{},BooleanASTGenerator{},ListRewriteGenerator{}},NonNovelPerFamily:8,NovelPerFamily:8,Seed:seed,Novelty:NoveltyRule{ASTDepthMin:3,GraphNodeCountMin:6,CycleRankMin:2,DependencyPathMin:5}}
}

func GenerateTaskPool(plan GeneratorPlan)([]ScoredTask,error){
	if len(plan.Generators)<2{return nil,errors.New("at least two generator families required")}
	out:=make([]ScoredTask,0,len(plan.Generators)*(plan.NonNovelPerFamily+plan.NovelPerFamily))
	for gi,g:=range plan.Generators{
		for i:=0;i<plan.NonNovelPerFamily;i++{t,err:=g.Generate(plan.Seed+int64(gi*1000+i),false);if err!=nil{return nil,err};if plan.Novelty.IsNovel(t.Public.Structure){return nil,fmt.Errorf("%s crossed novelty boundary",t.Public.ID)};out=append(out,t)}
		for i:=0;i<plan.NovelPerFamily;i++{t,err:=g.Generate(plan.Seed+100000+int64(gi*1000+i),true);if err!=nil{return nil,err};if !plan.Novelty.IsNovel(t.Public.Structure){return nil,fmt.Errorf("%s failed novelty boundary",t.Public.ID)};out=append(out,t)}
	}
	sort.Slice(out,func(i,j int)bool{return out[i].Public.ID<out[j].Public.ID})
	seen:=map[string]bool{};for _,t:=range out{if seen[t.Public.ID]{return nil,errors.New("duplicate generated task id")};seen[t.Public.ID]=true}
	return out,nil
}

type SplitManifest struct { ProtocolVersion,PreregHash,NoveltyFormula,DiscoverHash,SelectHash,ValidatePlainHash,ValidateCipherHash string; Counts map[string]int }

func encodeScored(tasks []ScoredTask)[]byte{var b bytes.Buffer;e:=json.NewEncoder(&b);for _,t:=range tasks{_ = e.Encode(t)};return b.Bytes()}
func encodePublic(tasks []ScoredTask)[]byte{var b bytes.Buffer;e:=json.NewEncoder(&b);for _,t:=range tasks{_ = e.Encode(t.Public)};return b.Bytes()}

func AuditGeneratedPool(tasks []ScoredTask,p Preregistration) error {
	public:=make([]Task,0,len(tasks))
	for _,t:=range tasks{public=append(public,Task{ID:t.Public.ID,Family:t.Public.Family,Prompt:t.Public.Prompt,Latent:map[string]string{"novel":strconv.FormatBool(t.Public.Novel)},Structure:map[string]string{"ast_depth":strconv.Itoa(t.Public.Structure.ASTDepth),"graph_nodes":strconv.Itoa(t.Public.Structure.GraphNodeCount),"cycle_rank":strconv.Itoa(t.Public.Structure.CycleRank),"dependency_path":strconv.Itoa(t.Public.Structure.DependencyPathLength)}})}
	if err:=AuditGenerator(public,p);err!=nil{return err}
	for _,t:=range tasks{if p.Generator.Novelty.IsNovel(t.Public.Structure)!=t.Public.Novel{return fmt.Errorf("novelty bit inconsistent for %s",t.Public.ID)}}
	return nil
}

func GenerateAuditPool(plan GeneratorPlan) ([]ScoredTask,error){
	audit:=plan
	audit.Seed=plan.Seed+9000000
	audit.NonNovelPerFamily=4
	audit.NovelPerFamily=4
	return GenerateTaskPool(audit)
}

func WriteAuditPool(tasks []ScoredTask,outDir string) error {
	if len(tasks)==0{return errors.New("empty audit pool")}
	if err:=os.MkdirAll(outDir,0700);err!=nil{return err}
	public:=encodePublic(tasks)
	scored:=encodeScored(tasks)
	if err:=os.WriteFile(filepath.Join(outDir,"D_audit.jsonl"),public,0600);err!=nil{return err}
	if err:=os.WriteFile(filepath.Join(outDir,"D_audit_scored.jsonl"),scored,0600);err!=nil{return err}
	return nil
}

func GenerateAndSplit(plan GeneratorPlan,p Preregistration,outDir string)(SplitManifest,[]byte,error){
	tasks,err:=GenerateTaskPool(plan);if err!=nil{return SplitManifest{},nil,err};if err:=AuditGeneratedPool(tasks,p);err!=nil{return SplitManifest{},nil,err}
	var d,s,v []ScoredTask
	nonNovel:=[]ScoredTask{};novel:=[]ScoredTask{}
	for _,t:=range tasks{if t.Public.Novel{novel=append(novel,t)}else{nonNovel=append(nonNovel,t)}}
	sort.Slice(nonNovel,func(i,j int)bool{return nonNovel[i].Public.ID<nonNovel[j].Public.ID});sort.Slice(novel,func(i,j int)bool{return novel[i].Public.ID<novel[j].Public.ID})
	discoverCount:=len(nonNovel)/2;selectCount:=len(nonNovel)-discoverCount
	d=append(d,nonNovel[:discoverCount]...);s=append(s,nonNovel[discoverCount:discoverCount+selectCount]...);v=append(v,novel...)
	if len(d)==0||len(s)==0||len(v)==0{return SplitManifest{},nil,errors.New("stratified split produced empty partition")}
	db,sb,vb:=encodePublic(d),encodeScored(s),encodeScored(v)
	if err:=os.MkdirAll(outDir,0700);err!=nil{return SplitManifest{},nil,err}
	if err:=os.WriteFile(filepath.Join(outDir,"D_discover.jsonl"),db,0600);err!=nil{return SplitManifest{},nil,err}
	if err:=os.WriteFile(filepath.Join(outDir,"D_select.jsonl"),sb,0600);err!=nil{return SplitManifest{},nil,err}
	key:=make([]byte,32);if _,err:=rand.Read(key);err!=nil{return SplitManifest{},nil,err}
	ct,err:=Encrypt(vb,key,p.StudyID+":D_validate");if err!=nil{return SplitManifest{},nil,err}
	m:=SplitManifest{ProtocolVersion:ProtocolVersion,PreregHash:p.CriteriaHash,NoveltyFormula:plan.Novelty.Formula(),DiscoverHash:SHA256Bytes(db),SelectHash:SHA256Bytes(sb),ValidatePlainHash:SHA256Bytes(vb),ValidateCipherHash:SHA256Bytes(ct),Counts:map[string]int{"discover":len(d),"select":len(s),"validate":len(v)}}
	mb,_:=json.MarshalIndent(m,"","  ");if err:=os.WriteFile(filepath.Join(outDir,"SPLIT_MANIFEST.json"),mb,0600);err!=nil{return SplitManifest{},nil,err}
	if err:=os.WriteFile(filepath.Join(outDir,"D_validate.enc"),ct,0600);err!=nil{return SplitManifest{},nil,err}
	return m,key,nil
}
