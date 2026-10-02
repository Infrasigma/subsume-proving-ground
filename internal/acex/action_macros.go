package acex

import "errors"

type ActionMacro struct {
	ID      string
	Actions []string
	Uses    int
}

func commonActionMacros(traces [][]string, minLen int) []ActionMacro {
	if len(traces) < 2 || minLen < 2 {
		return nil
	}
	base := traces[0]
	out := make([]ActionMacro,0)
	for start:=0; start<len(base); start++ {
		for length:=minLen; start+length<=len(base); length++ {
			cand:=append([]string(nil),base[start:start+length]...)
			uses:=1
			for ti:=1; ti<len(traces); ti++ {
				found:=false
				for j:=0;j+length<=len(traces[ti]);j++ {
					ok:=true
					for k:=0;k<length;k++ {
						if traces[ti][j+k]!=cand[k] { ok=false; break }
					}
					if ok { found=true; break }
				}
				if found { uses++ }
			}
			if uses>=2 {
				out=append(out,ActionMacro{
					ID: "action-macro-"+string(rune('A'+len(out))),
					Actions:cand,Uses:uses,
				})
			}
		}
	}
	return out
}

func PredictMacro(m *PredictiveModel, start NumericState, macro ActionMacro) (NumericState,float64,error) {
	if m==nil || len(macro.Actions)==0 { return nil,0,errors.New("invalid macro") }
	cur:=copyState(start)
	conf:=1.0
	for _,a:=range macro.Actions {
		p:=m.Predict(cur,a)
		if !p.Known || p.Confidence<0.5 {
			return nil,0,errors.New("macro contains unverified action")
		}
		cur=p.State
		if p.Confidence<conf { conf=p.Confidence }
	}
	return cur,conf,nil
}

func PlanWithActionLibrary(m *PredictiveModel,start NumericState,macros []ActionMacro,atomic []string,goal Goal,horizon int)(PlanResult,error) {
	if m==nil || goal==nil { return PlanResult{},errors.New("invalid planning inputs") }
	type node struct{ state NumericState; path []string; conf float64; cost int }
	q:=[]node{{state:copyState(start),conf:1}}
	for depth:=0;depth<=horizon;depth++ {
		next:=make([]node,0,len(q)*(len(atomic)+len(macros)))
		for _,n:=range q {
			if goal(n.state) {
				return PlanResult{Actions:n.path,State:n.state,Cost:n.cost,Confidence:n.conf},nil
			}
			for _,a:=range atomic {
				p:=m.Predict(n.state,a)
				if !p.Known || p.Confidence<0.5 { continue }
				path:=append(append([]string(nil),n.path...),a)
				next=append(next,node{state:p.State,path:path,conf:minFloat(n.conf,p.Confidence),cost:n.cost+1})
			}
			for _,macro:=range macros {
				s,c,err:=PredictMacro(m,n.state,macro)
				if err!=nil || c<0.5 { continue }
				path:=append(append([]string(nil),n.path...),macro.ID)
				next=append(next,node{state:s,path:path,conf:minFloat(n.conf,c),cost:n.cost+1})
			}
		}
		q=next
	}
	return PlanResult{},errors.New("no plan")
}

func minFloat(a,b float64) float64 {
	if a<b { return a }
	return b
}
