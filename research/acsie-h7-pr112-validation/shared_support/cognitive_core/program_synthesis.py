"""Generic, model-free executable program synthesis used inside GeneralLearner.

The primitives here are deliberately task-independent data/computation operations. The
learner synthesizes compositions and parameters from examples; no task-family dispatch or
semantic representation names are used.
"""
from __future__ import annotations

from dataclasses import dataclass, asdict
from hashlib import sha256
import base64
import copy
import json
import math
from collections import Counter
from typing import Any, Iterable, Sequence

Json = Any


def canon(x: Json) -> str:
    return json.dumps(x, sort_keys=True, separators=(",", ":"), ensure_ascii=True)


def digest(x: Json) -> str:
    return sha256(canon(x).encode()).hexdigest()


def scalar(v: Any) -> bool:
    return not isinstance(v, (list, dict, tuple, set))


def rows(x: Json) -> list[Any] | None:
    if not isinstance(x, list):
        return None
    return list(x)


def _group_by_index(x: Json, idx: int) -> Json | None:
    rs = rows(x)
    if rs is None:
        return None
    buckets: dict[str, tuple[Any, list[Any]]] = {}
    for row in rs:
        if not isinstance(row, (list, tuple)) or idx >= len(row):
            return None
        key = row[idx]
        ck = canon(key)
        if ck not in buckets:
            buckets[ck] = (copy.deepcopy(key), [])
        buckets[ck][1].append(copy.deepcopy(row))
    return [[key, vals] for _, (key, vals) in sorted(buckets.items(), key=lambda kv: kv[0])]


MAX_MACRO_EXPANSION_DEPTH = 8


