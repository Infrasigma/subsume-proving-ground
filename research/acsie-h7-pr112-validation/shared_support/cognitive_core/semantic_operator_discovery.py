from __future__ import annotations
import copy, json, math
from dataclasses import dataclass, asdict
from hashlib import sha256
from typing import Any, Mapping, Sequence

def _digest(x: Any) -> str:
    return sha256(json.dumps(x, sort_keys=True, separators=(",", ":"), default=str).encode()).hexdigest()

def _sig(values: Sequence[float]) -> tuple[float,float,float,float]:
    if not values: return (0.0,0.0,0.0,0.0)
    m=sum(values)/len(values); v=sum((x-m)*(x-m) for x in values)/len(values)
    return (min(values),max(values),m,math.sqrt(v))

def _ast(e: Mapping[str,Any]) -> float:
    if e["op"] in {"state","role","const"}: return 1
    if e["op"]=="threshold": return 1+_ast(e["arg"])
    return 1+_ast(e["left"])+_ast(e["right"])

def eval_expr(e: Mapping[str,Any], state: Mapping[str,float], roles: Mapping[str,float]) -> float:
    op=e["op"]
    if op=="const": return float(e["value"])
    if op=="state": return float(state[e["key"]])
    if op=="role": return float(roles[e["key"]])
    if op=="threshold": return 1.0 if eval_expr(e["arg"],state,roles)>=float(e["lo"]) else 0.0
    a=eval_expr(e["left"],state,roles); b=eval_expr(e["right"],state,roles)
    return {"add":lambda:a+b,"sub":lambda:a-b,"mul":lambda:a*b,"max":lambda:max(a,b),"min":lambda:min(a,b)}[op]()

@dataclass(frozen=True)
class Effect:
    field: str
    expression: Mapping[str,Any]
    value_type: str="continuous"

@dataclass(frozen=True)
class SemanticOperator:
    operator_id: str
    input_roles: tuple[str,...]
    effects: tuple[Effect,...]
    kind: str
    generation: int
    parent_operator_ids: tuple[str, ...]
    train_error: float
    holdout_error: float
    transfer_error: float
    complexity: int
    provenance: Mapping[str,Any]
    status: str="candidate"

    def to_dict(self): return asdict(self)
    def apply(self,state:Mapping[str,float],roles:Mapping[str,float],language=None) -> dict[str,float]:
        out=dict(state)
        if language is not None:
            for pid in self.parent_operator_ids:
                out=language.operators[pid].apply(out,roles,language)
        for e in self.effects: out[e.field]=eval_expr(e.expression,out,roles)
        return out

def _canonical(rows:Sequence[Mapping[str,Any]], role="pressure"):
    out=[]
    for r in rows:
        vals=[float(v) for v in dict(r.get("telemetry",{})).values() if isinstance(v,(int,float)) and not isinstance(v,bool)]
        if vals:
            q=copy.deepcopy(dict(r)); q["telemetry_roles"]={role:vals[0]}; out.append(q)
    return out

def _mae(rows,effects):
    err=[]
    for r in rows:
        role=float(r["telemetry_roles"]["pressure"])
        roles={"pressure":role}
        for e in effects:
            try: err.append(abs(eval_expr(e.expression,r["before"],roles)-float(r["after"][e.field])))
            except Exception: return math.inf
    return sum(err)/max(1,len(err))

def _exprs(fields):
    consts=(-1000,-500,-100,-32,-16,-8,-4,-2,-1,0,1,2,4,8,16,32,64,100,500,1000)
    role={"op":"role","key":"pressure"}; gate={"op":"threshold","arg":role,"lo":0.5}
    out=[]
    for k in fields:
        st={"op":"state","key":k}; out.append(st)
        for c in consts:
            cc={"op":"const","value":c}
            out.extend([
                {"op":"add","left":st,"right":cc},{"op":"sub","left":st,"right":cc},
                {"op":"mul","left":st,"right":cc},
                {"op":"add","left":st,"right":{"op":"mul","left":role,"right":cc}},
                {"op":"add","left":st,"right":{"op":"mul","left":gate,"right":cc}},
                {"op":"mul","left":st,"right":{"op":"add","left":{"op":"const","value":1},"right":gate}},
            ])
    return list({json.dumps(x,sort_keys=True):x for x in out}.values())

def _bind(op:SemanticOperator,telemetry:Mapping[str,Any]) -> dict[str,float]:
    vals=[float(v) for v in telemetry.values() if isinstance(v,(int,float)) and not isinstance(v,bool)]
    if not vals: raise ValueError("no numeric telemetry")
    return {r:vals[0] for r in op.input_roles}

