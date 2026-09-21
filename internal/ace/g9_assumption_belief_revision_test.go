package ace

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

type g9Assumption struct {
	ID     string
	Literal string
	Cost   int
	Hard   bool
}

type g9Rule struct {
	Antecedents []string
	Consequent string
}

type g9RevisionReport struct {
	Updates                 int
	OptimalPasses           int
	ConsistencyPasses       int
	SuccessPasses           int
	NoOverRetractionPasses  int
	OrderInvariantPasses    int
	NaiveInconsistencies    int
	Classification           string
}

func g9Neg(lit string) string {
	if len(lit) > 0 && lit[0] == '!' { return lit[1:] }
	return "!" + lit
}

func g9Atoms(lit string) string {
	if len(lit) > 0 && lit[0] == '!' { return lit[1:] }
	return lit
}

func g9SubsetKey(xs []int) string {
	cp := append([]int(nil), xs...)
	sort.Ints(cp)
	out := make([]byte, 0, len(cp)*3)
	for _, x := range cp { out = append(out, byte(x+1)) }
	return string(out)
}

func g9NormalizeSets(in [][]int) [][]int {
	uniq := map[string][]int{}
	for _, s := range in {
		cp := append([]int(nil), s...)
		sort.Ints(cp)
		k := g9SubsetKey(cp)
		uniq[k] = cp
	}
	all := make([][]int, 0, len(uniq))
	for _, s := range uniq { all = append(all, s) }
	sort.Slice(all, func(i,j int) bool {
		if len(all[i]) == len(all[j]) { return g9SubsetKey(all[i]) < g9SubsetKey(all[j]) }
		return len(all[i]) < len(all[j])
	})
	minimal := make([][]int, 0, len(all))
	for _, s := range all {
		superset := false
		for _, m := range minimal {
			if g9IsSubset(m, s) { superset = true; break }
		}
		if !superset { minimal = append(minimal, s) }
	}
	return minimal
}

func g9IsSubset(a,b []int) bool {
	i,j := 0,0
	for i < len(a) && j < len(b) {
		if a[i] == b[j] { i++; j++ } else if a[i] > b[j] { j++ } else { return false }
	}
	return i == len(a)
}

func g9Union(a,b []int) []int {
	out := append([]int(nil), a...)
	out = append(out, b...)
	sort.Ints(out)
	res := out[:0]
	for _, x := range out { if len(res) == 0 || res[len(res)-1] != x { res = append(res, x) } }
	return res
}

func g9Labels(assumptions []g9Assumption, rules []g9Rule) map[string][][]int {
	labels := map[string][][]int{}
	for id, a := range assumptions {
		labels[a.Literal] = g9NormalizeSets(append(labels[a.Literal], []int{id}))
	}
	changed := true
	for changed {
		changed = false
		for _, rule := range rules {
			if len(rule.Antecedents) == 0 { continue }
			sets := [][]int{{}}
			for _, ant := range rule.Antecedents {
				next := make([][]int, 0)
				for _, base := range sets {
					for _, lab := range labels[ant] {
						next = append(next, g9Union(base, lab))
					}
				}
				sets = g9NormalizeSets(next)
				if len(sets) == 0 { break }
			}
			if len(sets) == 0 { continue }
			merged := append(append([][]int(nil), labels[rule.Consequent]...), sets...)
			norm := g9NormalizeSets(merged)
			if len(norm) != len(labels[rule.Consequent]) || !g9EqualSets(norm, labels[rule.Consequent]) {
				labels[rule.Consequent] = norm
				changed = true
			}
		}
	}
	return labels
}

func g9EqualSets(a,b [][]int) bool {
	if len(a) != len(b) { return false }
	for i := range a {
		if g9SubsetKey(a[i]) != g9SubsetKey(b[i]) { return false }
	}
	return true
}

func g9Nogoods(labels map[string][][]int) [][]int {
	out := make([][]int, 0)
	seenAtoms := map[string]bool{}
	for lit := range labels { seenAtoms[g9Atoms(lit)] = true }
	for atom := range seenAtoms {
		pos, neg := labels[atom], labels["!"+atom]
		for _, a := range pos {
			for _, b := range neg { out = append(out, g9Union(a,b)) }
		}
	}
	return g9NormalizeSets(out)
}