def _apply_one(
    x: Json,
    token: str,
    *,
    macro_depth: int = 0,
    macro_stack: tuple[str, ...] = (),
) -> Json | None:
    if token.startswith("macro:"):
        if macro_depth >= MAX_MACRO_EXPANSION_DEPTH or token in macro_stack:
            return None
        try:
            raw = base64.urlsafe_b64decode(token.split(":", 1)[1].encode()).decode()
            program = tuple(json.loads(raw))
            if not all(isinstance(step, str) for step in program):
                return None
            return apply_program(
                x,
                program,
                _macro_depth=macro_depth + 1,
                _macro_stack=macro_stack + (token,),
            )
        except Exception:
            return None
    if token == "id":
        return copy.deepcopy(x)
    if token == "flatten_pairs":
        if not isinstance(x, list) or not all(isinstance(v, (list, tuple)) and len(v) == 2 for v in x):
            return None
        out=[]
        for a,b in x:
            out.extend([copy.deepcopy(a), copy.deepcopy(b)])
        return out
    if token == "count_tokens":
        if not isinstance(x, list): return None
        counts=Counter(canon(v) for v in x)
        vals={canon(v):v for v in x}
        return [[copy.deepcopy(vals[k]), counts[k]] for k in sorted(counts)]
    if token == "reverse":
        if not isinstance(x, list): return None
        return list(reversed(copy.deepcopy(x)))
    if token == "unique":
        if not isinstance(x, list): return None
        seen={}; out=[]
        for v in x:
            c=canon(v)
            if c not in seen:
                seen[c]=1; out.append(copy.deepcopy(v))
        return out
    if token == "sort":
        if not isinstance(x, list): return None
        try: return sorted(copy.deepcopy(x), key=canon)
        except Exception: return None
    if token == "sum_scalars":
        if not isinstance(x, list) or not all(isinstance(v,(int,float)) and not isinstance(v,bool) for v in x): return None
        return sum(x)
    if token == "count":
        if isinstance(x, list): return len(x)
        return None
    if token == "group_reduce_count":
        if not isinstance(x, list): return None
        out=[]
        for g in x:
            if not isinstance(g,list) or len(g)!=2 or not isinstance(g[1],list): return None
            out.append([copy.deepcopy(g[0]), len(g[1])])
        return out
    if token == "flatten":
        if not isinstance(x, list) or not all(isinstance(v, (list, tuple)) for v in x):
            return None
        out=[]
        for v in x: out.extend(copy.deepcopy(v))
        return out
    if token in {"concat", "interleave", "zip_max"}:
        if not isinstance(x, list) or len(x) != 2 or not all(isinstance(v, list) for v in x):
            return None
        a, b = x
        if token == "concat":
            return copy.deepcopy(a) + copy.deepcopy(b)
        if token == "interleave":
            if len(a) != len(b):
                return None
            out = []
            for u, v in zip(a, b):
                out.extend([copy.deepcopy(u), copy.deepcopy(v)])
            return out
        if len(a) != len(b) or not all(
            isinstance(v, (int, float)) and not isinstance(v, bool) for v in a + b
        ):
            return None
        return [max(u, v) for u, v in zip(a, b)]
    if token == "reverse_each":
        if not isinstance(x, list) or not all(isinstance(v, list) for v in x):
            return None
        return [list(reversed(copy.deepcopy(v))) for v in x]
    if token in {"prefix_sum", "differences"}:
        if not isinstance(x, list) or not all(
            isinstance(v, (int, float)) and not isinstance(v, bool) for v in x
        ):
            return None
        if token == "prefix_sum":
            total = 0
            out = []
            for value in x:
                total += value
                out.append(total)
            return out
        return [x[i + 1] - x[i] for i in range(len(x) - 1)]

    if token in {"sum", "mean", "min", "max"}:
        if not isinstance(x, list) or not x or not all(
            isinstance(v, (int, float)) and not isinstance(v, bool) for v in x
        ):
            return 0 if token == "sum" and isinstance(x, list) and not x else None
        if token == "sum":
            return sum(x)
        if token == "mean":
            return sum(x) / len(x)
        if token == "min":
            return min(x)
        return max(x)

    if token == "map_abs":
        return [abs(v) for v in x] if isinstance(x, list) and all(
            isinstance(v, (int, float)) and not isinstance(v, bool) for v in x
        ) else None

    if token == "reverse_each":
        if not isinstance(x, list) or not all(isinstance(v, list) for v in x):
            return None
        return [list(reversed(copy.deepcopy(v))) for v in x]

    if token.startswith("repeat:"):
        if not isinstance(x, list):
            return None
        n = int(token.split(":", 1)[1])
        if n <= 0:
            return None
        return copy.deepcopy(x) * n

    if token.startswith("slice:"):
        if not isinstance(x, list):
            return None
        _, start, stop = token.split(":", 2)
        return copy.deepcopy(x[int(start):int(stop)])

    if token.startswith(("filter_gt:", "filter_lt:")):
        if not isinstance(x, list):
            return None
        _, raw_i, raw_value = token.split(":", 2)
        idx = int(raw_i)
        target = json.loads(raw_value)
        if not isinstance(target, (int, float)) or isinstance(target, bool):
            return None
        out = []
        for row in x:
            if not isinstance(row, (list, tuple)) or idx >= len(row):
                return None
            ok = row[idx] > target if token.startswith("filter_gt:") else row[idx] < target
            if ok:
                out.append(copy.deepcopy(row))
        return out

    if token.startswith("sort_by:"):
        if not isinstance(x, list):
            return None
        idx = int(token.split(":", 1)[1])
        try:
            return sorted(copy.deepcopy(x), key=lambda row: canon(row[idx]) if isinstance(row, (list, tuple)) and idx < len(row) else "~")
        except Exception:
            return None

    if token.startswith("group_reduce_sum:"):
        if not isinstance(x, list):
            return None
        idx = int(token.split(":", 1)[1])
        out = []
        for group in x:
            if not isinstance(group, list) or len(group) != 2 or not isinstance(group[1], list):
                return None
            total = 0
            for row in group[1]:
                if not isinstance(row, (list, tuple)) or idx >= len(row):
                    return None
                value = row[idx]
                if not isinstance(value, (int, float)) or isinstance(value, bool):
                    return None
                total += value
            out.append([copy.deepcopy(group[0]), total])
        return out
    if token in {"mean_scalars","min_scalars","max_scalars"}:
        if not isinstance(x, list) or not x or not all(isinstance(v,(int,float)) and not isinstance(v,bool) for v in x):
            return None
        if token == "mean_scalars": return sum(x)/len(x)
        if token == "min_scalars": return min(x)
        return max(x)
    if token == "map_neg":
        return [-v for v in x] if isinstance(x,list) and all(isinstance(v,(int,float)) and not isinstance(v,bool) for v in x) else None
    if token.startswith(("map_add:","map_sub:","map_mul:")):
        if not isinstance(x,list) or not all(isinstance(v,(int,float)) and not isinstance(v,bool) for v in x):
            return None
        c=float(token.split(":",1)[1]); op=token.split(":",1)[0]
        if op=="map_add": return [v+c for v in x]
        if op=="map_sub": return [v-c for v in x]
        return [v*c for v in x]
    if token.startswith("rotate_clamped:"):
        if not isinstance(x,list): return None
        k=int(token.split(":",1)[1])
        if k < 0: return None
        if not x: return []
        k=min(k, len(x)-1)
        return copy.deepcopy(x[k:]+x[:k])
    if token.startswith("rotate:"):
        if not isinstance(x,list): return None
        if not x: return []
        k=int(token.split(":",1)[1]) % len(x)
        return copy.deepcopy(x[k:]+x[:k])
    if token == "transpose":
        if not isinstance(x,list) or not x or not all(isinstance(r,(list,tuple)) for r in x):
            return None
        # Match Python's ordinary zip(*rows) semantics for ragged rows.
        return [list(col) for col in zip(*x)]
    if token in {"zip_add","zip_sub","zip_mul","zip_max"}:
        if not isinstance(x,list) or len(x)!=2 or not all(isinstance(r,list) for r in x):
            return None
        a,b=x
        if len(a)!=len(b) or not all(isinstance(v,(int,float)) and not isinstance(v,bool) for v in a+b):
            return None
        op=token
        if op=="zip_add": return [u+v for u,v in zip(a,b)]
        if op=="zip_sub": return [u-v for u,v in zip(a,b)]
        if op=="zip_mul": return [u*v for u,v in zip(a,b)]
        return [max(u,v) for u,v in zip(a,b)]
    if token.startswith("chunk:"):
        if not isinstance(x,list): return None
        n=int(token.split(":",1)[1])
        if n<=0: return None
        return [copy.deepcopy(x[i:i+n]) for i in range(0,len(x),n)]
    if token.startswith("window:"):
        if not isinstance(x,list): return None
        n=int(token.split(":",1)[1])
        if n<=0: return None
        return [copy.deepcopy(x[i:i+n]) for i in range(max(0,len(x)-n+1))]
    if token == "dict_values":
        return [copy.deepcopy(v) for _,v in sorted(x.items(), key=lambda kv:str(kv[0]))] if isinstance(x,dict) else None
    if token == "dict_keys":
        return [copy.deepcopy(k) for k in sorted(x,key=str)] if isinstance(x,dict) else None
    if token == "dict_items":
        return [[copy.deepcopy(k),copy.deepcopy(v)] for k,v in sorted(x.items(), key=lambda kv:str(kv[0]))] if isinstance(x,dict) else None
    if token.startswith("dict_get:"):
        if not isinstance(x,dict): return None
        key=json.loads(token.split(":",1)[1])
        return copy.deepcopy(x[key]) if key in x else None

    if token.startswith("project:"):
        i=int(token.split(":",1)[1])
        rs=rows(x)
        if rs is None: return None
        out=[]
        for r in rs:
            if not isinstance(r,(list,tuple)) or i>=len(r): return None
            out.append(copy.deepcopy(r[i]))
        return out
    if token.startswith("pair:"):
        _,a,b=token.split(":")
        a=int(a); b=int(b)
        rs=rows(x)
        if rs is None: return None
        out=[]
        for r in rs:
            if not isinstance(r,(list,tuple)) or max(a,b)>=len(r): return None
            out.append([copy.deepcopy(r[a]), copy.deepcopy(r[b])])
        return out
    if token.startswith("filter_truthy:"):
        i=int(token.split(":",1)[1])
        rs=rows(x)
        if rs is None: return None
        out=[]
        for r in rs:
            if not isinstance(r,(list,tuple)) or i>=len(r): return None
            if bool(r[i]): out.append(copy.deepcopy(r))
        return out
    if token.startswith("filter_falsy:"):
        i=int(token.split(":",1)[1])
        rs=rows(x)
        if rs is None: return None
        out=[]
        for r in rs:
            if not isinstance(r,(list,tuple)) or i>=len(r): return None
            if not bool(r[i]): out.append(copy.deepcopy(r))
        return out
    if token.startswith("filter_eq:"):
        _,i,val = token.split(":",2)
        i=int(i); target=json.loads(val)
        rs=rows(x)
        if rs is None: return None
        out=[]
        for r in rs:
            if not isinstance(r,(list,tuple)) or i>=len(r): return None
            if r[i]==target: out.append(copy.deepcopy(r))
        return out
    if token.startswith("filter_neq:"):
        _,i,val = token.split(":",2)
        i=int(i); target=json.loads(val)
        rs=rows(x)
        if rs is None: return None
        out=[]
        for r in rs:
            if not isinstance(r,(list,tuple)) or i>=len(r): return None
            if r[i]!=target: out.append(copy.deepcopy(r))
        return out
    if token.startswith(("filter_gt:", "filter_lt:")):
        _, i, raw = token.split(":", 2)
        i = int(i)
        target = json.loads(raw)
        if not isinstance(target, (int, float)) or isinstance(target, bool):
            return None
        rs = rows(x)
        if rs is None:
            return None
        out = []
        for r in rs:
            if not isinstance(r, (list, tuple)) or i >= len(r):
                return None
            ok = r[i] > target if token.startswith("filter_gt:") else r[i] < target
            if ok:
                out.append(copy.deepcopy(r))
        return out
    if token.startswith("group:"):
        return _group_by_index(x,int(token.split(":",1)[1]))
    if token.startswith("group_reduce_sum:"):
        idx=int(token.split(":",1)[1])
        if not isinstance(x,list): return None
        out=[]
        for g in x:
            if not isinstance(g,list) or len(g)!=2 or not isinstance(g[1],list): return None
            total=0; saw=False
            for r in g[1]:
                if not isinstance(r,(list,tuple)) or idx>=len(r): return None
                v=r[idx]
                if not isinstance(v,(int,float)) or isinstance(v,bool): return None
                total += v; saw=True
            if not saw: total=0
            out.append([copy.deepcopy(g[0]), total])
        return out
    if token.startswith("sort_by:"):
        idx=int(token.split(":",1)[1])
        if not isinstance(x,list): return None
        try: return sorted(copy.deepcopy(x), key=lambda r:(canon(r[idx]) if isinstance(r,(list,tuple)) and idx<len(r) else "~"))
        except Exception: return None
    if token.startswith("take:"):
        n=int(token.split(":",1)[1])
        return copy.deepcopy(x[:n]) if isinstance(x,list) else None
    if token.startswith("drop:"):
        n=int(token.split(":",1)[1])
        return copy.deepcopy(x[n:]) if isinstance(x,list) else None
    return None


