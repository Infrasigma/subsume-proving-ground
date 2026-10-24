from __future__ import annotations

import copy
import json
import statistics
from collections import Counter, defaultdict
from dataclasses import asdict, dataclass
from hashlib import sha256
from typing import Any, Mapping, Sequence

def _canon(x: Any) -> str:
    return json.dumps(x, sort_keys=True, separators=(",", ":"), default=str)

def _digest(x: Any) -> str:
    return sha256(_canon(x).encode()).hexdigest()

def outcome_category(outcome: Any) -> tuple[Any, ...]:
    if outcome is None:
        return ("none",)
    tok = tuple(outcome)
    if tok and str(tok[0]).startswith("delta") and len(tok) >= 2:
        try:
            value = float(tok[-1])
            return ("delta", str(tok[0]),
                    "positive" if value > 0 else "negative" if value < 0 else "zero",
                    "small" if abs(value) <= 1 else "large", *tok[2:])
        except (TypeError, ValueError):
            return tok
    if tok and tok[0] == "probe" and len(tok) >= 2:
        try:
            return ("probe", "low" if float(tok[1]) <= 1 else "high", *tok[2:])
        except (TypeError, ValueError):
            return tok
    return tok

@dataclass(frozen=True)
class OperatorExperience:
    event: str
    observed_outcome: tuple[Any, ...]
    chosen_policy: tuple[Any, ...]
    reward: float
    context_id: str
    step: int
    decision: bool = False
    episode_id: str = ""
    def to_dict(self) -> dict[str, Any]:
        return asdict(self)

@dataclass(frozen=True)
class OperatorClassSpec:
    class_id: str
    kind: str
    memory_depth: int
    transition_inputs: tuple[str, ...]
    dispatch_inputs: tuple[str, ...]
    stateful: bool
    history_dependent: bool
    generation: int
    parent_operator_ids: tuple[str, ...]
    train_reward: float
    holdout_reward: float
    transfer_reward: float
    stateless_holdout_reward: float
    stateless_transfer_reward: float
    holdout_regret: float
    transfer_regret: float
    provenance: Mapping[str, Any]
    status: str = "candidate"
    def to_dict(self) -> dict[str, Any]:
        return asdict(self)

class StatefulPolicyRouter:
    def __init__(self, spec: OperatorClassSpec, dispatch: Mapping[tuple[str, ...], tuple[Any, ...]], *, parent: "StatefulPolicyRouter | None" = None):
        self.spec = spec
        self.dispatch = {tuple(k): tuple(v) for k, v in dispatch.items()}
        self.parent = parent
        self.history: tuple[str, ...] = ()
        self.last_policy: tuple[Any, ...] | None = None
    def reset(self) -> None:
        self.history = ()
        self.last_policy = None
    def step(self, event: str, observed_outcome: Any) -> tuple[Any, ...] | None:
        # Runtime execution uses the same behavioral-consequence abstraction as induction;
        # literal event names are intentionally not part of the dispatch key.
        unit = _canon(outcome_category(observed_outcome))
        self.history = (self.history + (unit,))[-self.spec.memory_depth:]
        chosen = self.dispatch.get(self.history)
        if chosen is not None:
            self.last_policy = tuple(chosen)
            return self.last_policy
        if self.parent is not None:
            parent_policy = self.parent.step(event, observed_outcome)
            if parent_policy is not None:
                self.last_policy = tuple(parent_policy)
        return self.last_policy
    def export_state(self) -> dict[str, Any]:
        return {"spec": self.spec.to_dict(), "dispatch": {_canon(k): list(v) for k,v in self.dispatch.items()},
                "history": list(self.history), "last_policy": list(self.last_policy) if self.last_policy is not None else None,
                "parent": self.parent.export_state() if self.parent is not None else None}
    @classmethod
    def from_state(cls, state: Mapping[str, Any]) -> "StatefulPolicyRouter":
        raw_spec=dict(state["spec"])
        raw_spec["transition_inputs"]=tuple(raw_spec.get("transition_inputs",()))
        raw_spec["dispatch_inputs"]=tuple(raw_spec.get("dispatch_inputs",()))
        raw_spec["parent_operator_ids"]=tuple(raw_spec.get("parent_operator_ids",()))
        raw_spec["provenance"]=dict(raw_spec.get("provenance",{}))
        spec=OperatorClassSpec(**raw_spec)
        dispatch={tuple(json.loads(k)): tuple(v) for k,v in dict(state.get("dispatch",{})).items()}
        parent_raw=state.get("parent")
        router = cls(spec,dispatch,parent=cls.from_state(parent_raw) if parent_raw else None)
        router.history=tuple(state.get("history",()))
        lp=state.get("last_policy")
        router.last_policy=tuple(lp) if lp is not None else None
        return router

