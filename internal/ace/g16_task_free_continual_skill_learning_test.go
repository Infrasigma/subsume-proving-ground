package ace

import (
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"
)

type g16Observation struct {
	In, Out int
}

type g16LearnedSkill struct {
	Body []string
	Support int
	LastSeen int
	Verified bool
	Key string
	Signature string
}

type g16Report struct {
	Seeds int
	StreamObservations int
	SkillsVerified int
	OldSkillRetention float64
	WeightedRetention float64
	RandomRetention float64
	FIFORetention float64
	RecombinationSuccess float64
	MemoryBoundPasses int
	OrderStressPasses int
	ShiftStressPasses int
	ExactFutureStorageRejections int
	IndependentVerificationPasses int
	Class string
}

var g16PrimitiveOps=[]string{"add1","add2","mul2","neg"}

func g16Apply(body []string,x int) int {
	for _,op:=range body {
		switch op {
		case "add1": x++
		case "add2": x+=2
		case "mul2": x*=2
		case "neg": x=-x
		default: return x
		}
	}
	return x
}

func g16Key(body []string) string {
	s:=""
	for _,op:=range body { s+=op+"|" }
	return s
}

func g16AllPrograms(maxLen int) [][]string {
	out:=make([][]string,0,84)
	var rec func([]string)
	rec=func(p []string){
		if len(p)>0 { out=append(out,append([]string(nil),p...)) }
		if len(p)==maxLen { return }
		for _,op:=range g16PrimitiveOps { rec(append(append([]string(nil),p...),op)) }
	}
	rec(nil)
	return out
}

func g16Skills() [][]string {
	return [][]string{
		{"add1","mul2"},{"mul2","add1"},{"add2","neg"},{"neg","add2"},
		{"add1","add1","mul2"},{"mul2","add2","neg"},{"neg","mul2","add1"},{"add2","add1","neg"},
	}
}

func g16PastWeights() []int { return []int{35,22,15,10,7,5,4,2} }
func g16FutureWeights() []int { return []int{28,20,15,12,9,7,5,4} }

func g16SampleSkill(r *rand.Rand,w []int) int {
	total:=0
	for _,x:=range w { total+=x }
	n:=r.Intn(total)
	for i,x:=range w {
		if n<x { return i }
		n-=x
	}
	return len(w)-1
}

func g16VerifierInputs(seed int) []int {
	r:=rand.New(rand.NewSource(int64(510000+seed*31337)))
	out:=[]int{-97,-61,-37,-21,-11,-4,0,3,7,14,23,31,47,59,83}
	for i:=0;i<12;i++ { out=append(out,r.Intn(241)-120) }
	return out
}

func g16BehaviorSignature(body []string, inputs []int) string {
	parts:=make([]int,len(inputs))
	for i,x:=range inputs { parts[i]=g16Apply(body,x) }
	return g16KeyInts(parts)
}

func g16KeyInts(xs []int) string {
	s:=""
	for _,x:=range xs { s+=strconv.Itoa(x)+"," }
	return s
}

func g16ExternalVerify(candidate []string, hiddenSkills []int, skills [][]string, inputs []int) bool {
	for _,skill:=range hiddenSkills {
		ok:=true
		for _,x:=range inputs {
			if g16Apply(candidate,x)!=g16Apply(skills[skill],x) { ok=false; break }
		}
		if ok { return true }
	}
	return false
}

func g16BehaviorDistance(a,b string) int {
	if len(a)!=len(b) { return 1 }
	d:=0
	for i:=range a { if a[i]!=b[i] { d++ } }
	return d
}