func g9Contains(xs []int, x int) bool {
	i := sort.SearchInts(xs, x)
	return i < len(xs) && xs[i] == x
}

type g9HitResult struct {
	IDs []int
	Cost int
	OK  bool
}

func g9HitNogoods(nogoods [][]int, assumptions []g9Assumption) g9HitResult {
	retractable := map[int]bool{}
	cost := map[int]int{}
	hard := map[int]bool{}
	for id, a := range assumptions {
		retractable[id], cost[id], hard[id] = !a.Hard, a.Cost, a.Hard
	}
	for _, ng := range nogoods {
		can := false
		for _, id := range ng {
			if retractable[id] { can = true; break }
		}
		if !can { return g9HitResult{OK:false} }
	}
	best := g9HitResult{Cost:math.MaxInt, OK:false}
	var dfs func([]int, int, [][]int)
	dfs = func(chosen []int, total int, pending [][]int) {
		if total > best.Cost { return }
		if len(pending) == 0 {
			cp := append([]int(nil), chosen...)
			sort.Ints(cp)
			if !best.OK || total < best.Cost || (total == best.Cost && g9SubsetKey(cp) < g9SubsetKey(best.IDs)) {
				best = g9HitResult{IDs:cp, Cost:total, OK:true}
			}
			return
		}
		// Select the shortest currently-unhit nogood to branch on.
		sort.SliceStable(pending, func(i,j int) bool {
			if len(pending[i]) == len(pending[j]) { return g9SubsetKey(pending[i]) < g9SubsetKey(pending[j]) }
			return len(pending[i]) < len(pending[j])
		})
		var target []int
		for _, ng := range pending {
			hit := false
			for _, id := range chosen { if g9Contains(ng,id) { hit=true; break } }
			if !hit { target=ng; break }
		}
		if target == nil {
			dfs(chosen,total,nil)
			return
		}
		for _, id := range target {
			if !retractable[id] { continue }
			if g9Contains(chosen,id) { continue }
			nextChosen := append(append([]int(nil), chosen...), id)
			nextPending := make([][]int,0,len(pending))
			for _, ng := range pending {
				if !g9Contains(ng,id) { nextPending=append(nextPending,ng) }
			}
			dfs(nextChosen,total+cost[id],nextPending)
		}
	}
	dfs(nil,0,nogoods)
	_ = hard
	return best
}

func g9Closure(assumptions []g9Assumption, rules []g9Rule) map[string]bool {
	out := map[string]bool{}
	for _, a := range assumptions { out[a.Literal] = true }
	changed := true
	for changed {
		changed = false
		for _, rule := range rules {
			ok := true
			for _, ant := range rule.Antecedents {
				if !out[ant] { ok=false; break }
			}
			if ok && !out[rule.Consequent] {
				out[rule.Consequent]=true
				changed=true
			}
		}
	}
	return out
}

func g9Consistent(assumptions []g9Assumption, rules []g9Rule) bool {
	closure := g9Closure(assumptions,rules)
	for lit := range closure {
		if closure[g9Neg(lit)] { return false }
	}
	return true
}

func g9Oracle(base []g9Assumption, evidence g9Assumption, rules []g9Rule) g9HitResult {
	all := append(append([]g9Assumption(nil), base...), evidence)
	old := make([]int, len(base))
	for i := range base { old[i]=i }
	best := g9HitResult{Cost:math.MaxInt}
	limit := 1 << len(old)
	for mask := 0; mask < limit; mask++ {
		cost := 0
		kept := make([]g9Assumption,0,len(all))
		for i,a := range all {
			if i < len(base) && mask&(1<<i) != 0 { cost += a.Cost; continue }
			kept=append(kept,a)
		}
		if !g9Consistent(kept,rules) { continue }
		ids:=make([]int,0)
		for i := range base { if !base[i].Hard && mask&(1<<i) != 0 { ids=append(ids,i) } }
		if !best.OK || cost < best.Cost || (cost==best.Cost && g9SubsetKey(ids)<g9SubsetKey(best.IDs)) {
			best=g9HitResult{IDs:ids,Cost:cost,OK:true}
		}
	}
	return best
}