StatefulHistoryRouter = StatefulPolicyRouter

class OpenSemanticSubstrate:
    LEGACY_CLASS="stateless_event_dispatch"
    CANDIDATE_DEPTHS=(1,2,3)
    def __init__(self,seed:int=0):
        self.seed=int(seed); self.generation=0; self.language={}; self.active_operator_id=None
    @staticmethod
    def _unit(row: OperatorExperience) -> str:
        # Transfer is defined over observed behavioral consequence, not the literal
        # action/name vocabulary. This is what allows the class to cross renamed surfaces.
        return _canon(outcome_category(row.observed_outcome))
    @staticmethod
    def _decision_rows(episodes):
        return [r for ep in episodes for r in ep if r.decision]
    @staticmethod
    def _fit_dispatch(episodes, depth):
        grouped=defaultdict(list)
        for ep in episodes:
            for row in ep:
                grouped[(str(row.context_id), str(row.episode_id))].append(row)
        votes=defaultdict(Counter); rewards=defaultdict(lambda:defaultdict(float)); counts=defaultdict(Counter)
        for context_rows in grouped.values():
            context_rows.sort(key=lambda r:int(r.step)); history=[]
            for row in context_rows:
                history.append(OpenSemanticSubstrate._unit(row))
                if not row.decision:
                    continue
                key=tuple(history[-depth:])
                pol=tuple(row.chosen_policy)
                counts[key][pol]+=1; rewards[key][pol]+=float(row.reward)
        return {k:max(counts[k], key=lambda p:(rewards[k][p]/max(1,counts[k][p]), _canon(p))) for k in counts}
    @staticmethod
    def _evaluate_dispatch(episodes,dispatch,depth):
        grouped=defaultdict(lambda: defaultdict(list))
        for ep in episodes:
            for row in ep:
                grouped[str(row.context_id)][str(row.episode_id)].append(row)
        vals=[]; regrets=[]
        for trials in grouped.values():
            representative=next(iter(trials.values()))
            representative.sort(key=lambda r:int(r.step)); history=[]
            for row in representative:
                history.append(OpenSemanticSubstrate._unit(row))
                if not row.decision:
                    continue
                key=tuple(history[-depth:]); chosen=dispatch.get(key)
                if chosen is None: continue
                all_rows=[r for trial in trials.values() for r in trial if r.decision]
                matching=[r for r in all_rows if tuple(r.chosen_policy)==tuple(chosen)]
                if not matching: continue
                actual=statistics.mean(float(r.reward) for r in matching)
                best=max(float(r.reward) for r in all_rows)
                vals.append(actual); regrets.append(max(0.0,best-actual))
        return (statistics.mean(vals) if vals else 0.0, statistics.mean(regrets) if regrets else 1.0)
    @staticmethod
    def _conflict_fraction(rows):
        grouped=defaultdict(set)
        for r in rows: grouped[OpenSemanticSubstrate._unit(r)].add(tuple(r.chosen_policy))
        return sum(len(v)>1 for v in grouped.values())/max(1,len(grouped))
    def discover_operator_class(self,train,holdout,transfer,*,parent_operator_ids=()):
        train_rows=self._decision_rows(train)
        if len(train_rows)<8:return None
        legacy=self._fit_dispatch(train,1)
        sh,sr=self._evaluate_dispatch(holdout,legacy,1); st,tr=self._evaluate_dispatch(transfer,legacy,1)
        best=None
        for d in self.CANDIDATE_DEPTHS:
            disp=self._fit_dispatch(train,d); h,hr=self._evaluate_dispatch(holdout,disp,d); t,rr=self._evaluate_dispatch(transfer,disp,d)
            cand=(d,h,t,hr,rr,disp)
            if best is None or (h,t,-hr-rr,-d)>(best[1],best[2],-best[3]-best[4],-best[0]): best=cand
        d,h,t,hr,rr,disp=best; conflict=self._conflict_fraction(train_rows)
        gain=(h+t)-(sh+st)
        if d<2 or conflict<0.25 or h<0.90 or t<0.90 or hr>0.10 or rr>0.10 or st>=0.90 or gain<0.20: return None
        cid="oc:"+_digest(("stateful_policy_router",d,sorted((k,v) for k,v in disp.items()),tuple(parent_operator_ids),self.generation))[:20]
        spec=OperatorClassSpec(cid,"stateful_policy_router",int(d),("event","observed_outcome"),("history_state","event","observed_outcome"),True,True,
            int(self.generation),tuple(parent_operator_ids),
            statistics.mean(float(r.reward) for r in train_rows),float(h),float(t),float(sh),float(st),float(hr),float(rr),
            {"origin":"trace_induced_operator_class","class_discovered_from_execution_traces":True,
             "reward_only_induction":True,"oracle_policy_hidden_from_learner":True,"regime_labels_hidden_from_learner":True,
             "human_authored_solution":False,"external_model_solution":False,"legacy_class":self.LEGACY_CLASS,
             "state_conflict_fraction":conflict,"genuine_gain":gain},"validated")
        self.language[cid]={"spec":spec.to_dict(),"dispatch":{_canon(k):list(v) for k,v in disp.items()}}
        self.active_operator_id=cid
        return spec
    def instantiate(self,spec):
        raw=self.language[spec.class_id]
        disp={tuple(json.loads(k)):tuple(v) for k,v in dict(raw.get("dispatch",{})).items()}
        parent=None
        if spec.parent_operator_ids and str(spec.parent_operator_ids[0]) in self.language:
            ps=OperatorClassSpec(**dict(self.language[str(spec.parent_operator_ids[0])]["spec"]))
            parent=self.instantiate(ps)
        return StatefulPolicyRouter(spec,disp,parent=parent)
    def export_state(self):
        return {"generation":self.generation,"active_operator_id":self.active_operator_id,"language":copy.deepcopy(self.language)}
    @classmethod
    def from_state(cls,state):
        o=cls(0)
        o.generation=int(state.get("generation",0))
        o.active_operator_id=state.get("active_operator_id")
        raw_language=copy.deepcopy(dict(state.get("language",{})))
        normalized={}
        for oid, raw in dict(raw_language).items():
            spec=dict(raw.get("spec",{}))
            spec["transition_inputs"]=tuple(spec.get("transition_inputs",()))
            spec["dispatch_inputs"]=tuple(spec.get("dispatch_inputs",()))
            spec["parent_operator_ids"]=tuple(spec.get("parent_operator_ids",()))
            spec["provenance"]=dict(spec.get("provenance",{}))
            normalized[str(oid)]={"spec":spec,"dispatch":dict(raw.get("dispatch",{}))}
        o.language=normalized
        return o

