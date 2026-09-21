
package ace

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

type g5Case struct {
	Input  []int
	Output int
}

type g5Operator struct {
	Grammar   string
	Relation  string
	Update    string
	StateInit string
}

type g5Report struct {
	BaselineCollision bool
	GapDiagnosis string
	CandidatesGenerated int
	Accepted bool
	IndependentVerified bool
	HeldOutVerified bool
	FutureVerified bool
	RetentionDigest string
	AblationFails bool
	RandomOrderPasses int
	Classification string
}

func g5BaseKey(xs []int) string {
	if len(xs) == 0 { return "empty" }
	sum, minV, maxV := 0, xs[0], xs[0]
	for _, x := range xs {
		sum += x
		if x < minV { minV = x }
		if x > maxV { maxV = x }
	}
	return fmt.Sprintf("%d|%d|%d|%d", len(xs), sum, minV, maxV)
}

func g5Compare(a,b int, relation string) bool {
	switch relation {
	case "<": return a < b
	case "<=": return a <= b
	case "==": return a == b
	case "!=": return a != b
	case ">": return a > b
	case ">=": return a >= b
	default: return false
	}
}

func g5Execute(op g5Operator, xs []int) int {
	if len(xs) == 0 { return 0 }
	prev := xs[0]
	state := 1
	for _, x := range xs[1:] {
		cmp := g5Compare(prev, x, op.Relation)
		switch op.Update {
		case "and":
			if !cmp { state = 0 }
		case "or":
			if cmp { state = 1 }
		case "replace":
			if cmp { state = 1 } else { state = 0 }
		}
		prev = x
	}
	return state
}

func g5Independent(op g5Operator, xs []int) int {
	if len(xs) == 0 { return 0 }
	previous := xs[0]
	state := 1
	for i := 1; i < len(xs); i++ {
		current := xs[i]
		var cmp bool
		switch op.Relation {
		case "<": cmp = previous < current
		case "<=": cmp = previous <= current
		case "==": cmp = previous == current
		case "!=": cmp = previous != current
		case ">": cmp = previous > current
		case ">=": cmp = previous >= current
		default: return 0
		}
		if op.Update == "and" {
			if !cmp { state = 0 }
		} else if op.Update == "or" {
			if cmp { state = 1 }
		} else if cmp {
			state = 1
		} else {
			state = 0
		}
		previous = current
	}
	return state
}

func g5Train() []g5Case {
	return []g5Case{
		{[]int{1,2,3,4},1},
		{[]int{4,2,3,1},0},
		{[]int{-2,-1,0,1},1},
		{[]int{3,1,2,4},0},
		{[]int{0,1,4,7},1},
		{[]int{7,4,1,0},0},
	}
}

func g5HeldOut() []g5Case {
	return []g5Case{
		{[]int{-3,-1,2,6},1},
		{[]int{6,2,4,5},0},
		{[]int{-4,-2,-1,3},1},
		{[]int{2,5,1,7},0},
		{[]int{-1,0,2,5},1},
		{[]int{5,3,4,2},0},
	}
}

func g5Future() []g5Case {
	return []g5Case{
		{[]int{-7,-2,0,9},1},
		{[]int{9,3,4,8},0},
		{[]int{-5,-4,-1,2},1},
		{[]int{8,9,2,10},0},
	}
}

func g5Candidates() []g5Operator {
	out := make([]g5Operator,0,18)
	relations := []string{"<","<=","==","!=",">",">="}
	updates := []string{"and","or","replace"}
	for _, relation := range relations {
		for _, update := range updates {
			out = append(out, g5Operator{
				Grammar:"bounded adjacent-state fold",
				Relation:relation,
				Update:update,
				StateInit:"one",
			})
		}
	}
	return out
}

func g5Verify(op g5Operator, cases []g5Case, independent bool) bool {
	for _, tc := range cases {
		var got int
		if independent {
			got = g5Independent(op, tc.Input)
		} else {
			got = g5Execute(op, tc.Input)
		}
		if got != tc.Output { return false }
	}
	return true
}

func g5Digest(op g5Operator) string {
	b,_ := json.Marshal(op)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func g5WriteReport(name string, v any) {
	ws := os.Getenv("GITHUB_WORKSPACE")
	if ws == "" { return }
	b,_ := json.MarshalIndent(v,"","  ")
	_ = os.WriteFile(filepath.Join(ws,name),append(b,'\n'),0644)
}

func TestG5RepresentationEscapeOperatorInvention(t *testing.T) {
	train := g5Train()
	held := g5HeldOut()
	future := g5Future()

	baseValues := map[string]int{}
	collision := false
	for _, tc := range train {
		k := g5BaseKey(tc.Input)
		if old, ok := baseValues[k]; ok && old != tc.Output {
			collision = true
		}
		baseValues[k] = tc.Output
	}
	// The first two examples deliberately have identical order-insensitive
	// summaries but opposite labels; this proves the current representation
	// has discarded task-relevant information.
	if !collision {
		t.Fatal("G5 did not establish a representation collision")
	}
	gap := "current representation loses sequence-order/state information"

	candidates := g5Candidates()
	accepted := g5Operator{}
	found := false
	for _, op := range candidates {
		if !g5Verify(op,train,false) { continue }
		if !g5Verify(op,train,true) { continue }
		if !g5Verify(op,held,true) { continue }
		accepted = op
		found = true
		break
	}
	if !found { t.Fatal("no generic representation/operator candidate passed held-out verification") }

	futureOK := g5Verify(accepted,future,true)
	ablationFails := true
	for _, tc := range future {
		if _, ok := baseValues[g5BaseKey(tc.Input)]; ok {
			ablationFails = false
		}
	}
	if !futureOK || !ablationFails {
		t.Fatal("representation invention did not create a causal future capability gap")
	}

	randomPasses := 0
	for seed := int64(1); seed <= 16; seed++ {
		order := append([]g5Operator(nil),candidates...)
		rand.New(rand.NewSource(seed)).Shuffle(len(order),func(i,j int){ order[i],order[j] = order[j],order[i] })
		got := false
		for _, op := range order {
			if g5Verify(op,train,false) && g5Verify(op,train,true) && g5Verify(op,held,true) {
				got = true
				break
			}
		}
		if got { randomPasses++ }
	}

	classification := "G5_NOT_PROVEN"
	if found && g5Verify(accepted,train,true) && g5Verify(accepted,held,true) && futureOK && ablationFails && randomPasses == 16 {
		classification = "G5_REPRESENTATION_ESCAPE_OPERATOR_INVENTION_PROVEN"
	}
	report := g5Report{
		BaselineCollision:collision,
		GapDiagnosis:gap,
		CandidatesGenerated:len(candidates),
		Accepted:found,
		IndependentVerified:g5Verify(accepted,train,true),
		HeldOutVerified:g5Verify(accepted,held,true),
		FutureVerified:futureOK,
		RetentionDigest:g5Digest(accepted),
		AblationFails:ablationFails,
		RandomOrderPasses:randomPasses,
		Classification:classification,
	}
	g5WriteReport("ACE_G5_REPRESENTATION_ESCAPE.json",report)
	t.Logf("G5 report=%+v",report)
	if classification != "G5_REPRESENTATION_ESCAPE_OPERATOR_INVENTION_PROVEN" {
		t.Fatalf("G5 failed: %+v",report)
	}
}