def apply_program(
    x: Json,
    program: Sequence[str],
    *,
    _macro_depth: int = 0,
    _macro_stack: tuple[str, ...] = (),
) -> Json | None:
    out=copy.deepcopy(x)
    for token in program:
        out=_apply_one(
            out,
            token,
            macro_depth=_macro_depth,
            macro_stack=_macro_stack,
        )
        if out is None: return None
    return out


def _type_shape(x: Json):
    if isinstance(x,list):
        return ("list", len(x), _type_shape(x[0]) if x else "empty")
    if isinstance(x,dict): return ("dict", tuple(sorted((str(k),_type_shape(v)) for k,v in x.items())))
    if isinstance(x,tuple): return ("tuple",len(x),_type_shape(x[0]) if x else "empty")
    if isinstance(x,bool): return "bool"
    if isinstance(x,(int,float)): return "number"
    return type(x).__name__


def structural_distance(a: Json, b: Json) -> float:
    if a == b: return 0.0
    if _type_shape(a) == _type_shape(b): base=0.25
    else: base=0.75
    if isinstance(a,list) and isinstance(b,list):
        if not a and not b: return base
        len_pen=abs(len(a)-len(b))/max(1,len(a),len(b))
        common=min(len(a),len(b))
        value_pen=0.0
        for i in range(common):
            if a[i] != b[i]: value_pen += 0.1
        return min(1.0,base+0.25*len_pen+value_pen/max(1,common))
    if isinstance(a,dict) and isinstance(b,dict):
        return min(1.0,base+0.05*len(set(a)^set(b)))
    return min(1.0, base + 0.25)


