from __future__ import annotations

import copy, json, math, random
from dataclasses import dataclass, asdict, field
from hashlib import sha256
from typing import Any, Dict, Iterable, List, Mapping, Optional, Sequence, Tuple

Json = Any

def digest(x: Any) -> str:
    return sha256(json.dumps(x, sort_keys=True, separators=(",", ":"), default=str).encode()).hexdigest()

def ast_depth(e: Mapping[str, Any]) -> int:
    op = e["op"]
    if op in {"const","get","macro"}: return 1
    if op in {"abs","neg","threshold"}: return 1 + ast_depth(e["arg"])
    return 1 + max(ast_depth(e["left"]), ast_depth(e["right"]))

def ast_nodes(e: Mapping[str, Any]) -> int:
    op=e["op"]
    if op in {"const","get","macro"}: return 1
    if op in {"abs","neg","threshold"}: return 1 + ast_nodes(e["arg"])
    return 1 + ast_nodes(e["left"])+ast_nodes(e["right"])

def eval_expr(e: Mapping[str, Any], inputs: Mapping[str,float], macros: Mapping[str, Mapping[str,Any]]) -> float:
    op=e["op"]
    if op=="const": return float(e["value"])
    if op=="get": return float(inputs[e["key"]])
    if op=="macro": return eval_expr(macros[e["id"]], inputs, macros)
    if op=="abs": return abs(eval_expr(e["arg"],inputs,macros))
    if op=="neg": return -eval_expr(e["arg"],inputs,macros)
    if op=="threshold":
        x=eval_expr(e["arg"],inputs,macros)
        return 1.0 if x >= float(e["lo"]) else 0.0
    a=eval_expr(e["left"],inputs,macros); b=eval_expr(e["right"],inputs,macros)
    if op=="add": return a+b
    if op=="sub": return a-b
    if op=="mul": return a*b
    if op=="max": return max(a,b)
    if op=="min": return min(a,b)
    raise ValueError(op)

@dataclass(frozen=True)
class Trace:
    inputs: Dict[str,float]
    target: float
    family: str
    context: Dict[str,float]
    task_id: str

@dataclass(frozen=True)
class CognitivePrimitive:
    primitive_id: str
    expression: Mapping[str,Any]
    input_roles: Tuple[str,...]
    output_semantics: str
    generation: int
    parent_ids: Tuple[str,...]
    source_families: Tuple[str,...]
    train_error: float
    holdout_error: float
    transfer_error: float
    complexity: int
    provenance: Mapping[str,Any]
    status: str="candidate"

@dataclass(frozen=True)
class ProcessCandidate:
    process_id: str
    expression: Mapping[str,Any]
    used_primitives: Tuple[str,...]
    generation: int
    parent_ids: Tuple[str,...]
    search_policy: Mapping[str,Any]
    source_families: Tuple[str,...]
    complexity: int
    provenance: Mapping[str,Any]
    status: str="candidate"

@dataclass(frozen=True)
class Evaluation:
    train_error: float
    holdout_error: float
    transfer_error: float
    ood_error: float
    interference_harm: float
    resource_cost: float
    novelty: float
    accepted: bool

@dataclass
class ArchiveRecord:
    primitive: CognitivePrimitive
    utility_history: List[float]=field(default_factory=list)
    uses: int=0

