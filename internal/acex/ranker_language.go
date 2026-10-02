package acex

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
)

type RankExpr struct {
	Kind       string
	Value      float64
	Left,Right *RankExpr
}

type RankerProgram struct {
	Expr *RankExpr
}

func rankMetric(name string, p FeatureProfile) float64 {
	switch name {
	case "balance":
		return p.Balance
	case "positive":
		return p.Positive
	case "negative":
		return p.Negative
	case "frequency":
		return p.Positive + p.Negative
	case "cooccurrence":
		return float64(len(p.Cooccurrence))
	default:
		return 0
	}
}

func evalRankExpr(e *RankExpr, p FeatureProfile) float64 {
	if e == nil {
		return 0
	}
	switch e.Kind {
	case "metric":
		return rankMetric(strconv.FormatFloat(e.Value,'f',-1,64),p)
	case "const":
		return e.Value
	case "add":
		return evalRankExpr(e.Left,p)+evalRankExpr(e.Right,p)
	case "mul":
		return evalRankExpr(e.Left,p)*evalRankExpr(e.Right,p)
	case "neg":
		return -evalRankExpr(e.Left,p)
	default:
		return 0
	}
}

func (p RankerProgram) Signature() string {
	var walk func(*RankExpr) string
	walk = func(e *RankExpr) string {
		if e==nil { return "" }
		if e.Kind=="metric" { return "m:"+strconv.FormatFloat(e.Value,'f',-1,64) }
		if e.Kind=="const" { return "c:"+strconv.FormatFloat(e.Value,'f',-1,64) }
		return e.Kind+"("+walk(e.Left)+","+walk(e.Right)+")"
	}
	return walk(p.Expr)
}

func (p RankerProgram) Order(data Dataset) []string {
	profile := Profile(data)
	names := candidateFeatures(data)
	sort.SliceStable(names,func(i,j int) bool{
		a:=evalRankExpr(p.Expr,profile[names[i]])
		b:=evalRankExpr(p.Expr,profile[names[j]])
		if a!=b { return a>b }
		return names[i]<names[j]
	})
	return names
}

func leafRankPrograms() []RankerProgram {
	out:=make([]RankerProgram,0,5)
	for _,name:=range []string{"balance","positive","negative","frequency","cooccurrence"} {
		out=append(out,RankerProgram{Expr:&RankExpr{Kind:"metric",Value:metricID(name)}})
	}
	return out
}

func metricID(name string) float64 {
	switch name {
	case "balance": return 1
	case "positive": return 2
	case "negative": return 3
	case "frequency": return 4
	default: return 5
	}
}

func metricName(id float64) string {
	switch int(id) {
	case 1: return "balance"
	case 2: return "positive"
	case 3: return "negative"
	case 4: return "frequency"
	default: return "cooccurrence"
	}
}

func evalRankExprFixed(e *RankExpr,p FeatureProfile) float64 {
	if e==nil { return 0 }
	switch e.Kind {
	case "metric":
		return rankMetric(metricName(e.Value),p)
	case "const": return e.Value
	case "add": return evalRankExprFixed(e.Left,p)+evalRankExprFixed(e.Right,p)
	case "mul": return evalRankExprFixed(e.Left,p)*evalRankExprFixed(e.Right,p)
	case "neg": return -evalRankExprFixed(e.Left,p)
	default: return 0
	}
}

func (p RankerProgram) OrderFixed(data Dataset) []string {
	profile:=Profile(data)
	names:=candidateFeatures(data)
	sort.SliceStable(names,func(i,j int)bool{
		a,b:=evalRankExprFixed(p.Expr,profile[names[i]]),evalRankExprFixed(p.Expr,profile[names[j]])
		if a!=b { return a>b }
		return names[i]<names[j]
	})
	return names
}

func DiscoverWithOrder(train,holdout Dataset,ordered []string,maxAtoms int)(Concept,Resource,error){
	if maxAtoms<1 { return Concept{},Resource{},errors.New("maxAtoms must be positive") }
	best:=Concept{}
	bestScore:=-1.0
	cost:=Resource{}
	limit:=maxAtoms
	if len(ordered)<limit { limit=len(ordered) }
	for k:=1;k<=limit;k++ {
		for _,c:=range combinations(ordered,k) {
			cost.Search++
			if Accuracy(train,c)<0.90 { continue }
			cost.Verify++
			ha:=Accuracy(holdout,c)
			if ha<0.90 { continue }
			score:=ha-0.01*float64(k)
			if score>bestScore {
				bestScore=score
				best=Concept{ID:stringsJoin(c),Features:append([]string(nil),c...),Support:Support(train,c),Accuracy:ha,Complexity:k}
			}
		}
	}
	if best.ID=="" { return Concept{},cost,errors.New("no verified concept discovered") }
	cost.Memory=best.Complexity
	cost.Storage=len(best.Features)
	return best,cost,nil
}

func stringsJoin(xs []string) string {
	out:=""
	for i,x:=range xs {
		if i>0 { out+="+" }
		out+=x
	}
	return out
}

func enumerateRankerPrograms(depth int) []RankerProgram {
	leaves:=leafRankPrograms()
	if depth<=0 { return leaves }
	out:=append([]RankerProgram(nil),leaves...)
	constants:=[]RankerProgram{
		{Expr:&RankExpr{Kind:"const",Value:1}},
		{Expr:&RankExpr{Kind:"const",Value:0.5}},
	}
	all:=append(append([]RankerProgram(nil),leaves...),constants...)
	seen:=map[string]bool{}
	for _,p:=range all { seen[p.Signature()]=true }
	for _,a:=range all {
		for _,b:=range all {
			for _,kind:=range []string{"add","mul"} {
				p:=RankerProgram{Expr:&RankExpr{Kind:kind,Left:a.Expr,Right:b.Expr}}
				if !seen[p.Signature()] { seen[p.Signature()]=true; out=append(out,p) }
			}
		}
	}
	return out
}

type RankerSearchResult struct {
	Program RankerProgram
	Baseline int
	Improved int
}

func SearchRankerProgram(train,holdout []Dataset,current RankerProgram)(RankerSearchResult,error){
	base:=0
	for i:=range train {
		ordered:=candidateFeatures(train[i])
		_,c,e:=DiscoverWithOrder(train[i],holdout[i],ordered,4)
		if e!=nil { return RankerSearchResult{},e }
		base+=c.Total()
	}
	best:=RankerSearchResult{Program:current,Baseline:base,Improved:base}
	bestAny:=base
	bestAnySig:=""
	for _,p:=range enumerateRankerPrograms(1) {
		if p.Signature()==current.Signature() { continue }
		cost:=0
		ok:=true
		for i:=range train {
			_,c,e:=DiscoverWithOrder(train[i],holdout[i],p.OrderFixed(train[i]),4)
			if e!=nil { ok=false; break }
			cost+=c.Total()
		}
		if ok {
			if cost<bestAny { bestAny=cost; bestAnySig=p.Signature() }
			if cost<best.Improved { best=RankerSearchResult{Program:p,Baseline:base,Improved:cost} }
		}
	}
	if best.Improved>=best.Baseline {
		return RankerSearchResult{},fmt.Errorf("no verified ranker improvement: baseline=%d best_any=%d best_signature=%s",base,bestAny,bestAnySig)
	}
	return best,nil
}

func EnsureFiniteRanker(p RankerProgram) error {
	for _,x:=range []float64{1,2,3,4,5} {
		if math.IsNaN(evalRankExprFixed(p.Expr,FeatureProfile{Positive:x,Negative:1-x})) { return errors.New("ranker produced NaN") }
	}
	return nil
}