def encode_macro(program: Sequence[str]) -> str:
    payload = json.dumps(tuple(program), separators=(",", ":"), ensure_ascii=True)
    return "macro:" + base64.urlsafe_b64encode(payload.encode()).decode()


def _parameter_tokens(
    examples: Sequence[tuple[Json,Json]],
    library: Sequence[str] = (),
) -> tuple[str,...]:
    indices=set()
    for x,_ in examples:
        if isinstance(x,list):
            maxw=max((len(r) for r in x if isinstance(r,(list,tuple))),default=0)
            for i in range(min(6,maxw)): indices.add(i)
    toks={"id","flatten_pairs","count_tokens","unique","sort","sum_scalars","mean_scalars","min_scalars","max_scalars","count","group_reduce_count","reverse","flatten","concat","interleave","reverse_each","prefix_sum","differences","map_neg","transpose","zip_add","zip_sub","zip_mul","zip_max","dict_values","dict_keys","dict_items"}
    toks.update(str(token) for token in library if isinstance(token, str) and token.startswith("macro:"))
    numeric_constants={0,1,-1,2,-2,3,-3}
    lengths={1,2,3}
    dict_keys=set()
    for x,y in examples:
        for value in (x,y):
            if isinstance(value,list):
                lengths.add(len(value))
                for row in value:
                    if isinstance(row,(list,tuple)):
                        for v in row:
                            if isinstance(v,(int,float)) and not isinstance(v,bool) and abs(float(v))<=16: numeric_constants.add(v)
                    if isinstance(row,dict): dict_keys.update(row.keys())
            if isinstance(value,dict): dict_keys.update(value.keys())
            if isinstance(value,(int,float)) and not isinstance(value,bool) and abs(float(value))<=16: numeric_constants.add(value)
    for n in sorted(lengths):
        if 0<n<=8:
            toks.update({f"take:{n}",f"drop:{n}",f"chunk:{n}",f"window:{n}",f"rotate:{n}",f"rotate:{-n}"})
    for c in sorted(numeric_constants,key=lambda z:(float(z),str(type(z)))):
        toks.update({f"map_add:{c}",f"map_sub:{c}",f"map_mul:{c}"})
    for key in sorted(dict_keys,key=lambda z:canon(z)):
        toks.add("dict_get:"+json.dumps(key,sort_keys=True,separators=(",",":")))
    for i in sorted(indices):
        toks.add(f"project:{i}")
        toks.add(f"group:{i}")
        toks.add(f"sort_by:{i}")
        toks.add(f"group_reduce_sum:{i}")
        toks.add(f"filter_truthy:{i}")
        toks.add(f"filter_falsy:{i}")
        for c in sorted(numeric_constants, key=lambda z:(float(z),str(type(z)))):
            raw = json.dumps(c, sort_keys=True, separators=(",", ":"))
            toks.add(f"filter_gt:{i}:{raw}")
            toks.add(f"filter_lt:{i}:{raw}")
        for j in sorted(indices):
            toks.add(f"pair:{i}:{j}")
    return tuple(sorted(toks))


