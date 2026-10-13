"""ASI-CORE-009 generic open-capability acquisition kernel.

This module adds no task-family solver. It gives the single GeneralLearner a generic,
role-agnostic capability program search, uncertainty/diagnosis records, active evidence
selection, acquisition/retention memory, and serialization. Evaluator truth is never
imported here.
"""
from __future__ import annotations
from dataclasses import dataclass, asdict
from hashlib import sha256
from itertools import product
from typing import Any, Callable, Mapping, Sequence
import copy, json, math, time

Json = Any


def _canon(x: Any) -> str:
    return json.dumps(x, sort_keys=True, separators=(",", ":"), ensure_ascii=True, default=str)


def _digest(x: Any) -> str:
    return sha256(_canon(x).encode()).hexdigest()


@dataclass(frozen=True)
class Diagnosis:
    diagnosis_id: str
    kind: str
    confidence: float
    evidence: Mapping[str, Any]
    action: str

    def to_dict(self) -> dict[str, Any]:
        return asdict(self)


@dataclass(frozen=True)
class CapabilityRecord:
    capability_id: str
    program: tuple[str, ...]
    evidence_count: int
    search_work: int
    validation_score: float
    acquired_from_failure: bool
    source: str = "open_capability"

    def to_dict(self) -> dict[str, Any]:
        return asdict(self)


def _unwrap(x: Json) -> Json:
    if isinstance(x, list) and x and all(isinstance(v, dict) and "payload" in v for v in x):
        return [v["payload"] for v in x]
    return copy.deepcopy(x)


def _field_by_index(row: Mapping[str, Any], idx: int) -> Any:
    keys = list(row.keys())
    if idx < 0 or idx >= len(keys):
        return None
    return row[keys[idx]]