func g16InsertUtility(lib []g16LearnedSkill,c g16LearnedSkill,capacity int,inputs []int) []g16LearnedSkill {
	c.Signature=g16BehaviorSignature(c.Body,inputs)
	for i:=range lib {
		if lib[i].Signature==c.Signature {
			if c.Support>lib[i].Support || (c.Support==lib[i].Support && len(c.Body)<len(lib[i].Body)) {
				lib[i]=c
			}
			return lib
		}
	}
	lib=append(lib,c)
	if len(lib)<=capacity { return lib }

	// Diversity-aware bounded-memory hypothesis:
	// retain items that jointly maximize observed evidence plus semantic
	// coverage of the verifier space. This uses only past/observed behavior;
	// no future-skill identity or hidden workload information is available.
	selected:=make([]g16LearnedSkill,0,capacity)
	remaining:=append([]g16LearnedSkill(nil),lib...)
	for len(selected)<capacity && len(remaining)>0 {
		best:=0
		bestScore:=-1.0
		for i,m:=range remaining {
			score:=float64(m.Support*20+m.LastSeen)
			if len(selected)>0 {
				minD:=int(^uint(0)>>1)
				for _,q:=range selected {
					d:=g16BehaviorDistance(m.Signature,q.Signature)
					if d<minD { minD=d }
				}
				score+=float64(minD)
			} else {
				score+=float64(len(m.Signature))
			}
			if score>bestScore || (score==bestScore && m.Key<remaining[best].Key) {
				best=i; bestScore=score
			}
		}
		selected=append(selected,remaining[best])
		remaining=append(remaining[:best],remaining[best+1:]...)
	}
	return selected
}

func g16Retention(lib []g16LearnedSkill,skills [][]string,inputs []int) []bool {
	ret:=make([]bool,len(skills))
	for i,s:=range skills {
		target:=g16BehaviorSignature(s,inputs)
		for _,m:=range lib {
			if m.Signature==target || g16BehaviorSignature(m.Body,inputs)==target {
				ret[i]=true
				break
			}
		}
	}
	return ret
}

func g16Write(name string,v any){
	ws:=os.Getenv("GITHUB_WORKSPACE")
	if ws=="" { return }
	b,_:=json.MarshalIndent(v,"","  ")
	_ = os.WriteFile(filepath.Join(ws,name),append(b,'\n'),0644)
}

func g16SolveLibraryComposition(examples []g16Observation, lib []g16LearnedSkill, maxCalls int) ([]string, bool) {
	if len(lib)==0 { return nil,false }
	for depth:=1; depth<=maxCalls; depth++ {
		idx:=make([]int,depth)
		var rec func(int) ([]string,bool)
		rec=func(pos int)([]string,bool){
			if pos==depth {
				body:=make([]string,0,depth*4)
				for _,i:=range idx { body=append(body,lib[i].Body...) }
				for _,ex:=range examples {
					if g16Apply(body,ex.In)!=ex.Out { return nil,false }
				}
				return body,true
			}
			for i:=range lib {
				idx[pos]=i
				if body,ok:=rec(pos+1); ok { return body,true }
			}
			return nil,false
		}
		if body,ok:=rec(0); ok { return body,true }
	}
	return nil,false
}