@dataclass(frozen=True)
class SynthesizedProgram:
    program_id: str
    steps: tuple[str,...]
    training_error: float
    holdout_error: float
    complexity: int
    trials: int

    def apply(self,x:Json)->Json|None:
        return apply_program(x,self.steps)

    def spec(self):
        return asdict(self)


def _prefix_signature(program: Sequence[str], width: int = 2) -> tuple[str, ...]:
    return tuple(
        token.split(":", 1)[0]
        for token in tuple(program)[: max(1, min(int(width), len(tuple(program))))]
    )


def _select_diverse_frontier(
    candidates: Sequence[tuple[float, tuple[str, ...], float, float]],
    beam: int,
    diversity: float,
) -> list[tuple[str, ...]]:
    ordered = sorted(candidates, key=lambda q: (q[0], q[1]))
    limit = max(1, min(len(ordered), int(beam)))
    strength = max(0.0, min(1.0, float(diversity)))
    novelty_slots = min(limit, int(round(limit * strength)))
    selected: list[tuple[float, tuple[str, ...], float, float]] = []
    selected_ids: set[tuple[str, ...]] = set()
    seen_signatures: set[tuple[str, ...]] = set()

    if novelty_slots:
        for item in ordered:
            sig = _prefix_signature(item[1], width=1)
            if sig in seen_signatures:
                continue
            seen_signatures.add(sig)
            selected.append(item)
            selected_ids.add(item[1])
            if len(selected) >= novelty_slots:
                break

    for item in ordered:
        if len(selected) >= limit:
            break
        if item[1] in selected_ids:
            continue
        selected.append(item)
        selected_ids.add(item[1])

    return [item[1] for item in selected]


