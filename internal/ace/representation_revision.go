package ace

import (
    "errors"
    "sort"
    "strings"
)

type Representation struct { Variables []string; Relations []string }
type RepresentationCandidate struct { Representation Representation; Score float64; ResidualBefore, ResidualAfter int; EvidenceIDs []string; Reason string }

func projectRepresentation(s State,r Representation) string {parts:=make([]string,0,len(r.Variables)+len(r.Relations));for _,k:=range r.Variables{parts=append(parts,"v:"+k+"="+s.Values[k])};for _,rel:=range r.Relations{p:=strings.Split(rel,"=");if len(p)!=2{continue};parts=append(parts,"r:"+p[0]+"="+boolString(s.Values[p[0]]==s.Values[p[1]]))};sort.Strings(parts);return strings.Join(parts,"|")}
func boolString(x bool)string{if x{return "true"};return "false"}
func residualCount(xs []Experience,r Representation)int{type bucket map[string]bool;b:=map[string]bucket{};for _,x:=range xs{tr,e:=DecodeTransition(x);if e!=nil{continue};key:=projectRepresentation(tr.Before,r)+"|op:"+tr.Action.Operation;if b[key]==nil{b[key]=bucket{}};b[key][stateSignature(tr.After)]=true};n:=0;for _,o:=range b{if len(o)>1{n++}};return n}
func observedVariables(xs []Experience)[]string{m:=map[string]bool{};for _,x:=range xs{tr,e:=DecodeTransition(x);if e!=nil{continue};for k:=range tr.Before.Values{m[k]=true};for k:=range tr.After.Values{m[k]=true}};out:=make([]string,0,len(m));for k:=range m{out=append(out,k)};sort.Strings(out);return out}
func containsString(xs []string,x string)bool{for _,y:=range xs{if y==x{return true}};return false}

// ProposeRepresentationRevisions turns prediction residuals into competing
// representation hypotheses. It does not know which field is causal: every
// currently unrepresented observed variable is a candidate, and pairwise
// equality relations are additional candidates when enough evidence exists.
func ProposeRepresentationRevisions(xs []Experience,current Representation) ([]RepresentationCandidate,error){if len(xs)<2{return nil,errors.New("representation revision requires multiple transitions")};base:=residualCount(xs,current);if base==0{return nil,nil};vars:=observedVariables(xs);var out []RepresentationCandidate;for _,v:=range vars{if containsString(current.Variables,v){continue};r:=Representation{Variables:append(append([]string{},current.Variables...),v),Relations:append([]string{},current.Relations...)};after:=residualCount(xs,r);out=append(out,RepresentationCandidate{Representation:r,ResidualBefore:base,ResidualAfter:after,Score:float64(base-after),Reason:"new-variable distinction"})};for i:=0;i<len(vars);i++{for j:=i+1;j<len(vars);j++{rel:=vars[i]+"="+vars[j];if containsString(current.Relations,rel){continue};r:=Representation{Variables:append([]string{},current.Variables...),Relations:append(append([]string{},current.Relations...),rel)};after:=residualCount(xs,r);out=append(out,RepresentationCandidate{Representation:r,ResidualBefore:base,ResidualAfter:after,Score:float64(base-after),Reason:"new-relation distinction"})}};sort.SliceStable(out,func(i,j int)bool{if out[i].Score==out[j].Score{return out[i].Reason<out[j].Reason};return out[i].Score>out[j].Score});return out,nil}

func SelectRepresentationRevision(xs []Experience,current Representation) (RepresentationCandidate,error){cs,e:=ProposeRepresentationRevisions(xs,current);if e!=nil{return RepresentationCandidate{},e};if len(cs)==0{return RepresentationCandidate{},errors.New("no representation revision improves residuals")};best:=cs[0];if best.Score<=0{return RepresentationCandidate{},errors.New("no candidate improves residuals")};return best,nil}