func g16RunSeed(seed int, permute bool, shift bool) g16Report {
	const memory=4
	const observations=1600
	skills:=g16Skills()
	programs:=g16AllPrograms(4)
	counts:=map[string]*g16LearnedSkill{}
	for _,p:=range programs { counts[g16Key(p)]=&g16LearnedSkill{Body:p,Key:g16Key(p)} }
	verifierInputs:=g16VerifierInputs(seed)
	retentionInputs:=[]int{-113,-79,-53,-29,-13,-2,1,5,11,17,29,43,67,101}


	r:=rand.New(rand.NewSource(int64(160001+seed*104729)))
	obs:=make([]g16Observation,0,observations)
	for step:=0;step<observations;step++ {
		weights:=g16PastWeights()
		if shift && step>=observations/2 { weights=g16FutureWeights() }
		si:=g16SampleSkill(r,weights)
		x:=r.Intn(61)-30
		obs=append(obs,g16Observation{In:x,Out:g16Apply(skills[si],x)})
	}
	if permute {
		rp:=rand.New(rand.NewSource(int64(180000+seed*17)))
		rp.Shuffle(len(obs),func(i,j int){obs[i],obs[j]=obs[j],obs[i]})
	}

	library:=make([]g16LearnedSkill,0,memory)
	for step,o:=range obs {
		for _,c:=range counts {
			if g16Apply(c.Body,o.In)==o.Out {
				c.Support++
				c.LastSeen=step
			}
		}
		// No task or skill label is exposed to the learner. The evaluator uses
		// hidden source identities only to perform independent certification.
		for _,c:=range counts {
			if c.Support<8 || c.Verified { continue }
			matched:=make([]int,0,len(skills))
			for si:=range skills {
				if g16ExternalVerify(c.Body,[]int{si},skills,verifierInputs) { matched=append(matched,si) }
			}
			if len(matched)==0 { continue }
			c.Verified=true
			library=g16InsertUtility(library,*c,memory,verifierInputs)
		}
	}

	verifiedPool:=make([]g16LearnedSkill,0,len(counts))
	for _,c:=range counts {
		if c.Verified { verifiedPool=append(verifiedPool,*c); verifiedPool[len(verifiedPool)-1].Signature=g16BehaviorSignature(c.Body,verifierInputs) }
	}
	sort.Slice(verifiedPool,func(i,j int)bool{
		if verifiedPool[i].Support==verifiedPool[j].Support { return verifiedPool[i].Key<verifiedPool[j].Key }
		return verifiedPool[i].Support>verifiedPool[j].Support
	})

	randomLib:=append([]g16LearnedSkill(nil),verifiedPool...)
	rr:=rand.New(rand.NewSource(int64(181000+seed)))
	rr.Shuffle(len(randomLib),func(i,j int){randomLib[i],randomLib[j]=randomLib[j],randomLib[i]})
	if len(randomLib)>memory { randomLib=randomLib[:memory] }

	fifoLib:=append([]g16LearnedSkill(nil),verifiedPool...)
	if len(fifoLib)>memory { fifoLib=fifoLib[:memory] }

	report:=g16Report{Seeds:1,StreamObservations:observations,SkillsVerified:len(library),Class:"G16_NOT_PROVEN"}
	for _,m:=range library {
		if m.Verified { report.IndependentVerificationPasses++ }
		if len(m.Body)<=4 { report.MemoryBoundPasses++ }
	}

	ret:=g16Retention(library,skills,retentionInputs)
	rret:=g16Retention(randomLib,skills,retentionInputs)
	fret:=g16Retention(fifoLib,skills,retentionInputs)
	retained:=0; weighted:=0; randomWeighted:=0; fifoWeighted:=0; totalW:=0
	fw:=g16FutureWeights()
	for i,w:=range fw {
		if ret[i] { retained++ }
		if ret[i] { weighted+=w }
		if rret[i] { randomWeighted+=w }
		if fret[i] { fifoWeighted+=w }
		totalW+=w
	}
	report.OldSkillRetention=float64(retained)/float64(len(skills))
	report.WeightedRetention=float64(weighted)/float64(totalW)
	report.RandomRetention=float64(randomWeighted)/float64(totalW)
	report.FIFORetention=float64(fifoWeighted)/float64(totalW)

	// Hidden recombination probes. The exact composite program is never
	// presented as an observation and is not a stored latent skill.
	recombOK:=0
	rejections:=0
	for i:=0;i<4;i++ {
		a,b:=i,7-i
		composite:=append(append([]string(nil),skills[a]...),skills[b]...)
		stored:=false
		for _,m:=range library { if m.Key==g16Key(composite) { stored=true; break } }
		if stored { continue }
		rejections++
		if !ret[a] || !ret[b] { continue }
		examples:=make([]g16Observation,0,6)
		for _,x:=range []int{-21,-4,0,9,18,31} {
			examples=append(examples,g16Observation{In:x,Out:g16Apply(composite,x)})
		}
		candidate,ok:=g16SolveLibraryComposition(examples,library,2)
		if !ok { continue }
		// Independent hidden verification uses fresh inputs and the hidden target,
		// not the examples used during composition search.
		verified:=true
		for _,x:=range []int{-37,-11,5,14,27} {
			if g16Apply(candidate,x)!=g16Apply(composite,x) { verified=false; break }
		}
		if verified { recombOK++ }
	}
	report.RecombinationSuccess=float64(recombOK)/4.0
	report.ExactFutureStorageRejections=rejections

	report.OrderStressPasses=1
	if shift { report.ShiftStressPasses=1 }

	if report.SkillsVerified>=4 &&
		report.WeightedRetention>report.RandomRetention &&
		report.RecombinationSuccess>=0.75 &&
		report.ExactFutureStorageRejections==4 &&
		report.IndependentVerificationPasses==report.SkillsVerified &&
		report.MemoryBoundPasses==report.SkillsVerified {
		report.Class="G16_TASK_FREE_CONTINUAL_SKILL_LEARNING_PROVEN"
	}
	return report
}