func g9Revise(base []g9Assumption, evidence g9Assumption, rules []g9Rule) ([]g9Assumption,g9HitResult) {
	all:=append(append([]g9Assumption(nil),base...),evidence)
	labels:=g9Labels(all,rules)
	nogoods:=g9Nogoods(labels)
	best:=g9HitNogoods(nogoods,all)
	if !best.OK { return nil,best }
	retract:=map[int]bool{}
	for _,id:=range best.IDs { retract[id]=true }
	out:=make([]g9Assumption,0,len(all))
	for id,a:=range all {
		if !retract[id] { out=append(out,a) }
	}
	return out,best
}

func g9IndependentClosure(assumptions []g9Assumption, rules []g9Rule) map[string]bool {
	known:=map[string]bool{}
	for _,a:=range assumptions { known[a.Literal]=true }
	for changed:=true; changed; {
		changed=false
		for _,r:=range rules {
			satisfied:=true
			for _,a:=range r.Antecedents {
				if !known[a] { satisfied=false; break }
			}
			if satisfied && !known[r.Consequent] { known[r.Consequent]=true; changed=true }
		}
	}
	return known
}

func g9RandomScenario(seed int64) ([]g9Assumption, []g9Rule, string, int) {
	r:=rand.New(rand.NewSource(seed))
	atoms:=[]string{"p","q","r","s","t","u"}
	base:=make([]g9Assumption,0,7)
	for i:=0;i<7;i++ {
		atom:=atoms[r.Intn(len(atoms))]
		if r.Intn(2)==1 { atom="!"+atom }
		cost:=1+r.Intn(9)
		a:=g9Assumption{ID:"a"+string(rune('0'+i)),Literal:atom,Cost:cost}
		base=append(base,a)
	}
	rules:=make([]g9Rule,0,8)
	for i:=1;i<6;i++ {
		antAtom:=atoms[i-1]
		consAtom:=atoms[i]
		if r.Intn(2)==1 { antAtom="!"+antAtom }
		if r.Intn(2)==1 { consAtom="!"+consAtom }
		arity:=1+r.Intn(2)
		ants:=[]string{antAtom}
		if arity==2 {
			second:=atoms[r.Intn(i)]
			if r.Intn(2)==1 { second="!"+second }
			if second==antAtom { second=g9Neg(second) }
			ants=append(ants,second)
		}
		rules=append(rules,g9Rule{Antecedents:ants,Consequent:consAtom})
	}
	// Add one direct dependency chain to make provenance and retraction nontrivial.
	rules=append(rules,
		g9Rule{Antecedents:[]string{"p"},Consequent:"q"},
		g9Rule{Antecedents:[]string{"q"},Consequent:"r"},
	)
	return base,rules,"",0
}

func g9FindContradictoryEvidence(base []g9Assumption, rules []g9Rule) (string,bool) {
	closure:=g9IndependentClosure(base,rules)
	cands:=make([]string,0)
	for lit:=range closure {
		if !closure[g9Neg(lit)] { cands=append(cands,lit) }
	}
	sort.Strings(cands)
	for _,lit:=range cands {
		ev:=g9Assumption{ID:"probe",Literal:g9Neg(lit),Cost:0,Hard:true}
		if g9Oracle(base,ev,rules).OK { return ev.Literal,true }
	}
	return "",false
}

func g9IDs(xs []g9Assumption) []string {
	out:=make([]string,0,len(xs))
	for _,a:=range xs { out=append(out,a.ID) }
	sort.Strings(out)
	return out
}

func g9WriteReport(name string,v any) {
	ws:=os.Getenv("GITHUB_WORKSPACE"); if ws=="" { return }
	b,_:=json.MarshalIndent(v,"","  ")
	_ = os.WriteFile(filepath.Join(ws,name),append(b,'\n'),0644)
}