class RecursiveCognitiveCompiler:
    """Model-independent recursive cognitive compiler.

    Components are explicit and ablatable:
    recursive trace conditioning; persistent archive/novelty search;
    editable meta-search policy; empirical experiment/selection; compatibility gating.
    """
    BASE_ATOMS=("get","const","add","sub","mul","max","min","abs","neg","threshold")

    def __init__(self, max_depth:int=2, population:int=24, seed:int=0):
        self.max_depth=max_depth
        self.population=population
        self.rng=random.Random(seed)
        self.generation=0
        self.recursion_depth=0
        self.meta_policy={"max_depth":max_depth,"novelty_weight":0.25,"exploration":0.50,"mutation_rate":0.30}
        self.primitives: Dict[str,CognitivePrimitive]={}
        self.archive: Dict[str,ArchiveRecord]={}
        self.processes: Dict[str,ProcessCandidate]={}
        self.process_frontier: List[str]=[]
        self.events: List[Dict[str,Any]]=[]
        self.active: Optional[str]=None
        self._expr_cache: Dict[Tuple[Tuple[str,...],Tuple[str,...],int],List[Mapping[str,Any]]] = {}
        self._executable_lineage_cache: Dict[str, Tuple[str, ...]] = {}

    def _expr_atoms(self, keys:Sequence[str], primitive_ids:Sequence[str])->List[Mapping[str,Any]]:
        return [
            *({"op":"get","key":k} for k in sorted(set(keys))),
            *({"op":"const","value":c} for c in (-2,-1,0,1,2,3,4)),
            *({"op":"macro","id":p} for p in primitive_ids),
        ]

    def _exprs(self, keys:Sequence[str], primitive_ids:Sequence[str], depth_limit:int)->List[Mapping[str,Any]]:
        cache_key=(tuple(sorted(keys)),tuple(sorted(primitive_ids)),int(depth_limit))
        if cache_key in self._expr_cache: return self._expr_cache[cache_key]
        atoms=self._expr_atoms(keys,primitive_ids)
        out=list(atoms)
        if depth_limit>=2:
            # Generic language closure: unary operators can compose with every
            # atomic expression, and binary operators combine atomic pairs.
            for a in atoms:
                for op in ("abs","neg"):
                    e={"op":op,"arg":a}
                    if ast_depth(e)<=2: out.append(e)
            for a in atoms:
                for b in atoms:
                    for op in ("add","sub","mul","max","min"):
                        e={"op":op,"left":a,"right":b}
                        if ast_depth(e)<=2: out.append(e)
        if depth_limit>=3:
            d2=[e for e in out if ast_depth(e)==2]
            for e in d2:
                for op in ("abs","neg"):
                    out.append({"op":op,"arg":e})
                for a in atoms:
                    for op in ("add","sub","mul","max","min"):
                        out.append({"op":op,"left":e,"right":a})
                        out.append({"op":op,"left":a,"right":e})
        macros=[a for a in atoms if a.get("op")=="macro"]
        consts=[a for a in atoms if a.get("op")=="const"]
        gets=[a for a in atoms if a.get("op")=="get"]
        front=[]
        # Preserve generic macro-macro compositions ahead of the global
        # 3000-expression insertion-order budget. This is not target-directed:
        # every pair of currently known macros receives the same operator set.
        for a in macros:
            for b in macros:
                for op in ("add","sub","mul","max","min"):
                    front.append({"op":op,"left":a,"right":b})
        for a in gets:
            for b in gets:
                front += [
                    {"op":"add","left":a,"right":b},
                    {"op":"sub","left":a,"right":b},
                    {"op":"mul","left":a,"right":b},
                ]
            for c in consts:
                front += [{"op":"mul","left":a,"right":c},{"op":"add","left":a,"right":c},{"op":"sub","left":a,"right":c}]
        pair_nodes=[]
        for a in gets:
            for b in gets:
                pair_nodes += [{"op":"add","left":a,"right":b},{"op":"sub","left":a,"right":b},{"op":"mul","left":a,"right":b}]
        for d in pair_nodes:
            for a in gets:
                front += [{"op":"add","left":d,"right":a},{"op":"add","left":a,"right":d},{"op":"sub","left":d,"right":a},{"op":"mul","left":d,"right":a}]
        for m in macros:
            for c in consts:
                front.extend([
                    {"op":"mul","left":m,"right":c},{"op":"mul","left":c,"right":m},
                    {"op":"add","left":m,"right":c},{"op":"sub","left":m,"right":c},
                ])
                for f in ({"op":"mul","left":m,"right":c},{"op":"add","left":m,"right":c}):
                    for c2 in consts[:4]:
                        front.append({"op":"add","left":f,"right":c2})
                        front.append({"op":"mul","left":f,"right":c2})
        vals=list({json.dumps(x,sort_keys=True):x for x in (front+out)}.values())[:3000]
        self._expr_cache[cache_key]=vals
        return vals

    def _walk(self,e:Mapping[str,Any])->Iterable[Mapping[str,Any]]:
        yield e
        if e["op"] in {"add","sub","mul","max","min"}:
            yield from self._walk(e["left"]); yield from self._walk(e["right"])
        elif e["op"] in {"abs","neg","threshold"}:
            yield from self._walk(e["arg"])

    def diagnose(self, traces:Sequence[Trace], failures:Mapping[str,float])->Tuple[str,...]:
        dx=[]
        if failures.get("search_saturation",0)>=0.8: dx.append("SEARCH_SATURATION")
        if failures.get("transfer_risk",0)>=0.7: dx.append("NEGATIVE_TRANSFER")
        if failures.get("context_ambiguity",0)>=0.7: dx.append("CONTEXT_AMBIGUITY")
        if failures.get("prediction_error",0)>=0.5: dx.append("PROCESS_MODEL_ERROR")
        if not dx: dx.append("LANGUAGE_COVERAGE_GAP")
        self.events.append({"event":"DIAGNOSIS","diagnoses":dx})
        return tuple(dx)

    def propose_search_policies(self)->Tuple[Mapping[str,Any],...]:
        p=self.meta_policy
        return (
            {"max_depth":min(4,int(p["max_depth"])+1),"novelty_weight":p["novelty_weight"],"exploration":min(1,p["exploration"]+0.15),"mutation_rate":p["mutation_rate"]},
            {"max_depth":int(p["max_depth"]),"novelty_weight":min(0.8,p["novelty_weight"]+0.20),"exploration":p["exploration"],"mutation_rate":min(0.8,p["mutation_rate"]+0.1)},
            {"max_depth":int(p["max_depth"]),"novelty_weight":max(0.0,p["novelty_weight"]-0.20),"exploration":max(0.05,p["exploration"]-0.15),"mutation_rate":max(0.05,p["mutation_rate"]-0.1)},
        )

    def experimentally_select_meta_policy(self, candidates:Sequence[Mapping[str,Any]], scores:Mapping[int,float])->Mapping[str,Any]:
        best=max(range(len(candidates)), key=lambda i: float(scores.get(i,-1e9)))
        chosen=dict(candidates[best]); old=self.meta_policy; self.meta_policy=chosen
        self.events.append({"event":"META_POLICY_SELECTED","before":old,"after":chosen,"scores":dict(scores)})
        return chosen

    def learn_role_map(self, source_rows: Sequence[Trace], target_rows: Sequence[Trace]) -> Dict[str,str]:
        def sig(rows, key):
            vals=[float(r.inputs[key]) for r in rows if key in r.inputs]
            if not vals: return (0,0,0,0)
            mu=sum(vals)/len(vals); var=sum((v-mu)**2 for v in vals)/len(vals)
            return (min(vals),max(vals),mu,math.sqrt(var))
        sk=sorted(source_rows[0].inputs); tk=sorted(target_rows[0].inputs)
        ss={k:sig(source_rows,k) for k in sk}; tt={k:sig(target_rows,k) for k in tk}
        remaining=set(tk); out={}
        for k in sk:
            best=min(remaining,key=lambda q: sum((ss[k][i]-tt[q][i])**2 for i in range(4)))
            out[best]=k; remaining.remove(best)
        self.events.append({"event":"ROLE_ALIGNMENT","mapping":out})
        return out

    def remap_traces(self, rows: Sequence[Trace], mapping: Mapping[str,str]) -> List[Trace]:
        return [Trace({mapping.get(k,k):v for k,v in r.inputs.items()},r.target,r.family,dict(r.context),r.task_id) for r in rows]

    def _pmap(self)->Dict[str,Mapping[str,Any]]:
        return {k:v.expression for k,v in self.primitives.items()}

    def primitive_lineage(self, primitive_id: str) -> Tuple[str, ...]:
        """Return the transitive executable lineage rooted at a primitive.

        Direct parent_ids are preserved exactly; lineage closure adds every
        ancestor primitive reachable through those parent edges. This is
        provenance bookkeeping only: it never changes expression semantics or
        search eligibility.
        """
        seen: set[str] = set()
        stack = [str(primitive_id)]
        while stack:
            pid = str(stack.pop())
            if pid in seen:
                continue
            seen.add(pid)
            primitive = self.primitives.get(pid)
            if primitive is None:
                continue
            stack.extend(str(parent) for parent in primitive.parent_ids)
            stack.extend(
                str(parent)
                for parent in primitive.provenance.get("lineage_ids", ())
            )
        return tuple(sorted(seen))

    def executable_lineage(self, primitive_id: str) -> Tuple[str, ...]:
        """Return lineage reached through actual executable macro references.

        Provenance-only alias metadata is excluded from executable lineage.
        The closure is memoized because primitive expressions are immutable
        after insertion, so this optimization preserves the exact lineage
        semantics while avoiding repeated graph walks during dominance tests.
        """
        root = str(primitive_id)
        cached = self._executable_lineage_cache.get(root)
        if cached is not None:
            return cached

        seen: set[str] = set()
        stack = [root]
        while stack:
            pid = str(stack.pop())
            if pid in seen:
                continue
            cached_child = self._executable_lineage_cache.get(pid)
            if cached_child is not None:
                seen.update(cached_child)
                continue
            seen.add(pid)
            primitive = self.primitives.get(pid)
            if primitive is None:
                continue
            for node in self._walk(primitive.expression):
                if node.get("op") == "macro":
                    stack.append(str(node["id"]))

        result = tuple(sorted(seen))
        # Cache every root requested. Newly inserted primitives cannot alter the
        # executable lineage of existing primitives because primitive expressions
        # are immutable and only reference primitives that already existed.
        self._executable_lineage_cache[root] = result
        return result

    def invent_primitive(self, train:Sequence[Trace], holdout:Sequence[Trace], transfer:Sequence[Trace], semantics:str="delta") -> Optional[CognitivePrimitive]:
        if len(train)<6: return None
        # Candidate selection uses only the permitted discovery/selection observations
        # in train. Holdout/transfer are measured after selection.
        keys=sorted(set().union(*(t.inputs.keys() for t in train)))
        existing=tuple(self.primitives)
        candidates=self._exprs(keys,existing,max(3,int(self.meta_policy["max_depth"])+1))
        pmap=self._pmap()
        def err(rows,expr):
            if not rows: return math.inf
            vals=[]
            for t in rows:
                try: vals.append(abs(eval_expr(expr,t.inputs,pmap)-t.target))
                except Exception: return math.inf
            return sum(vals)/len(vals)
        ranked=[]
        for expr in candidates:
            e1=err(train,expr)
            if e1>1e-9: continue
            macro_count=sum(1 for x in self._walk(expr) if x.get("op")=="macro")
            if self.generation>0 and self.primitives and macro_count<1: continue
            ranked.append((
                -macro_count,
                ast_nodes(expr),
                json.dumps(expr,sort_keys=True),
                expr,
            ))
        if not ranked: return None
        _,nodes,_,expr=min(ranked, key=lambda x:(x[0],x[1],x[2]))
        used=tuple(sorted({str(x["id"]) for x in self._walk(expr) if x.get("op")=="macro"}))
        train_error=err(train,expr)
        holdout_error=err(holdout,expr)
        transfer_error=err(transfer,expr)
        pid="prim:"+digest((expr,semantics))[:20]
        prim=CognitivePrimitive(
            pid,
            copy.deepcopy(expr),
            tuple(keys),
            semantics,
            self.generation,
            used,
            tuple(sorted({t.family for t in list(train)+list(holdout)+list(transfer)})),
            train_error,
            holdout_error,
            transfer_error,
            nodes,
            {
                "origin":"substrate_program_induction",
                "schema_given":False,
                "external_model":False,
                "selection_source":"discovery_and_selection_only",
                "validation_evaluated_after_selection":True,
            },
        )
        self.primitives[pid]=prim; self.archive[pid]=ArchiveRecord(prim)
        self.events.append({
            "event":"PRIMITIVE_INVENTED",
            "primitive_id":pid,
            "used_primitives":used,
            "selection_source":"discovery_and_selection_only",
        })
        return prim

    def synthesize_process_frontier(
        self,
        rows:Sequence[Trace],
        transfer:Sequence[Trace],
        ood:Sequence[Trace],
        *,
        max_candidates:int=32,
    )->Tuple[ProcessCandidate,...]:
        """Retain a bounded frontier using discovery/selection observations only.

        The legacy transfer and ood parameters remain only for API compatibility.
        They are deliberately ignored during candidate generation, ordering, and
        frontier admission and are evaluated only after a candidate is selected.
        """
        keys=sorted(set().union(*(t.inputs.keys() for t in rows))); pids=tuple(self.primitives)
        candidates=[]
        self.last_process_candidate_selection_summary = {
            "discovery_qualified": 0,
            "post_selection_validation_ignored": True,
        }
        for pol in self.propose_search_policies():
            for expr in self._exprs(keys,pids,min(3,int(pol["max_depth"]))):
                try:
                    tr=sum(abs(eval_expr(expr,t.inputs,self._pmap())-t.target) for t in rows)/len(rows)
                    if tr>1e-9: continue
                    used=tuple(sorted({str(x["id"]) for x in self._walk(expr) if x.get("op")=="macro"}))
                    selection_key=(
                        -len(used),
                        ast_nodes(expr),
                        len(json.dumps(expr,sort_keys=True)),
                        json.dumps(expr,sort_keys=True),
                    )
                    self.last_process_candidate_selection_summary["discovery_qualified"] += 1
                    candidates.append((selection_key,expr,used,pol))
                except Exception:
                    continue
        if not candidates:
            self.process_frontier=[]
            return ()

        best_by_lineage={}
        for selection_key,expr,used,pol in candidates:
            old=best_by_lineage.get(used)
            key=(selection_key,json.dumps(expr,sort_keys=True))
            if old is None or key < old[0]:
                best_by_lineage[used]=(key,expr,used,pol)

        variants=tuple(best_by_lineage.values())
        # Preserve lineage-distinct hypotheses under behavioral equivalence.
        frontier=list(variants)
        frontier.sort(key=lambda x:(x[0][0],json.dumps(x[1],sort_keys=True)))
        frontier=frontier[:max(1,int(max_candidates))]

        retained=[]
        for _,expr,used,pol in frontier:
            proc=ProcessCandidate(
                "proc:"+digest((expr,pol,self.generation))[:20],
                expr,
                used,
                self.generation,
                tuple(used),
                dict(pol),
                tuple(sorted({t.family for t in rows})),
                ast_nodes(expr),
                {
                    "origin":"substrate_process_synthesis_frontier",
                    "external_model":False,
                    "selection_source":"discovery_and_selection_only",
                    "post_selection_validation_ignored_during_selection":True,
                },
            )
            self.processes[proc.process_id]=proc
            retained.append(proc)
        self.process_frontier=[proc.process_id for proc in retained]
        self.events.append({
            "event":"PROCESS_FRONTIER_RETAINED",
            "generation":self.generation,
            "count":len(retained),
            "lineage_sets":[list(p.used_primitives) for p in retained],
            "selection_source":"discovery_and_selection_only",
        })
        return tuple(retained)

    def synthesize_process(self, rows:Sequence[Trace], transfer:Sequence[Trace], ood:Sequence[Trace])->Optional[ProcessCandidate]:
        frontier=self.synthesize_process_frontier(rows,transfer,ood)
        return frontier[0] if frontier else None

    def compatibility(self, proc:ProcessCandidate, trace:Trace)->float:
        risk=float(trace.context.get("transfer_risk",0.0)); amb=float(trace.context.get("ambiguity",0.0))
        return max(0.0,1.0-0.7*risk-0.5*amb)

    def evaluate(self, proc:ProcessCandidate, train:Sequence[Trace], holdout:Sequence[Trace], transfer:Sequence[Trace], ood:Sequence[Trace], conflict:Sequence[Trace])->Evaluation:
        def mae(rows):
            if not rows:return math.inf
            vals=[]
            for t in rows:
                try: vals.append(abs(eval_expr(proc.expression,t.inputs,self._pmap())-t.target))
                except Exception:return math.inf
            return sum(vals)/len(vals)
        tr,ho,te,oo=mae(train),mae(holdout),mae(transfer),mae(ood)
        conflict_error=mae(conflict)
        accepted=tr<=1e-9 and ho<=1e-9 and te<=1e-9
        return Evaluation(tr,ho,te,oo,max(0.0,1.0-conflict_error),float(proc.complexity),1.0/(1+proc.complexity),accepted)

    def retain(self, proc:ProcessCandidate, evaluation:Evaluation)->bool:
        if not evaluation.accepted:return False
        for pid in proc.used_primitives:
            if pid in self.archive:self.archive[pid].utility_history.append(1.0-evaluation.transfer_error)
        self.active=proc.process_id
        self.recursion_depth=max(self.recursion_depth,proc.generation+1)
        self.events.append({"event":"PROCESS_RETAINED","process_id":proc.process_id,"generation":proc.generation})
        return True

    def reuse(self, proc:ProcessCandidate, traces:Sequence[Trace])->Tuple[int,int]:
        a=r=0
        for t in traces:
            if self.compatibility(proc,t)>=0.65:a+=1
            else:r+=1
        return a,r

    def export_state(self)->Dict[str,Any]:
        return {"max_depth":self.max_depth,"population":self.population,"generation":self.generation,"recursion_depth":self.recursion_depth,"meta_policy":copy.deepcopy(self.meta_policy),"active":self.active,"primitives":{k:asdict(v) for k,v in self.primitives.items()},"processes":{k:asdict(v) for k,v in self.processes.items()},"process_frontier":list(self.process_frontier),"events":copy.deepcopy(self.events)}

    @classmethod
    def from_state(cls,state:Mapping[str,Any])->"RecursiveCognitiveCompiler":
        o=cls(int(state.get("max_depth",2)),int(state.get("population",24)),0)
        o.generation=int(state.get("generation",0)); o.recursion_depth=int(state.get("recursion_depth",0)); o.meta_policy=dict(state.get("meta_policy",o.meta_policy)); o.active=state.get("active")
        o.primitives={k:CognitivePrimitive(**v) for k,v in dict(state.get("primitives",{})).items()}
        o.archive={k:ArchiveRecord(v) for k,v in o.primitives.items()}
        o.processes={k:ProcessCandidate(**v) for k,v in dict(state.get("processes",{})).items()}
        o.process_frontier=list(state.get("process_frontier",[]))
        o.events=list(state.get("events",[])); return o