package acex

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

type V7Step struct {
	Before   string
	Action   string
	After    string
	Reward   float64
	Terminal bool
}

type V7ExperienceGraph struct {
	Nodes map[string]RelationalState
	Edges map[string][]V7Step
}

func NewV7ExperienceGraph() *V7ExperienceGraph {
	return &V7ExperienceGraph{
		Nodes: map[string]RelationalState{},
		Edges: map[string][]V7Step{},
	}
}

func V7StateKey(s RelationalState) string {
	return WLInvariant(s, 4)
}

func (g *V7ExperienceGraph) ObserveState(s RelationalState) string {
	k := V7StateKey(s)
	if _, ok := g.Nodes[k]; !ok {
		g.Nodes[k] = cloneRelationalState(s)
	}
	return k
}

func cloneRelationalState(s RelationalState) RelationalState {
	out := RelationalState{
		Nodes: make([]RelNode, len(s.Nodes)),
		Edges: make([]RelEdge, len(s.Edges)),
	}
	for i, n := range s.Nodes {
		out.Nodes[i] = RelNode{ID:n.ID, Kind:n.Kind, Attrs:map[string]string{}}
		for k,v := range n.Attrs { out.Nodes[i].Attrs[k] = v }
	}
	copy(out.Edges, s.Edges)
	return out
}

func (g *V7ExperienceGraph) AddStep(before RelationalState, action string, after RelationalState, reward float64, terminal bool) V7Step {
	bk := g.ObserveState(before)
	ak := g.ObserveState(after)
	step := V7Step{Before:bk, Action:action, After:ak, Reward:reward, Terminal:terminal}
	g.Edges[bk] = append(g.Edges[bk], step)
	return step
}

func V7KnownTransition(g *V7ExperienceGraph, stateKey, action string) (V7Step, bool) {
	for _, e := range g.Edges[stateKey] {
		if e.Action == action {
			return e, true
		}
	}
	return V7Step{}, false
}

func V7ActionEffectSignature(step V7Step) string {
	return hashString(step.Before+"|"+step.After+"|"+fmt.Sprintf("%.6f|%t", step.Reward, step.Terminal))
}

type V7Explorer struct {
	Attempts map[string]int
}

func (x *V7Explorer) Choose(stateKey string, actions []string, graph *V7ExperienceGraph) (string, error) {
	if len(actions) == 0 {
		return "", errors.New("no available actions")
	}
	if x.Attempts == nil {
		x.Attempts = map[string]int{}
	}
	type cand struct {
		action string
		score  int
	}
	cs := make([]cand, 0, len(actions))
	for _, action := range actions {
		score := 0
		if _, ok := V7KnownTransition(graph, stateKey, action); !ok {
			score += 1000
		}
		score -= x.Attempts[stateKey+"|"+action]
		cs = append(cs, cand{action, score})
	}
	sort.SliceStable(cs, func(i,j int) bool {
		if cs[i].score != cs[j].score { return cs[i].score > cs[j].score }
		return cs[i].action < cs[j].action
	})
	x.Attempts[stateKey+"|"+cs[0].action]++
	return cs[0].action, nil
}

type V7PlanResult struct {
	Actions  []string
	States   []string
	Verified bool
	Cost     int
}

func (g *V7ExperienceGraph) Plan(start string, goal func(V7Step) bool) (V7PlanResult, error) {
	type node struct {
		key   string
		path  []string
		state []string
	}
	q := []node{{key:start,state:[]string{start}}}
	seen := map[string]bool{start:true}
	for len(q) > 0 {
		cur := q[0]
		q = q[1:]
		for _, e := range g.Edges[cur.key] {
			if !goal(e) {
				continue
			}
			path := append(append([]string(nil),cur.path...),e.Action)
			states := append(append([]string(nil),cur.state...),e.After)
			return V7PlanResult{Actions:path,States:states,Verified:true,Cost:len(path)},nil
		}
		for _, e := range g.Edges[cur.key] {
			if seen[e.After] {
				continue
			}
			seen[e.After] = true
			q = append(q,node{
				key:e.After,
				path:append(append([]string(nil),cur.path...),e.Action),
				state:append(append([]string(nil),cur.state...),e.After),
			})
		}
	}
	return V7PlanResult{}, errors.New("no verified path to observed goal")
}

type V7Macro struct {
	ID      string
	Actions []string
	Uses    int
	Effect  string
}