func TestG9AssumptionBasedBeliefRevision(t *testing.T) {
	const seeds=64
	report:=g9RevisionReport{}
	for seed:=1;seed<=seeds;seed++ {
		base,rules,_,_:=g9RandomScenario(int64(930000+seed))
		// Regenerate until the base is consistent and contains enough derived structure.
		for tries:=0; tries<40 && (!g9Consistent(base,rules)); tries++ {
			base,rules,_,_=g9RandomScenario(int64(930000+seed+tries+1000))
		}
		if !g9Consistent(base,rules) { t.Fatalf("seed %d generated inconsistent base",seed) }
		active:=append([]g9Assumption(nil),base...)
		for update:=0;update<4;update++ {
			evidenceLiteral,ok:=g9FindContradictoryEvidence(active,rules)
			if !ok {
				evidenceLiteral=[]string{"p","!p","q","!q"}[(seed+update)%4]
			}
			evidence:=g9Assumption{ID:"e"+string(rune('0'+update)),Literal:evidenceLiteral,Cost:0,Hard:true}
			oracle:=g9Oracle(active,evidence,rules)
			updated,got:=g9Revise(active,evidence,rules)
			if !oracle.OK || !got.OK { t.Fatalf("seed %d update %d had no revision solution",seed,update) }

			if got.Cost!=oracle.Cost { t.Fatalf("seed %d update %d nonminimal revision: got=%d oracle=%d",seed,update,got.Cost,oracle.Cost) }
			report.OptimalPasses++

			if !g9Consistent(updated,rules) { t.Fatalf("seed %d update %d inconsistent revised state",seed,update) }
			report.ConsistencyPasses++

			foundEvidence:=false
			for _,a:=range updated { if a.ID==evidence.ID && a.Literal==evidence.Literal {foundEvidence=true} }
			if !foundEvidence { t.Fatalf("seed %d update %d revised state dropped hard evidence",seed,update) }
			report.SuccessPasses++

			retracted:=map[int]bool{}
			for _,id:=range got.IDs { retracted[id]=true }
			union := map[int]bool{}
			all:=append(append([]g9Assumption(nil),active...),evidence)
			for _,ng:=range g9Nogoods(g9Labels(all,rules)) {
				for _,id:=range ng {
					if id<len(active) { union[id]=true }
				}
			}
			for _,id:=range got.IDs {
				if !union[id] { t.Fatalf("seed %d update %d over-retracted assumption %d",seed,update,id) }
			}
			// Hard evidence must never be retractable.
			for _,id:=range got.IDs {
				if id>=len(active) { t.Fatalf("seed %d update %d retracted hard evidence",seed,update) }
			}
			report.NoOverRetractionPasses++

			shuffled:=append([]g9Assumption(nil),active...)
			rr:=rand.New(rand.NewSource(int64(940000+seed*10+update)))
			rr.Shuffle(len(shuffled),func(i,j int){shuffled[i],shuffled[j]=shuffled[j],shuffled[i]})
			rules2:=append([]g9Rule(nil),rules...)
			rr.Shuffle(len(rules2),func(i,j int){rules2[i],rules2[j]=rules2[j],rules2[i]})
			shuffledEvidence:=evidence
			alt,altBest:=g9Revise(shuffled,shuffledEvidence,rules2)
			if altBest.Cost!=got.Cost || len(g9IDs(alt))!=len(g9IDs(updated)) {
				t.Fatalf("seed %d update %d order changed revision result",seed,update)
			}
			report.OrderInvariantPasses++
			active=updated
			report.Updates++
		}
	}

	// Demonstrate why revision is necessary: at least one generated update must
	// have contradicted the append-only state.
	for seed:=1;seed<=seeds;seed++ {
		base,rules,_,_:=g9RandomScenario(int64(950000+seed))
		if !g9Consistent(base,rules) { continue }
		target,ok:=g9FindContradictoryEvidence(base,rules)
		if !ok { continue }
		ev:=g9Assumption{ID:"naive",Literal:g9Neg(target),Cost:0,Hard:true}
		if !g9Consistent(append(append([]g9Assumption(nil),base...),ev),rules) { report.NaiveInconsistencies++ }
	}
	class:="G9_NOT_PROVEN"
	want:=seeds*4
	if report.Updates==want &&
		report.OptimalPasses==want &&
		report.ConsistencyPasses==want &&
		report.SuccessPasses==want &&
		report.NoOverRetractionPasses==want &&
		report.OrderInvariantPasses==want &&
		report.NaiveInconsistencies>0 {
		class="G9_ASSUMPTION_BASED_BELIEF_REVISION_PROVEN"
	}
	report.Classification=class
	g9WriteReport("ACE_G9_BELIEF_REVISION.json",report)
	t.Logf("G9 report=%+v",report)
	if class!="G9_ASSUMPTION_BASED_BELIEF_REVISION_PROVEN" { t.Fatalf("G9 failed: %+v",report) }
}