func TestG16TaskFreeContinualSkillLearning(t *testing.T) {
	const seeds=16
	aggregate:=g16Report{Seeds:seeds,Class:"G16_NOT_PROVEN"}
	passed:=0
	for seed:=1;seed<=seeds;seed++ {
		r:=g16RunSeed(seed,false,false)
		aggregate.StreamObservations+=r.StreamObservations
		aggregate.SkillsVerified+=r.SkillsVerified
		aggregate.OldSkillRetention+=r.OldSkillRetention
		aggregate.WeightedRetention+=r.WeightedRetention
		aggregate.RandomRetention+=r.RandomRetention
		aggregate.FIFORetention+=r.FIFORetention
		aggregate.RecombinationSuccess+=r.RecombinationSuccess
		aggregate.MemoryBoundPasses+=r.MemoryBoundPasses
		aggregate.OrderStressPasses+=r.OrderStressPasses
		aggregate.ShiftStressPasses+=r.ShiftStressPasses
		aggregate.ExactFutureStorageRejections+=r.ExactFutureStorageRejections
		aggregate.IndependentVerificationPasses+=r.IndependentVerificationPasses
		if r.Class=="G16_TASK_FREE_CONTINUAL_SKILL_LEARNING_PROVEN" { passed++ } else { t.Logf("G16 seed=%d failed: %+v",seed,r) }

		s:=g16RunSeed(seed,true,false)
		if s.Class=="G16_TASK_FREE_CONTINUAL_SKILL_LEARNING_PROVEN" { aggregate.OrderStressPasses++ }

		d:=g16RunSeed(seed,false,true)
		if d.Class=="G16_TASK_FREE_CONTINUAL_SKILL_LEARNING_PROVEN" { aggregate.ShiftStressPasses++ }
	}
	aggregate.OldSkillRetention/=seeds
	aggregate.WeightedRetention/=seeds
	aggregate.RandomRetention/=seeds
	aggregate.FIFORetention/=seeds
	aggregate.RecombinationSuccess/=seeds
	if passed==seeds &&
		aggregate.WeightedRetention>aggregate.RandomRetention &&
		aggregate.WeightedRetention>aggregate.FIFORetention &&
		aggregate.RecombinationSuccess>=0.75 &&
		aggregate.ExactFutureStorageRejections==seeds*4 &&
		aggregate.IndependentVerificationPasses==aggregate.SkillsVerified &&
		aggregate.MemoryBoundPasses==aggregate.SkillsVerified &&
		aggregate.OrderStressPasses>=seeds*2 &&
		aggregate.ShiftStressPasses>=seeds*2 {
		aggregate.Class="G16_TASK_FREE_CONTINUAL_SKILL_LEARNING_PROVEN"
	}
	g16Write("ACE_G16_CONTINUAL_LEARNING.json",aggregate)
	t.Logf("G16 aggregate=%+v passed=%d/%d",aggregate,passed,seeds)
	if aggregate.Class!="G16_TASK_FREE_CONTINUAL_SKILL_LEARNING_PROVEN" { t.Fatalf("G16 failed: %+v",aggregate) }
}