func V7InduceMacros(traces [][]V7Step, minLen int) []V7Macro {
	if len(traces) < 2 || minLen < 2 {
		return nil
	}
	base := traces[0]
	type candidate struct { actions []string; effect string; uses int }
	seen := map[string]candidate{}
	for start := 0; start < len(base); start++ {
		for length := minLen; start+length <= len(base); length++ {
			actions := make([]string,length)
			effectParts := make([]string,length)
			for i := 0; i < length; i++ {
				actions[i] = base[start+i].Action
				effectParts[i] = V7ActionEffectSignature(base[start+i])
			}
			k := strings.Join(actions,"|")
			uses := 1
			for ti := 1; ti < len(traces); ti++ {
				for j := 0; j+length <= len(traces[ti]); j++ {
					ok := true
					for z := 0; z < length; z++ {
						if V7ActionEffectSignature(traces[ti][j+z]) != effectParts[z] {
							ok = false
							break
						}
					}
					if ok { uses++; break }
				}
			}
			if uses >= 2 {
				seen[k] = candidate{actions:actions,effect:strings.Join(effectParts,"|"),uses:uses}
			}
		}
	}
	out := make([]V7Macro,0,len(seen))
	for i,c := range seen {
		out = append(out,V7Macro{
			ID:"v7macro-"+hashString(i)[:10],
			Actions:append([]string(nil),c.actions...),
			Uses:c.uses,
			Effect:c.effect,
		})
	}
	sort.SliceStable(out,func(i,j int)bool {
		if len(out[i].Actions)!=len(out[j].Actions) { return len(out[i].Actions)>len(out[j].Actions) }
		return out[i].ID<out[j].ID
	})
	return out
}

func V7ApplyMacro(actions []string, macro V7Macro) []string {
	if len(macro.Actions)==0 { return append([]string(nil),actions...) }
	out := append([]string(nil),actions...)
	for i := 0; i+len(macro.Actions) <= len(out); {
		match := true
		for j := range macro.Actions {
			if out[i+j] != macro.Actions[j] { match=false; break }
		}
		if !match { i++; continue }
		repl := append([]string(nil),macro.ID)
		next := make([]string,0,len(out)-len(macro.Actions)+1)
		next = append(next,out[:i]...)
		next = append(next,repl...)
		next = append(next,out[i+len(macro.Actions):]...)
		out = next
		i += len(repl)
	}
	return out
}

type V7GoalHypothesis struct {
	StateKey string
	Reward   float64
	Terminal bool
	Evidence int
}

type V7CognitiveAgent struct {
	Graph       *V7ExperienceGraph
	Explorer    V7Explorer
	Goals       []V7GoalHypothesis
	Macros      []V7Macro
	Failures    []string
	Steps       int
}

func NewV7CognitiveAgent() *V7CognitiveAgent {
	return &V7CognitiveAgent{Graph:NewV7ExperienceGraph(),Explorer:V7Explorer{}}
}

func (a *V7CognitiveAgent) RecordExperience(step V7Step) {
	for i := range a.Goals {
		if a.Goals[i].StateKey == step.After {
			a.Goals[i].Evidence++
			if step.Reward > a.Goals[i].Reward { a.Goals[i].Reward = step.Reward }
			a.Goals[i].Terminal = a.Goals[i].Terminal || step.Terminal
			return
		}
	}
	if step.Reward > 0 || step.Terminal {
		a.Goals = append(a.Goals,V7GoalHypothesis{
			StateKey:step.After,Reward:step.Reward,Terminal:step.Terminal,Evidence:1,
		})
	}
}

func (a *V7CognitiveAgent) InferGoal() (V7GoalHypothesis, bool) {
	if len(a.Goals)==0 { return V7GoalHypothesis{},false }
	sort.SliceStable(a.Goals,func(i,j int)bool {
		if a.Goals[i].Terminal != a.Goals[j].Terminal { return a.Goals[i].Terminal }
		if a.Goals[i].Reward != a.Goals[j].Reward { return a.Goals[i].Reward > a.Goals[j].Reward }
		return a.Goals[i].Evidence > a.Goals[j].Evidence
	})
	return a.Goals[0],true
}

func (a *V7CognitiveAgent) NextAction(state RelationalState, actions []string) (string,error) {
	key := a.Graph.ObserveState(state)
	if goal,ok := a.InferGoal(); ok && goal.StateKey != key {
		if p,err := a.Graph.Plan(key,func(e V7Step)bool {
			return e.After==goal.StateKey && (e.Reward>0 || e.Terminal)
		}); err==nil && len(p.Actions)>0 {
			return p.Actions[0],nil
		}
	}
	return a.Explorer.Choose(key,actions,a.Graph)
}

func (a *V7CognitiveAgent) ExecuteObserved(before RelationalState, action string, after RelationalState, reward float64, terminal bool) V7Step {
	step:=a.Graph.AddStep(before,action,after,reward,terminal)
	a.RecordExperience(step)
	a.Steps++
	return step
}

func (a *V7CognitiveAgent) ObserveUnexpected(before RelationalState, action string, expectedAfter string, actualAfter RelationalState) error {
	actual:=V7StateKey(actualAfter)
	if actual==expectedAfter { return nil }
	a.Failures=append(a.Failures,fmt.Sprintf("transition-surprise:%s:%s",action,expectedAfter+"!="+actual))
	a.Graph.ObserveState(before)
	return nil
}

func (a *V7CognitiveAgent) Consolidate(traces [][]V7Step) {
	a.Macros = V7InduceMacros(traces,2)
}

func V7FindTransferredAction(source V7Step, targetBefore RelationalState, targetEdges []V7Step) (string,bool) {
	want:=V7ActionEffectSignature(source)
	for _, e := range targetEdges {
		_ = targetBefore
		if V7ActionEffectSignature(e)==want {
			return e.Action,true
		}
	}
	return "",false
}