def synthesize_program(
    examples: Sequence[tuple[Json,Json]],
    *,
    max_depth:int=5,
    beam:int=64,
    holdout_fraction:float=0.34,
    library: Sequence[str] = (),
    beam_growth:int=0,
    frontier_diversity:float=0.0,
) -> tuple[SynthesizedProgram|None,int]:
    if not examples: return None,0
    toks=_parameter_tokens(examples, library)
    cut=max(1,int(len(examples)*(1.0-holdout_fraction)))
    train=examples[:cut]; hold=examples[cut:]
    trials=0
    frontier=[tuple()]
    seen={tuple()}
    best=None
    for depth in range(1,max_depth+1):
        candidates=[]
        for prefix in frontier:
            for tok in toks:
                if prefix and tok==prefix[-1] and tok in {"id","sort","unique","flatten_pairs"}: continue
                p=prefix+(tok,)
                if p in seen: continue
                seen.add(p); trials += 1
                train_d=[]; hold_d=[]; valid=True
                for x,y in train:
                    z=apply_program(x,p)
                    if z is None: valid=False; break
                    train_d.append(structural_distance(z,y))
                if not valid: continue
                for x,y in hold:
                    z=apply_program(x,p)
                    if z is None: valid=False; break
                    hold_d.append(structural_distance(z,y))
                if not valid: continue
                tr=sum(train_d)/max(1,len(train_d)); ho=sum(hold_d)/max(1,len(hold_d))
                score=0.58*ho+0.42*tr+0.004*len(p)
                candidates.append((score,p,tr,ho))
                if ho==0.0 and tr==0.0:
                    pid="gprog:"+sha256(canon(p).encode()).hexdigest()[:16]
                    return SynthesizedProgram(pid,p,tr,ho,len(p),trials),trials
        candidates.sort(key=lambda q:(q[0],q[1]))
        depth_beam = max(1, min(len(candidates), int(beam) + max(0, int(beam_growth)) * max(0, depth - 1)))
        frontier=_select_diverse_frontier(candidates, depth_beam, frontier_diversity)
        if candidates:
            score,p,tr,ho=candidates[0]
            if best is None or score<best[0]: best=(score,p,tr,ho)
        if not frontier: break
    if best is None: return None,trials
    score,p,tr,ho=best
    if ho <= 0.15 and tr <= 0.15:
        pid="gprog:"+sha256(canon(p).encode()).hexdigest()[:16]
        return SynthesizedProgram(pid,p,tr,ho,len(p),trials),trials
    return None,trials