class SemanticOperatorLanguage:
    def __init__(self): self.operators={}
    def install(self,op): self.operators[op.operator_id]=op; return "sop:"+op.operator_id
    def token(self,operator_id): return "sop:"+str(operator_id)
    def execute(self, token, state, telemetry):
        if not str(token).startswith("sop:"):
            raise ValueError("invalid semantic operator token")
        oid=str(token)[4:]
        op=self.operators.get(oid)
        if op is None:
            raise KeyError(oid)
        return op.apply(state, _bind(op, telemetry), self)
    def export_state(self):
        return {"operators":{k:v.to_dict() for k,v in self.operators.items()}}
    @classmethod
    def from_state(cls,state):
        out=cls()
        for oid, raw in dict(state.get("operators",{})).items():
            op=SemanticOperator(
                operator_id=str(raw["operator_id"]),
                input_roles=tuple(raw.get("input_roles",())),
                effects=tuple(Effect(**e) for e in raw.get("effects",())),
                kind=str(raw["kind"]),
                generation=int(raw.get("generation",0)),
                parent_operator_ids=tuple(raw.get("parent_operator_ids",())),
                train_error=float(raw.get("train_error",0.0)),
                holdout_error=float(raw.get("holdout_error",0.0)),
                transfer_error=float(raw.get("transfer_error",0.0)),
                complexity=int(raw.get("complexity",0)),
                provenance=dict(raw.get("provenance",{})),
                status=str(raw.get("status","candidate")),
            )
            out.install(op)
        return out

class SemanticOperatorDiscovery:
    def __init__(self,seed:int=0): self.generation=0; self.language=SemanticOperatorLanguage()

    def export_state(self):
        return {"generation":self.generation,"language":self.language.export_state()}

    @classmethod
    def from_state(cls,state):
        out=cls(0)
        out.generation=int(state.get("generation",0))
        out.language=SemanticOperatorLanguage.from_state(state.get("language",{}))
        return out
    def discover(self,train,holdout,transfer):
        tc,hc,xc=_canonical(train),_canonical(holdout),_canonical(transfer)
        if len(tc)<8 or len(hc)<4 or len(xc)<6: return None
        fields=sorted(set(tc[0]["before"]) & set(tc[0]["after"]))
        changed=[k for k in fields if any(float(r["before"][k])!=float(r["after"][k]) for r in tc)]
        if len(changed)<2: return None
        parent_ids=tuple(sorted(self.language.operators))[-1:] if self.generation>0 and self.language.operators else ()
        def apply_parents(rows):
            if not parent_ids: return rows
            parent=self.language.operators[parent_ids[0]]
            out=[]
            for row in rows:
                q=copy.deepcopy(row)
                role=float(q["telemetry_roles"]["pressure"])
                q["before"]=parent.apply(q["before"],{"pressure":role},self.language)
                out.append(q)
            return out
        tc=apply_parents(tc); hc=apply_parents(hc); xc=apply_parents(xc)
        candidates=_exprs(fields); chosen=[]
        for field in changed:
            found=None
            for e in candidates:
                ef=Effect(field,e)
                if _mae(tc,[ef])<=1e-9 and _mae(hc,[ef])<=1e-9:
                    found=ef; break
            if found: chosen.append(found)
        if len(chosen)<2: return None
        tr,ho,te=_mae(tc,chosen),_mae(hc,chosen),_mae(xc,chosen)
        if max(tr,ho,te)>1e-9: return None
        fields_seen=sorted({next(iter(r["telemetry"])) for r in list(train)+list(transfer) if r.get("telemetry")})
        op=SemanticOperator(
            "sem:"+_digest((tuple(chosen),self.generation,parent_ids))[:20],
            ("pressure",),tuple(chosen),"multi_effect_typed_control",self.generation,parent_ids,
            tr,ho,te,sum(int(_ast(e.expression)) for e in chosen),
            {"origin":"autonomous_semantic_operator_discovery","human_authored_solution":False,
             "external_model_solution":False,"role_profiles":{"pressure":_sig([r["telemetry_roles"]["pressure"] for r in tc])},
             "source_telemetry_fields":fields_seen,"effect_count":len(chosen),
            "composes_prior_operator":bool(parent_ids),"parent_operator_ids":list(parent_ids)},
            "validated")
        self.language.install(op)
        return op
    def export_state(self): return {"generation":self.generation,"language":self.language.export_state()}

def apply_to_general_learner(learner:Any,op:SemanticOperator,telemetry:Mapping[str,Any],dry_run:bool=False):
    before={"representation_depth":int(getattr(learner,"representation_depth",2))}
    core9=getattr(learner,"_asi_core_009_state",None) or {}
    pol=core9.get("search_policy",{})
    before["search_budget"]=int(pol.get("budget",3500)); before["search_beam"]=int(pol.get("beam",64))
    language_state=getattr(learner,"_asi_core_013_state",{}).get("semantic_language",{})
    language=SemanticOperatorLanguage.from_state(language_state)
    proposed=op.apply(before,_bind(op,telemetry),language); after=dict(before)
    if not dry_run:
        learner.representation_depth=max(1,min(8,int(round(proposed.get("representation_depth",before["representation_depth"])))))
        after["representation_depth"]=learner.representation_depth
        if not hasattr(learner,"_asi_core_009_state") or learner._asi_core_009_state is None:
            learner._asi_core_009_state={"search_policy":{"max_depth":6,"beam":64,"budget":3500}}
        policy=learner._asi_core_009_state.setdefault("search_policy",{"max_depth":6,"beam":64,"budget":3500})
        if "search_budget" in proposed: policy["budget"]=max(1,min(20000,int(round(proposed["search_budget"]))))
        if "search_beam" in proposed: policy["beam"]=max(4,min(256,int(round(proposed["search_beam"]))))
        after.update({"search_budget":int(policy["budget"]),"search_beam":int(policy["beam"])})
    return {"operator_id":op.operator_id,"before":before,"proposed":proposed,"after":after,"dry_run":dry_run}