def apply_discovered_class_to_learner(learner,policy,*,dry_run=False):
    if len(policy)!=6: raise ValueError("CORE-016 cognitive policy must have six layer controls")
    rep_depth,search_budget,planning_horizon,learning_flag,acquisition_budget,world_horizon=policy
    before={"representation_depth":int(getattr(learner,"representation_depth",2)),
            "search_budget":int(getattr(learner,"_asi_core_009_state",{}).get("search_policy",{}).get("budget",3500)),
            "planning_horizon":int(getattr(learner,"_asi_core_013_state",{}).get("planning_policy",{}).get("horizon",4)),
            "learning_flag":bool(getattr(learner,"enable_primitive_invention",True)),
            "acquisition_budget":int(getattr(learner,"_asi_core_009_state",{}).get("search_policy",{}).get("beam",64)),
            "world_horizon":int(getattr(learner,"_asi_core_010_state",{}).get("max_delay",4))}
    if dry_run:return {"before":before,"proposed":dict(zip(before,policy)),"after":before,"dry_run":True}
    learner.representation_depth=max(1,min(8,int(rep_depth)))
    st9=getattr(learner,"_asi_core_009_state",None)
    if st9 is not None:
        search=st9.setdefault("search_policy",{"max_depth":6,"beam":64,"budget":3500}); search["budget"]=max(64,min(20000,int(search_budget))); search["beam"]=max(8,min(256,int(acquisition_budget)))
    st13=getattr(learner,"_asi_core_013_state",None)
    if st13 is not None:
        st13.setdefault("planning_policy",{}); st13["planning_policy"]["horizon"]=max(1,min(16,int(planning_horizon)))
    learner.enable_primitive_invention=bool(learning_flag)
    st10=getattr(learner,"_asi_core_010_state",None)
    if st10 is not None: st10["max_delay"]=max(1,min(8,int(world_horizon)))
    after={"representation_depth":int(getattr(learner,"representation_depth",rep_depth)),
           "search_budget":int(getattr(learner,"_asi_core_009_state",{}).get("search_policy",{}).get("budget",search_budget)),
           "planning_horizon":int(getattr(learner,"_asi_core_013_state",{}).get("planning_policy",{}).get("horizon",planning_horizon)),
           "learning_flag":bool(getattr(learner,"enable_primitive_invention",learning_flag)),
           "acquisition_budget":int(getattr(learner,"_asi_core_009_state",{}).get("search_policy",{}).get("beam",acquisition_budget)),
           "world_horizon":int(getattr(learner,"_asi_core_010_state",{}).get("max_delay",world_horizon))}
    return {"before":before,"proposed":dict(zip(before,policy)),"after":after,"dry_run":False}