def _apply_token(x: Json, token: str) -> Json | None:
    try:
        if token == "unwrap_payload":
            return _unwrap(x)
        if token.startswith("select_index:"):
            idx = int(token.split(":", 1)[1])
            if not isinstance(x, list):
                return None
            return [_field_by_index(row, idx) if isinstance(row, Mapping) else None for row in x]
        if token in {"select_numeric_lexical_min", "select_numeric_lexical_max", "select_numeric_wide", "select_numeric_narrow"}:
            rows=_unwrap(x)
            if not isinstance(rows, list) or not all(isinstance(row, Mapping) for row in rows):
                return None
            keys=sorted({k for row in rows for k,v in row.items() if isinstance(v,(int,float)) and not isinstance(v,bool)})
            if not keys: return None
            if token in {"select_numeric_lexical_min","select_numeric_lexical_max"}:
                chosen=keys[0] if token.endswith("min") else keys[-1]
            else:
                scored=[]
                for k in keys:
                    vals=[row.get(k) for row in rows]
                    if all(isinstance(v,(int,float)) and not isinstance(v,bool) for v in vals):
                        scored.append((max(vals)-min(vals),k))
                if not scored: return None
                chosen=(max if token=="select_numeric_wide" else min)(scored)[1]
            return [copy.deepcopy(row[chosen]) for row in rows]
        if token == "select_non_numeric_lexical_min":
            if not isinstance(x, list):
                return None
            out=[]
            for row in x:
                if not isinstance(row, Mapping): return None
                keys=sorted(k for k,v in row.items() if not isinstance(v,(int,float)) or isinstance(v,bool))
                if not keys: return None
                out.append(copy.deepcopy(row[keys[0]]))
            return out
        if token.startswith("window_sum:"):
            k = int(token.split(":", 1)[1])
            if not isinstance(x, list) or k < 2 or len(x) < k:
                return None
            if not all(isinstance(v, (int, float)) and not isinstance(v, bool) for v in x):
                return None
            return [sum(x[i:i+k]) for i in range(len(x)-k+1)]
        if token.startswith("window_max:"):
            k = int(token.split(":", 1)[1])
            if not isinstance(x, list) or k < 2 or len(x) < k:
                return None
            if not all(isinstance(v, (int, float)) and not isinstance(v, bool) for v in x):
                return None
            return [max(x[i:i+k]) for i in range(len(x)-k+1)]
        if token == "adjacent_delta":
            if not isinstance(x, list) or len(x) < 2:
                return None
            if not all(isinstance(v, (int, float)) and not isinstance(v, bool) for v in x):
                return None
            return [x[i+1]-x[i] for i in range(len(x)-1)]
        if token == "adjacent_ratio_floor":
            if not isinstance(x, list) or len(x) < 2:
                return None
            if not all(isinstance(v, (int, float)) and not isinstance(v, bool) for v in x):
                return None
            return [0 if x[i] == 0 else x[i+1] // x[i] for i in range(len(x)-1)]
        if token.startswith("map_affine:"):
            _, a, b = token.split(":")
            a, b = int(a), int(b)
            if not isinstance(x, list) or not all(isinstance(v, (int, float)) and not isinstance(v, bool) for v in x):
                return None
            return [a*v+b for v in x]
        if token.startswith("filter_parity:"):
            p = int(token.split(":", 1)[1])
            if not isinstance(x, list) or not all(isinstance(v, (int, float)) and not isinstance(v, bool) for v in x):
                return None
            return [v for v in x if int(v) % 2 == p]
        if token == "sort":
            if not isinstance(x, list):
                return None
            if all(isinstance(v,(int,float)) and not isinstance(v,bool) for v in x):
                return sorted(copy.deepcopy(x))
            return sorted(copy.deepcopy(x), key=lambda v: _canon(v))
        if token == "reverse":
            if not isinstance(x, list):
                return None
            return list(reversed(copy.deepcopy(x)))
        if token == "dedupe":
            if not isinstance(x, list):
                return None
            out = []
            for v in x:
                if v not in out:
                    out.append(copy.deepcopy(v))
            return out
        if token in {"group_count_numeric_narrow", "group_count_non_numeric_lexical_min"}:
            if token == "group_count_numeric_narrow":
                rows=_unwrap(x)
                if not isinstance(rows,list) or not all(isinstance(row,Mapping) for row in rows): return None
                nums=sorted(k for k,v in rows[0].items() if isinstance(v,(int,float)) and not isinstance(v,bool))
                # select the numeric role with the smallest observed spread across rows.
                scored=[]
                for k in nums:
                    vals=[row.get(k) for row in rows]
                    if all(isinstance(v,(int,float)) and not isinstance(v,bool) for v in vals): scored.append((max(vals)-min(vals),k))
                if not scored: return None
                gk=min(scored)[1]; c={}
                for row in rows:
                    g=row[gk]; c[g]=c.get(g,0)+1
                return [[k,c[k]] for k in sorted(c)]
        if token == "group_count_non_numeric_lexical_min":
            rows = _unwrap(x)
            if not isinstance(rows, list) or not all(isinstance(row, Mapping) for row in rows):
                return None
            counts: dict[str, list[Any]] = {}
            for row in rows:
                keys=sorted(k for k,v in row.items() if not isinstance(v,(int,float)) or isinstance(v,bool))
                if not keys: return None
                g=copy.deepcopy(row[keys[0]])
                key=_canon(g); counts.setdefault(key,[g,0]); counts[key][1]+=1
            return [counts[k] for k in sorted(counts)]
        if token.startswith("group_count_index:"):
            idx = int(token.split(":", 1)[1])
            rows = _unwrap(x)
            if not isinstance(rows, list) or not all(isinstance(row, Mapping) for row in rows):
                return None
            counts: dict[str, list[Any]] = {}
            for row in rows:
                g = _field_by_index(row, idx)
                key = _canon(g)
                counts.setdefault(key, [g, 0])
                counts[key][1] += 1
            return [counts[k] for k in sorted(counts)]
        if token.startswith("group_sum_indices:"):
            _, gidx, vidx = token.split(":")
            gidx, vidx = int(gidx), int(vidx)
            rows = _unwrap(x)
            if not isinstance(rows, list) or not all(isinstance(row, Mapping) for row in rows):
                return None
            sums: dict[str, list[Any]] = {}
            for row in rows:
                g, v = _field_by_index(row, gidx), _field_by_index(row, vidx)
                if not isinstance(v, (int, float)) or isinstance(v, bool):
                    return None
                key = _canon(g)
                sums.setdefault(key, [g, 0])
                sums[key][1] += v
            return [sums[k] for k in sorted(sums)]
    except (TypeError, ValueError, IndexError, KeyError):
        return None
    return None


def _run(program: Sequence[str], x: Json) -> Json | None:
    out = copy.deepcopy(x)
    for token in program:
        out = _apply_token(out, token)
        if out is None:
            return None
    return out


def _token_candidates(examples: Sequence[tuple[Json, Json]]) -> tuple[str, ...]:
    tokens = {"unwrap_payload", "select_numeric_lexical_min", "select_numeric_lexical_max", "select_numeric_wide", "select_numeric_narrow", "select_non_numeric_lexical_min", "group_count_numeric_narrow",
              "adjacent_delta", "adjacent_ratio_floor", "sort", "reverse", "dedupe", "group_count_non_numeric_lexical_min"}
    for idx in range(5):
        tokens.add(f"select_index:{idx}")
        tokens.add(f"group_count_index:{idx}")
        for vidx in range(5):
            if vidx != idx:
                tokens.add(f"group_sum_indices:{idx}:{vidx}")
    for k in (2, 3, 4):
        tokens.add(f"window_sum:{k}")
        tokens.add(f"window_max:{k}")
    for a in (-2, -1, 1, 2, 3):
        for b in (-4, -2, -1, 0, 1, 2, 4):
            tokens.add(f"map_affine:{a}:{b}")
    for p in (0, 1):
        tokens.add(f"filter_parity:{p}")
    return tuple(sorted(tokens))


def _fit(program: Sequence[str], examples: Sequence[tuple[Json, Json]]) -> float:
    good = 0
    for x, y in examples:
        good += int(_run(program, x) == y)
    return good / max(1, len(examples))


def _shape_distance(a: Json, b: Json) -> float:
    if a == b:
        return 0.0
    if type(a) is not type(b):
        return 1.0
    if isinstance(a, list) and isinstance(b, list):
        return min(1.0, abs(len(a)-len(b))/max(1,len(a),len(b)))
    if isinstance(a, dict) and isinstance(b, dict):
        return min(1.0, abs(len(a)-len(b))/max(1,len(a),len(b)))
    return 0.75


def _program_quality(program: Sequence[str], examples: Sequence[tuple[Json, Json]]) -> tuple[float, float, float]:
    exact = 0.0
    valid = 0.0
    dist = 0.0
    for x, y in examples:
        out = _run(program, x)
        if out is not None:
            valid += 1.0
            exact += float(out == y)
            dist += _shape_distance(out, y)
        else:
            dist += 1.0
    n=max(1,len(examples))
    exact/=n; valid/=n; dist/=n
    return exact, valid, dist


def _batch_apply(token: str, state: tuple[Json, ...]) -> tuple[Json | None, ...]:
    return tuple(_apply_token(x, token) for x in state)


def _behavior_distance(state: tuple[Json | None, ...], target: tuple[Json, ...]) -> tuple[float, float, float]:
    exact=shape=numeric=0.0
    for out,y in zip(state,target):
        exact += float(out==y)
        if isinstance(out,list) and isinstance(y,list):
            shape += 1.0-min(1.0,abs(len(out)-len(y))/max(1,len(out),len(y)))
            if len(out)==len(y) and all(isinstance(v,(int,float)) and not isinstance(v,bool) for v in out+y):
                scale=max(1.0,max((abs(float(v)) for v in y),default=0.0))
                numeric += 1.0 if not y else 1.0-min(1.0,sum(abs(float(a)-float(b)) for a,b in zip(out,y))/(len(y)*scale))
        elif type(out) is type(y):
            shape += 0.5
    n=max(1,len(target)); return exact/n,shape/n,numeric/n


def _infer_affine_tokens(state: tuple[Json,...], target: tuple[Json,...]) -> tuple[str,...]:
    if not state or not target: return ()
    pairs=[]
    for xs,ys in zip(state,target):
        if not (isinstance(xs,list) and isinstance(ys,list) and len(xs)==len(ys)): return ()
        pairs.extend((float(a),float(b)) for a,b in zip(xs,ys)
                      if isinstance(a,(int,float)) and not isinstance(a,bool) and isinstance(b,(int,float)) and not isinstance(b,bool))
    vals=[]
    for i,(x1,y1) in enumerate(pairs[:8]):
        for x2,y2 in pairs[i+1:12]:
            if x2==x1: continue
            a=(y2-y1)/(x2-x1); b=y1-a*x1
            if abs(a-round(a))<1e-9 and abs(b-round(b))<1e-9 and -8<=a<=8 and -16<=b<=16:
                vals.append((int(round(a)),int(round(b))))
            elif -8<=a<=8 and -16<=b<=16:
                vals.append((a,b))
    out=[]; seen=set()
    for a,b in vals:
        tok=f"map_affine:{a}:{b}"
        if tok not in seen: out.append(tok); seen.add(tok)
    return tuple(out[:12])


def _fast_active_tokens(state: tuple[Json,...], target: tuple[Json,...], examples: Sequence[tuple[Json,Json]], program: tuple[str,...])->tuple[str,...]:
    first=state[0] if state else None
    if isinstance(first,list) and first and all(isinstance(v,Mapping) for v in first):
        return tuple(t for t in _token_candidates(examples) if t in {
            "unwrap_payload","select_numeric_wide","select_numeric_narrow","select_numeric_lexical_min","select_numeric_lexical_max",
            "select_non_numeric_lexical_min","group_count_numeric_narrow"} or t.startswith(("select_index:","group_count_index:","group_sum_indices:")))
    if isinstance(first,list) and all(isinstance(v,(int,float)) and not isinstance(v,bool) for v in first):
        n0=len(first); t0=target[0] if target else None; nt=len(t0) if isinstance(t0,list) else -1
        ops=[]
        if nt>0 and n0>=nt:
            k=n0-nt+1
            if 2<=k<=4:
                ops += [f"window_sum:{k}",f"window_max:{k}"]
        if nt==n0-1: ops.append("adjacent_delta")
        ops += list(_infer_affine_tokens(state,target))
        ops += ["filter_parity:0","filter_parity:1","sort","reverse","dedupe"]
        return tuple(dict.fromkeys(ops))
    if isinstance(first,list):
        return ("sort","reverse","dedupe")
    return ("unwrap_payload",)


def _search_program(examples: Sequence[tuple[Json, Json]], *, budget: int, max_depth: int, beam: int) -> tuple[tuple[str, ...] | None, int, float]:
    """Task-independent synthesis using observable structure and inferred parameters."""
    if not examples: return None,0,1.0
    hold=examples[-2:] if len(examples)>=4 else examples[-1:]
    train=examples[:-len(hold)] if len(examples)>len(hold) else examples
    inputs=tuple(copy.deepcopy(x) for x,_ in train); target=tuple(copy.deepcopy(y) for _,y in train)
    candidates=[]
    # Start from generic role hypotheses and structural navigation hypotheses.
    prefixes=("unwrap_payload","select_numeric_wide","select_numeric_narrow","select_numeric_lexical_min",
              "select_numeric_lexical_max","select_non_numeric_lexical_min","select_index:0","select_index:1",
              "select_index:2","select_index:3","select_index:4","group_count_numeric_narrow","group_count_index:0",
              "group_count_index:1","group_count_index:2","group_count_index:3","group_count_index:4")
    frontier=[(inputs,())]
    seen={_digest(tuple(_canon(x) for x in inputs))}; tested=0; best_error=1.0
    for p in prefixes:
        st=_batch_apply(p,inputs); tested+=1
        if tested>budget: return None,tested,best_error
        ex,sh,nu=_behavior_distance(st,target); best_error=min(best_error,1-ex)
        if ex>=1.0 and (_fit((p,),hold) if hold else 1.0)>=1.0: return (p,),tested,0.0
        key=_digest(tuple(_canon(x) for x in st))
        if key not in seen:
            seen.add(key); candidates.append((4*ex+1.3*sh+0.6*nu-0.03, (p,), st))
    candidates.sort(key=lambda z:(-z[0],len(z[1]),_canon(z[1])))
    frontier=[(st,p) for _,p,st in candidates[:max(8,min(64,beam))]]
    for depth in range(2,max_depth+1):
        scored=[]
        for state,program in frontier:
            active=_fast_active_tokens(state,target,examples,program)
            for tok in active:
                if program and tok==program[-1] and tok in {"sort","reverse","dedupe"}: continue
                p=program+(tok,); tested+=1
                if tested>budget: return None,tested,best_error
                st2=_batch_apply(tok,state); ex,sh,nu=_behavior_distance(st2,target); best_error=min(best_error,1-ex)
                if ex>=1.0 and (_fit(p,hold) if hold else 1.0)>=1.0: return p,tested,0.0
                key=_digest(tuple(_canon(x) for x in st2))
                if key in seen: continue
                seen.add(key)
                score=4*ex+1.3*sh+0.6*nu-0.03*len(p)
                scored.append((score,p,st2))
        scored.sort(key=lambda z:(-z[0],len(z[1]),_canon(z[1])))
        frontier=[(st,p) for _,p,st in scored[:max(8,min(64,beam))]]
        if not frontier: break
    return None,tested,best_error

def _hypothesis_frontier(examples: Sequence[tuple[Json, Json]], *, budget: int = 2500, max_depth: int = 5, beam: int = 64, max_candidates: int = 32) -> tuple[tuple[str, ...], ...]:
    """Build competing generic hypotheses from observed examples only.

    The frontier is deliberately evaluator-blind: it consumes only input/output
    observations already possessed by the learner. It records multiple programs that
    are observationally consistent, allowing evidence selection to discriminate among
    them rather than collapsing immediately to the first exact hypothesis.
    """
    if not examples:
        return ((),)
    inputs=tuple(copy.deepcopy(x) for x,_ in examples)
    target=tuple(copy.deepcopy(y) for _,y in examples)
    prefixes=("unwrap_payload","select_numeric_wide","select_numeric_narrow","select_numeric_lexical_min",
              "select_numeric_lexical_max","select_non_numeric_lexical_min","select_index:0","select_index:1",
              "select_index:2","select_index:3","select_index:4","group_count_numeric_narrow","group_count_index:0",
              "group_count_index:1","group_count_index:2","group_count_index:3","group_count_index:4")
    tested=0; frontier=[(inputs,())]; seen=set(); scored=[]; exact=[]
    for tok in prefixes:
        tested += 1
        if tested > budget: break
        st=_batch_apply(tok,inputs)
        fit=_fit((tok,),examples)
        if fit >= 1.0:
            exact.append((tok,))
        ex,sh,nu=_behavior_distance(st,target)
        key=_digest(tuple(_canon(x) for x in st))
        if key not in seen:
            seen.add(key)
            scored.append((4*ex+1.3*sh+0.6*nu-0.03,(tok,),st,fit))
    scored.sort(key=lambda z:(-z[3],-z[0],len(z[1]),_canon(z[1])))
    frontier=[(st,prog) for _,prog,st,_ in scored[:max(8,min(beam,64))]]
    seen_programs=set(exact)
    frontier_best=list(scored)
    for _depth in range(2,max_depth+1):
        next_scored=[]
        for state,prog in frontier:
            active=_fast_active_tokens(state,target,examples,prog)
            for tok in active:
                if prog and tok==prog[-1] and tok in {"sort","reverse","dedupe"}: continue
                tested += 1
                if tested > budget: break
                p=prog+(tok,)
                if p in seen_programs: continue
                seen_programs.add(p)
                st2=_batch_apply(tok,state)
                fit=_fit(p,examples)
                if fit >= 1.0:
                    exact.append(p)
                ex,sh,nu=_behavior_distance(st2,target)
                next_scored.append((4*ex+1.3*sh+0.6*nu-0.03*len(p),p,st2,fit))
            if tested > budget: break
        next_scored.sort(key=lambda z:(-z[3],-z[0],len(z[1]),_canon(z[1])))
        frontier=[(st,p) for _,p,st,_ in next_scored[:max(8,min(beam,64))]]
        frontier_best.extend(next_scored[:max_candidates*2])
        if tested > budget or not frontier: break
    # Prefer exact hypotheses, then highest observed-fit alternatives. Always dedupe.
    out=[]; seen_out=set()
    for p in sorted(set(exact),key=lambda q:(len(q),_canon(q))):
        if p not in seen_out:
            out.append(p); seen_out.add(p)
            if len(out) >= max_candidates: return tuple(out)
    for _,p,_,fit in sorted(frontier_best,key=lambda z:(-z[3],-z[0],len(z[1]),_canon(z[1]))):
        if p not in seen_out:
            out.append(p); seen_out.add(p)
            if len(out) >= max_candidates: break
    return tuple(out) if out else ((),)


def _predictive_disagreement(candidates: Sequence[tuple[str, ...]], x: Json) -> float:
    vals = [_canon(_run(p, x)) for p in candidates]
    if len(vals) < 2:
        return 0.0
    return len(set(vals)) / len(vals)



def _init_009(self: Any) -> None:
    if hasattr(self, "_asi_core_009_state"):
        return
    self._asi_core_009_state = {
        "capabilities": [],
        "diagnoses": [],
        "evidence": [],
        "outcomes": [],
        "unknown_count": 0,
        "sufficient_count": 0,
        "acquisition_count": 0,
        "transfer_success_count": 0,
        "search_work": 0,
        "evidence_queries": 0,
        "generation": 0,
        "search_policy": {"max_depth": 6, "beam": 64, "budget": 3500},
        "improvement_hypotheses": [],
        "improvement_history": [],
    }


def _ensure_009(self: Any) -> None:
    if not hasattr(self, "_asi_core_009_state"):
        _init_009(self)


def diagnose_open_capability(self: Any, *, examples: Sequence[tuple[Json, Json]], prediction: Json | None, actual: Json | None,
                             search_work: int, evidence_pool_size: int = 0) -> tuple[Diagnosis, ...]:
    _ensure_009(self)
    error = 1.0 if prediction != actual else 0.0
    candidates = [
        Diagnosis("d:"+_digest(("sufficient", len(examples)))[:16], "CURRENT_MACHINERY_APPEARS_SUFFICIENT", 0.95 if error == 0 else 0.03,
                  {"prediction_error": error}, "reuse_without_modification"),
        Diagnosis("d:"+_digest(("representation", len(examples)))[:16], "REPRESENTATION_MISMATCH", 0.30 if error else 0.05,
                  {"prediction_error": error, "evidence_pool": evidence_pool_size}, "seek_discriminating_evidence"),
        Diagnosis("d:"+_digest(("operator", search_work))[:16], "OPERATOR_GAP", min(0.70, 0.16 + search_work/3500) if error else 0.04,
                  {"search_work": search_work, "prediction_error": error}, "construct_generic_program"),
        Diagnosis("d:"+_digest(("composition", len(examples), search_work))[:16], "COMPOSITION_GAP", min(0.60, 0.12 + search_work/5500) if error else 0.04,
                  {"search_work": search_work}, "revise_program_composition"),
        Diagnosis("d:"+_digest(("evidence", evidence_pool_size, len(examples)))[:16], "INSUFFICIENT_EVIDENCE", (0.65 if error and evidence_pool_size>0 and len(examples)<=2 else min(0.45, 0.06 + evidence_pool_size/40)) if error else 0.03,
                  {"evidence_pool": evidence_pool_size, "example_count":len(examples)}, "request_more_information"),
    ]
    # No single failure pattern is hard-routed to one diagnosis; retain calibrated competition.
    total = sum(max(1e-9, d.confidence) for d in candidates)
    norm = tuple(Diagnosis(d.diagnosis_id, d.kind, d.confidence/total, d.evidence, d.action) for d in candidates)
    self._asi_core_009_state["diagnoses"].extend(d.to_dict() for d in norm)
    self._asi_core_009_state["diagnoses"] = self._asi_core_009_state["diagnoses"][-256:]
    return norm


def select_open_evidence(self: Any, *, examples: Sequence[tuple[Json, Json]], experiment_pool: Sequence[Json],
                         evidence_query: Callable[[Json], Json], max_queries: int = 1) -> tuple[dict[str, Any], ...]:
    _ensure_009(self)
    if not experiment_pool:
        return ()
    candidate_programs = list(_hypothesis_frontier(examples, budget=2200, max_depth=min(5,int(self._asi_core_009_state["search_policy"]["max_depth"])),
                                                    beam=min(64,int(self._asi_core_009_state["search_policy"]["beam"])), max_candidates=128))
    remaining=list(experiment_pool)
    selected=[]
    for _ in range(max(1, max_queries)):
        if not remaining:
            break
        scored=[]
        for i,obs in enumerate(remaining):
            disagreement=_predictive_disagreement(candidate_programs, obs)
            scored.append((disagreement, i, obs))
        scored.sort(key=lambda t:(-t[0], _digest(t[2])))
        disagreement, idx, obs = scored[0]
        actual=evidence_query(obs)
        selected.append({"observation":copy.deepcopy(obs),"outcome":copy.deepcopy(actual),"predicted_disagreement":disagreement,
                         "candidate_count":len(candidate_programs),"candidate_hypotheses":tuple(candidate_programs)})
        self._asi_core_009_state["evidence_queries"] += 1
        self._asi_core_009_state["evidence"].append(selected[-1])
        remaining.pop(idx)
    self._asi_core_009_state["evidence"] = self._asi_core_009_state["evidence"][-128:]
    return tuple(selected)


def attempt_open_capability(self: Any, task_id: str, examples: Sequence[tuple[Json, Json]], query: Json, *, budget: int = 3500) -> tuple[Json | None, dict[str, Any]]:
    """Make a prediction without access to evaluator truth."""
    _ensure_009(self)
    start=time.perf_counter()
    reused=None; base_pred=None; base_work=0; base_bottleneck=None
    for cap in reversed(self._asi_core_009_state.get("capabilities", ())):
        program=tuple(cap.get("program", ()))
        if program and _fit(program,examples)>=1.0:
            cand=_run(program,query)
            if cand is not None:
                reused={"capability_id":cap.get("capability_id"),"program":program}; base_pred=cand; break
    if reused is None:
        base_pred,base_rec=self.learn_task(task_id,examples,query,budget=min(budget,1500))
        base_work=base_rec.hypotheses_tested; base_bottleneck=base_rec.bottleneck
    attempt={"task_id":task_id,"base_prediction":copy.deepcopy(base_pred),"base_work":base_work,
             "base_bottleneck":base_bottleneck,"reused_capability":reused,"elapsed_ms":(time.perf_counter()-start)*1000}
    self._asi_core_009_state["outcomes"].append({"task_id":task_id,"phase":"PREDICTION","prediction":copy.deepcopy(base_pred),
                                                   "base_work":base_work,"reused_capability":reused})
    self._asi_core_009_state["outcomes"]=self._asi_core_009_state["outcomes"][-256:]
    return base_pred,attempt

def observe_open_feedback(self: Any, task_id: str, examples: Sequence[tuple[Json, Json]], query: Json,
                          prediction: Json | None, actual: Json, *, attempt: Mapping[str, Any] | None = None,
                          experiment_pool: Sequence[Json] = (), evidence_query: Callable[[Json], Json] | None = None,
                          budget: int = 3500, max_depth: int = 6, beam: int | None = None) -> dict[str, Any]:
    """Update from an observed outcome. `actual` is feedback, never a synthesis input."""
    _ensure_009(self)
    start=time.perf_counter()
    base_work=int((attempt or {}).get("base_work",0))
    success=(prediction==actual)
    if success:
        reused=(attempt or {}).get("reused_capability")
        status="TRANSFER_REUSE" if reused else "SUFFICIENT"
        if reused: self._asi_core_009_state["transfer_success_count"] += 1
        else: self._asi_core_009_state["sufficient_count"] += 1
        outcome={"task_id":task_id,"phase":"FEEDBACK","status":status,"prediction_correct":True,"acquired":False,
                 "reused_capability":reused,"search_work":base_work,"elapsed_ms":(time.perf_counter()-start)*1000}
        self._asi_core_009_state["outcomes"].append(outcome)
        self._asi_core_009_state["outcomes"]=self._asi_core_009_state["outcomes"][-256:]
        return outcome

    self._asi_core_009_state["unknown_count"] += 1
    diagnoses=diagnose_open_capability(self,examples=examples,prediction=prediction,actual=actual,
                                       search_work=base_work,evidence_pool_size=len(experiment_pool))
    chosen_diag=max(diagnoses,key=lambda d:d.confidence)
    evidence=()
    augmented=list(examples)
    if evidence_query is not None and experiment_pool and chosen_diag.kind in {"INSUFFICIENT_EVIDENCE","REPRESENTATION_MISMATCH","COMPOSITION_GAP"}:
        evidence=select_open_evidence(self,examples=examples,experiment_pool=experiment_pool,evidence_query=evidence_query,max_queries=1)
        for item in evidence:
            if isinstance(item["outcome"], Mapping) and "input" in item["outcome"] and "output" in item["outcome"]:
                augmented.append((item["outcome"]["input"],item["outcome"]["output"]))
    policy=dict(self._asi_core_009_state.get("search_policy",{}))
    eff_budget=min(int(budget),int(policy.get("budget",budget)))
    eff_depth=min(int(max_depth),int(policy.get("max_depth",max_depth)))
    eff_beam=int(beam or policy.get("beam",max(48,min(128,int(getattr(self,"beam",32))*4))))
    program,work,best_error=_search_program(augmented,budget=eff_budget,max_depth=eff_depth,beam=eff_beam)
    self._asi_core_009_state["search_work"] += work
    candidate=_run(program,query) if program is not None else None
    # Actual outcome is used only as environmental feedback for accept/reject; it is not used
    # to alter the synthesis search, candidate enumeration, or diagnosis hypotheses.
    acquired=False; cap=None
    validation=_fit(program,augmented) if program is not None else 0.0
    if program is not None and validation>=1.0 and candidate==actual:
        cid="cap:"+_digest((program,tuple(sorted(_canon(x) for x,_ in augmented))))[:20]
        existing={str(x.get("capability_id")) for x in self._asi_core_009_state["capabilities"]}
        if cid not in existing:
            cap=CapabilityRecord(cid,tuple(program),len(augmented),work,validation,True)
            self._asi_core_009_state["capabilities"].append(cap.to_dict())
            self._asi_core_009_state["acquisition_count"] += 1
            acquired=True
    result={"task_id":task_id,"phase":"FEEDBACK","status":"ACQUIRED" if acquired else "FAILED",
            "prediction_correct":False,"acquired":acquired,"program":program,"capability":cap.to_dict() if cap else None,
            "candidate_prediction":candidate,"diagnosis":chosen_diag.to_dict(),"diagnoses":tuple(d.to_dict() for d in diagnoses),
            "search_work":work,"best_error":best_error,"evidence":evidence,
            "elapsed_ms":(time.perf_counter()-start)*1000}
    self._asi_core_009_state["generation"] += int(acquired)
    self._asi_core_009_state["outcomes"].append({k:v for k,v in result.items() if k not in {"program","diagnoses","evidence"}})
    self._asi_core_009_state["outcomes"]=self._asi_core_009_state["outcomes"][-256:]
    return result


def learn_open_capability(self: Any, task_id: str, examples: Sequence[tuple[Json, Json]], query: Json, *, budget: int = 3500):
    return attempt_open_capability(self, task_id, examples, query, budget=budget)


def propose_open_self_improvements(self: Any, *, max_candidates: int = 4) -> tuple[dict[str, Any], ...]:
    _ensure_009(self)
    outcomes=self._asi_core_009_state.get("outcomes",())[-32:]
    failed=[o for o in outcomes if o.get("phase")=="FEEDBACK" and o.get("status")=="FAILED"]
    cur=dict(self._asi_core_009_state.get("search_policy",{}))
    sat=sum(1 for o in failed if int(o.get("search_work",0)) >= int(cur.get("budget",3500)))
    depth_pressure=sum(1 for o in failed if o.get("diagnosis",{}).get("kind") in {"COMPOSITION_GAP","OPERATOR_GAP"} and float(o.get("best_error",1.0)) > 0.0)
    props=[]
    prior_count=len(self._asi_core_009_state.get("improvement_history",()))
    prior_capabilities=len(self._asi_core_009_state.get("capabilities",()))
    if depth_pressure:
        if prior_count:
            props.append({"improvement_id":"imp:"+_digest(("depth_budget",cur,prior_count,prior_capabilities))[:16],"operation":"increase_open_search_depth_and_budget",
                          "parent":cur,"child":{**cur,"max_depth":min(8,int(cur.get("max_depth",6))+1),"budget":min(20000,int(cur.get("budget",3500))*2)},
                          "evidence":{"composition_or_operator_failures":depth_pressure,"prior_improvements":prior_count,"prior_capabilities":prior_capabilities},
                          "prediction":"a deeper composition frontier paired with additional search budget may make the next capability reachable"})
        props.append({"improvement_id":"imp:"+_digest(("depth",cur,prior_count,prior_capabilities))[:16],"operation":"increase_open_search_depth",
                      "parent":cur,"child":{**cur,"max_depth":min(8,int(cur.get("max_depth",6))+1)},
                      "evidence":{"composition_or_operator_failures":depth_pressure,"prior_improvements":prior_count,"prior_capabilities":prior_capabilities},
                      "prediction":"an additional generic composition layer may make the observed capability gap reachable"})
    if sat:
        props.append({"improvement_id":"imp:"+_digest(("budget",cur,sat,prior_count,prior_capabilities))[:16],"operation":"increase_open_search_budget",
                      "parent":cur,"child":{**cur,"budget":min(20000,int(cur.get("budget",3500))*2)},
                      "evidence":{"saturated_failures":sat,"prior_improvements":prior_count,"prior_capabilities":prior_capabilities},
                      "prediction":"more generic search work may reach hypotheses truncated by the current resource bound"})
        if not any(p.get("operation")=="increase_open_search_depth" for p in props):
            props.append({"improvement_id":"imp:"+_digest(("depth",cur,"sat",prior_count))[:16],"operation":"increase_open_search_depth",
                          "parent":cur,"child":{**cur,"max_depth":min(8,int(cur.get("max_depth",6))+1)},
                          "evidence":{"saturated_failures":sat,"prior_improvements":prior_count,"prior_capabilities":prior_capabilities},
                          "prediction":"one additional generic composition layer may make new capabilities reachable"})
    if not props:
        props.append({"improvement_id":"imp:"+_digest(("beam",cur,len(outcomes),prior_count))[:16],"operation":"increase_open_search_beam",
                      "parent":cur,"child":{**cur,"beam":min(128,int(cur.get("beam",64))+16)},
                      "evidence":{"failures":len(failed),"prior_improvements":prior_count},
                      "prediction":"wider generic behavioral frontier may reduce search omissions"})
    self._asi_core_009_state["improvement_hypotheses"]=[dict(x) for x in props[:max_candidates]]
    return tuple(props[:max_candidates])


def adopt_open_self_improvement(self: Any, proposal: Mapping[str, Any], *, observed: Mapping[str, Any]) -> bool:
    _ensure_009(self)
    cur=dict(self._asi_core_009_state.get("search_policy",{})); parent=dict(proposal.get("parent",{})); child=dict(proposal.get("child",{}))
    if parent != cur: return False
    if not bool(observed.get("regression_passed")): return False
    if not bool(observed.get("future_enabled")): return False
    if float(observed.get("work_gain",0.0)) <= 0.0 and int(observed.get("new_capabilities",0)) <= 0: return False
    self._asi_core_009_state["search_policy"]=child
    rec={"improvement_id":proposal.get("improvement_id"),"operation":proposal.get("operation"),"parent":parent,"child":child,"evidence":dict(observed)}
    self._asi_core_009_state["improvement_history"].append(rec); self._asi_core_009_state["improvement_history"]=self._asi_core_009_state["improvement_history"][-64:]
    return True

def open_capability_state(self: Any) -> dict[str, Any]:
    _ensure_009(self)
    return copy.deepcopy(self._asi_core_009_state)


def open_capability_stats(self: Any) -> dict[str, Any]:
    _ensure_009(self)
    s=self._asi_core_009_state
    return {"acquisitions":len(s["capabilities"]),"diagnoses":len(s["diagnoses"]),"evidence_queries":s["evidence_queries"],
            "search_work":s["search_work"],"sufficient":s["sufficient_count"],"unknown":s["unknown_count"],"generation":s["generation"],"self_improvements":len(s.get("improvement_history",()))}


def _export_009(self: Any, base: Mapping[str, Any]) -> dict[str, Any]:
    _ensure_009(self)
    out=copy.deepcopy(dict(base))
    out["asi_core_009"]=_canon(self._asi_core_009_state)
    # Store JSON-compatible object separately; this avoids references into mutable learner state.
    out["asi_core_009"] = copy.deepcopy(self._asi_core_009_state)
    return out


def _load_009(self: Any, state: Mapping[str, Any]) -> None:
    _ensure_009(self)
    info=dict(state.get("asi_core_009", {}) or {})
    self._asi_core_009_state={
        "capabilities":[dict(x) for x in info.get("capabilities", ())],
        "diagnoses":[dict(x) for x in info.get("diagnoses", ())],
        "evidence":[dict(x) for x in info.get("evidence", ())],
        "outcomes":[dict(x) for x in info.get("outcomes", ())],
        "unknown_count":int(info.get("unknown_count",0)),
        "sufficient_count":int(info.get("sufficient_count",0)),
        "acquisition_count":int(info.get("acquisition_count",0)),
        "transfer_success_count":int(info.get("transfer_success_count",0)),
        "search_work":int(info.get("search_work",0)),
        "evidence_queries":int(info.get("evidence_queries",0)),
        "generation":int(info.get("generation",0)),
        "search_policy":dict(info.get("search_policy",{"max_depth":6,"beam":64,"budget":3500})),
        "improvement_hypotheses":[dict(x) for x in info.get("improvement_hypotheses",())],
        "improvement_history":[dict(x) for x in info.get("improvement_history",())],
    }


# Bind once, while preserving CORE-008 behavior underneath the wrapper.
_ORIG_INIT_009 = self_init = None
try:
    from .general_engine import GeneralLearner
    _ORIG_INIT_009 = GeneralLearner.__init__
    _ORIG_EXPORT_009 = GeneralLearner.export_state
    _ORIG_FROM_STATE_009 = GeneralLearner.from_state
    def _init_bound_009(self: Any, *args: Any, **kwargs: Any):
        _ORIG_INIT_009(self,*args,**kwargs)
        _init_009(self)
    def _export_bound_009(self: Any):
        return _export_009(self,_ORIG_EXPORT_009(self))
    def _from_state_bound_009(cls: Any, spec: Mapping[str, Any], **kwargs: Any):
        obj=_ORIG_FROM_STATE_009.__func__(cls,copy.deepcopy(dict(spec)),**kwargs)
        _init_009(obj); _load_009(obj,spec)
        return obj
    if not getattr(GeneralLearner,"_asi_core_009_bound",False):
        GeneralLearner.__init__=_init_bound_009
        GeneralLearner.export_state=_export_bound_009
        GeneralLearner.from_state=classmethod(_from_state_bound_009)
        GeneralLearner.learn_open_capability=attempt_open_capability
        GeneralLearner.attempt_open_capability=attempt_open_capability
        GeneralLearner.observe_open_feedback=observe_open_feedback
        GeneralLearner.diagnose_open_capability=diagnose_open_capability
        GeneralLearner.select_open_evidence=select_open_evidence
        GeneralLearner.open_capability_state=open_capability_state
        GeneralLearner.open_capability_stats=open_capability_stats
        GeneralLearner.propose_open_self_improvements=propose_open_self_improvements
        GeneralLearner.adopt_open_self_improvement=adopt_open_self_improvement
        GeneralLearner._asi_core_009_bound=True
except Exception:
    # Import-time failure is intentionally silent for compatibility with partial source tools;
    # the experiment test imports this module directly and will expose a real failure.
    pass
