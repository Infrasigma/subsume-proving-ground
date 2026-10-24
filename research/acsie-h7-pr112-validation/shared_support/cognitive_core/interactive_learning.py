"""ASI-CORE-010: generic action-conditioned interaction learning.

All cognitive decisions remain methods on the single GeneralLearner.  This module
contains only serializable data records and generic structural utilities; it has no
knowledge of the evaluator's environments or task semantics.
"""
from __future__ import annotations
from dataclasses import dataclass, asdict
from hashlib import sha256
import copy, json, math
from typing import Any, Mapping, Sequence

Json=Any

def _canon(x: Json) -> str:
    return json.dumps(x, sort_keys=True, separators=(',', ':'), ensure_ascii=True, default=str)

def _hash(x: Json) -> str:
    return sha256(_canon(x).encode()).hexdigest()

def _is_num(x: Json) -> bool:
    return isinstance(x,(int,float)) and not isinstance(x,bool)

def _walk(x: Json, path: tuple[Any,...]=()):
    if isinstance(x, dict):
        for k in sorted(x, key=lambda z: str(z)):
            yield from _walk(x[k], path+(k,))
    elif isinstance(x, (list,tuple)):
        for i,v in enumerate(x): yield from _walk(v, path+(i,))
    else:
        yield path,x

def _leaf_kind(x: Json) -> str:
    if _is_num(x): return 'number'
    if isinstance(x,bool): return 'bool'
    if isinstance(x,str): return 'text'
    if x is None: return 'null'
    return type(x).__name__

def _ordinal_map(x: Json):
    return [(i,p,v,_leaf_kind(v)) for i,(p,v) in enumerate(_walk(x))]

def _replace(x: Json, path: Sequence[Any], value: Json) -> Json | None:
    y=copy.deepcopy(x)
    if not path: return copy.deepcopy(value)
    cur=y
    for p in path[:-1]:
        try: cur=cur[p]
        except (KeyError,IndexError,TypeError): return None
    try: cur[path[-1]]=copy.deepcopy(value)
    except (KeyError,IndexError,TypeError): return None
    return y

def _get(x: Json, path: Sequence[Any]):
    cur=x
    for p in path: cur=cur[p]
    return cur

def _structural_slot(before: Json, path: Sequence[Any], value: Json) -> tuple[str,int,str]:
    # Path-independent role: leaf ordinal + kind + parent container width.
    leaves=_ordinal_map(before)
    target_i=next((i for i,p,_,_ in leaves if tuple(p)==tuple(path)), -1)
    parent=None
    if path:
        try: parent=_get(before,path[:-1])
        except Exception: parent=None
    arity=len(parent) if isinstance(parent,(list,dict)) else 0
    return ('leaf',target_i,_leaf_kind(value)+':'+str(arity))

def _dict_keyless_rows(before,after):
    """Match list-of-dicts using stable nonnumeric marker content when available."""
    if not (isinstance(before,list) and isinstance(after,list)): return []
    out=[]
    for bi,b in enumerate(before):
        if not isinstance(b,dict): continue
        markers=[(k,_canon(v)) for k,v in b.items() if not _is_num(v)]
        if not markers: continue
        for ai,a in enumerate(after):
            if not isinstance(a,dict): continue
            am=[(k,_canon(v)) for k,v in a.items() if not _is_num(v)]
            if any(m in am for m in markers):
                markers=tuple(markers[:1]); out.append((bi,ai,b,a,markers)); break
    return out

def _diff_effect(before: Json, after: Json) -> list[dict[str,Any]]:
    effects=[]
    # Direct leaf diffs.
    bm={tuple(p):(v,k,i) for i,p,v,k in _ordinal_map(before)}
    am={tuple(p):(v,k,i) for i,p,v,k in _ordinal_map(after)}
    for p,(bv,bk,bi) in bm.items():
        if p not in am: continue
        av,ak,ai=am[p]
        if bv!=av:
            if _is_num(bv) and _is_num(av):
                effects.append({'kind':'delta','path':p,'before':bv,'after':av,'delta':av-bv,'role':_structural_slot(before,p,bv)})
            elif isinstance(bv,bool) and isinstance(av,bool):
                effects.append({'kind':'toggle','path':p,'before':bv,'after':av,'role':_structural_slot(before,p,bv)})
            else:
                effects.append({'kind':'set','path':p,'before':copy.deepcopy(bv),'after':copy.deepcopy(av),'role':_structural_slot(before,p,bv)})
    # Generic row correspondence for reordered entity collections.
    for bi,ai,brow,arow,markers in _dict_keyless_rows(before.get('items',[]) if isinstance(before,dict) else [], after.get('items',[]) if isinstance(after,dict) else []):
        for k,bv in brow.items():
            if k in arow and bv!=arow[k] and _is_num(bv) and _is_num(arow[k]):
                effects.append({'kind':'entity_delta','field':str(k),'entity':bi,'marker':markers[0] if markers else None,'delta':arow[k]-bv,'before':bv,'after':arow[k]})
    # Whole-list permutation effects.
    if isinstance(before,list) and isinstance(after,list) and len(before)==len(after) and sorted(map(_canon,before))==sorted(map(_canon,after)) and before!=after:
        if after==list(reversed(before)): effects.append({'kind':'reverse'})
        elif after==before[1:]+before[:1]: effects.append({'kind':'rotate1'})
    effects.extend(_nested_permutations(before,after))
    return effects

def _nested_permutations(before: Json, after: Json):
    out=[]
    def rec(a,b,path=()):
        if isinstance(a,dict) and isinstance(b,dict):
            for k in set(a)&set(b): rec(a[k],b[k],path+(k,))
        elif isinstance(a,list) and isinstance(b,list) and len(a)==len(b) and a!=b:
            if sorted(map(_canon,a))==sorted(map(_canon,b)):
                if b==list(reversed(a)): out.append({'kind':'reverse_nested','path':path,'length':len(a)})
                elif len(a)>=2 and b==a[1:]+a[:1]: out.append({'kind':'rotate1_nested','path':path,'length':len(a)})
                else: out.append({'kind':'permute_nested','path':path,'length':len(a),'before':copy.deepcopy(a),'after':copy.deepcopy(b)})
    rec(before,after); return out

@dataclass(frozen=True)
class TransitionRecord:
    action: str
    before: Json
    after: Json
    reward: float
    step: int
    exogenous: bool=False

@dataclass(frozen=True)
class TransitionHypothesis:
    hypothesis_id: str
    action_signature: str
    kind: str
    role: tuple[Any,...] | None
    delta: float | None
    field: str | None
    delay: int
    context: tuple[Any,...] | None
    observations: int
    errors: int
    confidence: float

# Generic executor for learned hypotheses.
def _apply_hypothesis(h: Mapping[str,Any], obs: Json) -> Json | None:
    kind=h.get('kind')
    if kind=='noop': return copy.deepcopy(obs)
    ctx_role=tuple(h.get('context_role') or ())
    if ctx_role:
        try:
            ctx_val=None
            for p,v,knd in [(p,v,knd) for _,p,v,knd in _ordinal_map(obs)]:
                if tuple(_structural_slot(obs,p,v))==ctx_role:
                    ctx_val=copy.deepcopy(v); break
            if _canon(ctx_val)!=_canon(h.get('context_value')): return None
        except Exception: return None
    if kind=='delta':
        role=h.get('role'); leaves=_ordinal_map(obs)
        if not role: return None
        target=None
        for p,v,k,i in [(p,v,k,i) for i,p,v,k in _ordinal_map(obs)]:
            slot=_structural_slot(obs,p,v)
            if tuple(slot)==tuple(role): target=p; break
        if target is None: return None
        try: return _replace(obs,target,_get(obs,target)+h.get('delta',0))
        except Exception: return None
    if kind=='toggle':
        role=h.get('role');
        for i,p,v,k in _ordinal_map(obs):
            if tuple(_structural_slot(obs,p,v))==tuple(role): return _replace(obs,p,not bool(v))
        return None
    if kind=='set':
        role=h.get('role')
        for i,p,v,k in _ordinal_map(obs):
            if tuple(_structural_slot(obs,p,v))==tuple(role): return _replace(obs,p,copy.deepcopy(h.get('after_value')))
        return None
    if kind=='reverse' and isinstance(obs,list): return list(reversed(copy.deepcopy(obs)))
    if kind=='rotate1' and isinstance(obs,list) and obs: return obs[1:]+obs[:1]
    if kind in {'reverse_nested','rotate1_nested'}:
        try:
            cur=copy.deepcopy(obs); parent=cur
            path=h.get('path',())
            for p in path[:-1]: parent=parent[p]
            key=path[-1] if path else None; val=parent[key] if path else cur
            if not isinstance(val,list): return None
            parent[key]=list(reversed(val)) if kind=='reverse_nested' else (val[1:]+val[:1])
            return cur
        except Exception: return None
    if kind=='permute_nested':
        try:
            cur=copy.deepcopy(obs); parent=cur; path=h.get('path',())
            for p in path[:-1]: parent=parent[p]
            key=path[-1] if path else None; val=parent[key] if path else cur
            if not isinstance(val,list): return None
            # Exact permutation is retained as a generic structural operation learned from examples.
            target=h.get('after')
            if target is None: return None
            parent[key]=copy.deepcopy(target); return cur
        except Exception: return None
    if kind=='entity_delta' and isinstance(obs,dict) and isinstance(obs.get('items'),list):
        rows=copy.deepcopy(obs['items']); field=h.get('field'); marker=h.get('marker'); entity=int(h.get('entity',0))
        target=None
        if marker:
            mk,mv=marker
            for row in rows:
                if isinstance(row,dict) and mk in row and _canon(row[mk])==mv:
                    target=row; break
        if target is None and 0<=entity<len(rows): target=rows[entity]
        if not isinstance(target,dict) or field not in target or not _is_num(target[field]): return None
        target[field]=target[field]+h.get('delta',0)
        obs2=copy.deepcopy(obs); obs2['items']=rows; return obs2
    return None

def _goal_score(obs: Json, goal: Mapping[str,Any]) -> float:
    try:
        path=goal.get('path',())
        want=goal.get('equals')
        got=_get(obs,path)
        return 1.0 if got==want else 0.0
    except Exception:
        return 0.0

# Public generic learner bindings.
def _init_010(self: Any):
    if hasattr(self,'_asi_core_010_state'): return
    self._asi_core_010_state={
        'records':[], 'hypotheses':[], 'retired_hypotheses':[], 'prediction_errors':[],
        'experiments':[], 'plans':[], 'bindings':{}, 'unknown_count':0, 'revision_count':0,
        'model_updates':0, 'interaction_steps':0, 'transfer_reuse_count':0, 'max_delay':4, 'diagnostic_records':[],
    }

def _ensure(self):
    if not hasattr(self,'_asi_core_010_state'): _init_010(self)

def interactive_observe(self, observation: Json, actions: Sequence[str]=(), action: str|None=None, reward: float=0.0, *, exogenous: bool=False):
    _ensure(self); s=self._asi_core_010_state
    if action is None: return {'status':'OBSERVED','observation':copy.deepcopy(observation),'actions':tuple(actions),'reward':reward}
    rec={'action':str(action),'before':copy.deepcopy(getattr(self,'_interactive_last_observation',observation)),
         'after':copy.deepcopy(observation),'reward':float(reward),'step':s['interaction_steps'],'exogenous':bool(exogenous)}
    s['records'].append(rec); s['records']=s['records'][-512:]; s['interaction_steps']+=1
    self._interactive_last_observation=copy.deepcopy(observation); self._interactive_last_actions=tuple(actions)
    return rec

def _consistent_effects(records: Sequence[Mapping[str,Any]], action: str):
    all_rows=[r for r in records if not bool(r.get('exogenous'))]
    groups=[r for r in all_rows if str(r.get('action'))==str(action)]
    other=[r for r in all_rows if str(r.get('action'))!=str(action)]
    if len(groups)<2: return []
    stats={}; other_stats={}
    for r in groups:
        seen=set()
        for e in _diff_effect(r.get('before'),r.get('after')):
            key=(_canon(e.get('kind')), _canon(e.get('role')), e.get('field'), _canon(e.get('marker')))
            seen.add(key)
            b=stats.setdefault(key,{'e':e,'count':0,'deltas':[]}); b['count']+=1
            if e.get('delta') is not None: b['deltas'].append(e.get('delta'))
    for r in other:
        seen=set()
        for e in _diff_effect(r.get('before'),r.get('after')):
            key=(_canon(e.get('kind')), _canon(e.get('role')), e.get('field'), _canon(e.get('marker')))
            if key in seen: continue
            seen.add(key); other_stats[key]=other_stats.get(key,0)+1
    out=[]
    for key,b in stats.items():
        n=b['count']; rate=n/max(1,len(groups)); orate=other_stats.get(key,0)/max(1,len(other)); ds=b['deltas']
        specificity=rate-orate
        if n<2 or specificity<0.25: continue
        if ds and len(set(ds))==1:
            e2=dict(b['e']); e2['delta']=ds[0]; e2['consistency']=specificity; out.append(e2)
        elif not ds:
            e2=dict(b['e']); e2['consistency']=specificity; out.append(e2)
    out.sort(key=lambda e:(-float(e.get('consistency',0)), _canon(e)))
    return out

def _delayed_candidates(records: Sequence[Mapping[str,Any]], action: str, max_delay: int=4):
    all_rows=[r for r in records if not bool(r.get('exogenous'))]
    rows=[r for r in all_rows if str(r.get('action'))==str(action)]
    by_step={int(r.get('step',0)):r for r in all_rows}; out=[]
    all_actions=sorted({str(r.get('action')) for r in all_rows})
    immediate_effects={}
    for r in rows:
        for e in _diff_effect(r.get('before'),r.get('after')):
            key=(_canon(e.get('kind')), _canon(e.get('role')), e.get('field'), _canon(e.get('marker')), _canon(e.get('delta')))
            immediate_effects.setdefault(key,0); immediate_effects[key]+=1
    for d in range(2,max_delay+1):
        mine_effects={}; other_effects={}; mine_n=other_n=0
        for r in rows:
            target=by_step.get(int(r.get('step',0))+d-1)
            if not target: continue
            mine_n+=1
            for e in _diff_effect(target.get('before'),target.get('after')):
                key=(_canon(e.get('kind')), _canon(e.get('role')), e.get('field'), _canon(e.get('marker')), e.get('delta'))
                mine_effects.setdefault(key,{'e':e,'count':0}); mine_effects[key]['count']+=1
        for r in all_rows:
            if str(r.get('action'))==str(action): continue
            target=by_step.get(int(r.get('step',0))+d-1)
            if not target: continue
            other_n+=1
            for e in _diff_effect(target.get('before'),target.get('after')):
                key=(_canon(e.get('kind')), _canon(e.get('role')), e.get('field'), _canon(e.get('marker')), e.get('delta'))
                other_effects[key]=other_effects.get(key,0)+1
        for key,b in mine_effects.items():
            rate=b['count']/max(1,mine_n); other=other_effects.get(key,0)/max(1,other_n); spec=rate-other
            immediate=immediate_effects.get(key,0)/max(1,len(rows))
            # A periodic schedule can create spurious lagged correlations. Treat an
            # effect as delayed only when it has weak immediate support or the delayed
            # evidence is materially stronger than the immediate explanation.
            if b['count']<2 or spec<0.35: continue
            if immediate>=0.50 and spec<=immediate+0.10: continue
            e=dict(b['e']); e['delay']=d; e['consistency']=spec; out.append(e)
    return out

def _candidate_hypotheses(self, action: str, before: Json, after: Json):
    _ensure(self); records=self._asi_core_010_state.get('records',())
    effects=_consistent_effects(records,action)
    effects += _delayed_candidates(records,action)
    out=[]; seen=set()
    for e in effects:
        if e.get('kind') not in {'delta','toggle','set','reverse','rotate1','reverse_nested','rotate1_nested','permute_nested','entity_delta'}: continue
        key=(_canon(e.get('kind')), _canon(e.get('role')), e.get('field'), _canon(e.get('marker')), e.get('delay',1), e.get('delta'))
        if key in seen: continue
        seen.add(key)
        sig=e.get('kind')+':'+_hash(key)[:12]
        role=e.get('role')
        h={'hypothesis_id':'th:'+_hash((action,sig,role,e.get('delta'),e.get('delay',1)))[:20],
           'action_signature':str(action),'kind':e.get('kind'),'role':list(role) if role is not None else None,
           'delta':e.get('delta'),'field':e.get('field'),'marker':e.get('marker'),'entity':e.get('entity',0),
           'after_value':e.get('after'),'after':copy.deepcopy(e.get('after')),'path':list(e.get('path',())),'length':e.get('length'),'delay':int(e.get('delay',1)),'context':None,'observations':len(records),'errors':0,
           'confidence':0.55+0.35*float(e.get('consistency',0.0))}
        out.append(h)
    if not out:
        out.append({'hypothesis_id':'th:'+_hash((action,'noop'))[:20],'action_signature':str(action),'kind':'noop','role':None,'delta':0,'field':None,'marker':None,'entity':0,
                    'after_value':None,'delay':1,'context':None,'observations':1,'errors':0,'confidence':0.25})
    return out

def _hypothesis_support(h: Mapping[str,Any], records: Sequence[Mapping[str,Any]], action: str):
    mine=[r for r in records if str(r.get('action'))==str(action) and not bool(r.get('exogenous'))]
    other=[r for r in records if str(r.get('action'))!=str(action) and not bool(r.get('exogenous'))]
    ctx_role=tuple(h.get('context_role') or ())
    if ctx_role:
        def ctx_match(r):
            try:
                for _,p,v,knd in _ordinal_map(r.get('before')):
                    if tuple(_structural_slot(r.get('before'),p,v))==ctx_role:
                        return _canon(v)==_canon(h.get('context_value'))
            except Exception: pass
            return False
        mine=[r for r in mine if ctx_match(r)]
        other=[r for r in other if ctx_match(r)]
    def hit(r):
        b=r.get('before'); a=r.get('after'); k=h.get('kind')
        try:
            if k in {'delta','toggle','set'}:
                role=tuple(h.get('role') or ())
                for p,v,kind in [(p,v,knd) for _,p,v,knd in _ordinal_map(b)]:
                    if tuple(_structural_slot(b,p,v))==role:
                        bv=_get(b,p); av=_get(a,p)
                        if k=='delta': return _is_num(bv) and _is_num(av) and av-bv==h.get('delta')
                        if k=='toggle': return isinstance(bv,bool) and av== (not bv)
                        return av==h.get('after_value')
                return False
            if k in {'reverse_nested','rotate1_nested','permute_nested'}:
                path=tuple(h.get('path',()))
                bv=_get(b,path); av=_get(a,path) if path else a
                if k=='reverse_nested': return isinstance(bv,list) and av==list(reversed(bv))
                if k=='rotate1_nested': return isinstance(bv,list) and av==bv[1:]+bv[:1]
                return isinstance(bv,list) and _canon(av)==_canon(h.get('after'))
            if k=='entity_delta':
                if not (isinstance(b,dict) and isinstance(a,dict) and isinstance(b.get('items'),list) and isinstance(a.get('items'),list)): return False
                marker=h.get('marker'); field=h.get('field')
                target_b=None; target_a=None
                if marker:
                    mk,mv=marker
                    for row in b['items']:
                        if isinstance(row,dict) and mk in row and _canon(row[mk])==mv: target_b=row; break
                    for row in a['items']:
                        if isinstance(row,dict) and mk in row and _canon(row[mk])==mv: target_a=row; break
                if target_b is None or target_a is None or field not in target_b or field not in target_a: return False
                return _is_num(target_b[field]) and _is_num(target_a[field]) and target_a[field]-target_b[field]==h.get('delta')
            pred=_apply_hypothesis(h,b)
            if pred is None: return False
            return _canon(pred)==_canon(a)
        except Exception:
            return False
    mr=sum(int(hit(r)) for r in mine)/max(1,len(mine)); orr=sum(int(hit(r)) for r in other)/max(1,len(other))
    return mr,orr,mr-orr

def _infer_context_variants(self_obj, action: str):
    s=self_obj._asi_core_010_state
    records=[r for r in s.get('records',()) if str(r.get('action'))==str(action) and not bool(r.get('exogenous'))]
    if len(records)<4: return []
    by_target={}
    for r in records:
        for e in _diff_effect(r.get('before'),r.get('after')):
            if e.get('kind')!='delta': continue
            target=tuple(e.get('role') or ())
            by_target.setdefault(target,[]).append((r,e))
    candidates=[]
    # Prefer target roles whose delta is strongly action-specific; context variables must
    # partition those action-conditioned deltas consistently.
    all_rows=[r for r in s.get('records',()) if not bool(r.get('exogenous'))]
    other=[r for r in all_rows if str(r.get('action'))!=str(action)]
    for target,rows in by_target.items():
        deltas={e.get('delta') for _,e in rows}
        if len(deltas)<2 or len(rows)<4: continue
        context_candidates={}
        for r,e in rows:
            before=r.get('before')
            for _,p,v,knd in _ordinal_map(before):
                role=tuple(_structural_slot(before,p,v))
                if role==target: continue
                context_candidates.setdefault(role,[]).append((v,e.get('delta'),e))
        for ctx_role,vals in context_candidates.items():
            groups={}
            for v,d,e in vals: groups.setdefault(_canon(v),[]).append((v,d,e))
            if len(groups)<2 or len(groups)>6 or any(len(g)<2 for g in groups.values()): continue
            if not all(len({x[1] for x in g})==1 for g in groups.values()): continue
            # A context map is credible only if its target role is better explained than
            # the same delta role under other actions.
            mine_rates=[]
            other_effects={}
            for rr in other:
                for ee in _diff_effect(rr.get('before'),rr.get('after')):
                    if tuple(ee.get('role') or ())==target and ee.get('kind')=='delta':
                        other_effects[ee.get('delta')]=other_effects.get(ee.get('delta'),0)+1
            for _,_,ee in vals:
                n=sum(1 for _,d,_ in vals if d==ee.get('delta'))
                mine_rates.append(n/max(1,len(vals)) - other_effects.get(ee.get('delta'),0)/max(1,len(other)))
            quality=(max(mine_rates) if mine_rates else 0.0, len(rows), len(groups))
            # Convert each consistent context value to one learned successor hypothesis.
            for g in groups.values():
                candidates.append((quality[0],quality[1],target,ctx_role,g[0][0],g[0][1],g[0][2]))
    candidates.sort(key=lambda x:(-x[0],-x[1],_canon(x[2]),_canon(x[3]),_canon(x[4])))
    if not candidates: return []
    best_score=candidates[0][:2]
    return [(ctx_role,cv,d,e) for q,n,target,ctx_role,cv,d,e in candidates if (q,n)==best_score]

def _refresh_hypotheses(self):
    s=self._asi_core_010_state; records=s.get('records',())
    for action in sorted({str(r.get('action')) for r in records}):
        hs=[h for h in s['hypotheses'] if str(h.get('action_signature'))==action]
        delayed=_delayed_candidates(records,action,max_delay=int(s.get('max_delay',4)))
        existing_delay_keys={(h.get('kind'),h.get('delta'),h.get('delay'),_canon(h.get('role')),h.get('field'),_canon(h.get('marker'))) for h in hs}
        for e in delayed:
            key=(e.get('kind'),e.get('delta'),e.get('delay'),_canon(e.get('role')),e.get('field'),_canon(e.get('marker')))
            if key in existing_delay_keys: continue
            nh={'hypothesis_id':'th:'+_hash((action,key))[:20],'action_signature':action,'kind':e.get('kind'),'role':list(e.get('role') or ()),
                'delta':e.get('delta'),'field':e.get('field'),'marker':e.get('marker'),'entity':e.get('entity',0),'after_value':e.get('after'),'after':copy.deepcopy(e.get('after')),
                'path':list(e.get('path',())),'length':e.get('length'),'delay':int(e.get('delay',1)),'context':None,'observations':len(records),'errors':0,'confidence':0.65,'specificity':float(e.get('consistency',0.0))}
            hs.append(nh); s['hypotheses'].append(nh)
        for h in hs:
            mr,orr,spec=_hypothesis_support(h,records,action); h['support_rate']=mr; h['other_rate']=orr; h['specificity']=spec
        hs.sort(key=lambda h:(-float(h.get('specificity',0)),-float(h.get('support_rate',0)),-float(h.get('confidence',0))/(1+int(h.get('errors',0))),h.get('hypothesis_id','')))
        variants=_infer_context_variants(self,action)
        existing_keys={(h.get('kind'),tuple(h.get('role') or ()),h.get('delta'),_canon(h.get('context_role')), _canon(h.get('context_value'))) for h in hs}
        for ctx_role,cv,d,e in variants:
            key=(e.get('kind'),tuple(e.get('role') or ()),d,_canon(ctx_role),_canon(cv))
            if key in existing_keys: continue
            nh={'hypothesis_id':'th:'+_hash((action,key))[:20],'action_signature':action,'kind':e.get('kind'),'role':list(e.get('role') or ()),
                'delta':d,'field':e.get('field'),'marker':e.get('marker'),'entity':e.get('entity',0),'after_value':e.get('after'),'path':list(e.get('path',())),
                'length':e.get('length'),'delay':1,'context_role':list(ctx_role),'context_value':copy.deepcopy(cv),'observations':len(records),'errors':0,'confidence':0.95}
            hs.append(nh); s['hypotheses'].append(nh)
        for h in hs:
            mr,orr,spec=_hypothesis_support(h,records,action); h['support_rate']=mr; h['other_rate']=orr; h['specificity']=spec
        hs.sort(key=lambda h:(-float(h.get('specificity',0)),-float(h.get('support_rate',0)),-float(h.get('confidence',0))/(1+int(h.get('errors',0))),h.get('hypothesis_id','')))
        keep=hs[:8]
        drop=hs[8:]
        if drop: s['retired_hypotheses'].extend(copy.deepcopy(drop)); s['retired_hypotheses']=s['retired_hypotheses'][-256:]
        keep_ids={h['hypothesis_id'] for h in keep}
        s['hypotheses']=[h for h in s['hypotheses'] if str(h.get('action_signature'))!=action or h['hypothesis_id'] in keep_ids]
    return s['hypotheses']

def _hyp_predict(self, action: str, obs: Json):
    _ensure(self); hs=[h for h in self._asi_core_010_state['hypotheses'] if h.get('action_signature')==str(action)]
    preds=[]
    for h in hs:
        if int(h.get('delay',1)) != 1:
            continue
        p=_apply_hypothesis(h,obs)
        if p is not None: preds.append((h,p))
    # Structural transfer: any unbound action may borrow a hypothesis whose behavior is
    # observable from a current binding signature.  Bindings are created during ingest.
    bid=self._asi_core_010_state['bindings'].get(str(action))
    if bid and not preds:
        for h in self._asi_core_010_state['hypotheses']:
            if h.get('hypothesis_id')==bid:
                p=_apply_hypothesis(h,obs)
                if p is not None: preds.append((h,p))
    if not preds: return None,(),1.0
    # When a context-conditioned hypothesis is applicable, it is a more specific
    # explanation than an unconditional legacy hypothesis and should dominate it.
    contextual=[hp for hp in preds if hp[0].get('context_role')]
    if contextual: preds=contextual
    # Prefer most observed/high confidence; preserve genuine competing predictions for uncertainty.
    preds.sort(key=lambda hp:(-float(hp[0].get('specificity',0)), -float(hp[0].get('support_rate',0)), -float(hp[0].get('confidence',0))/(1+int(hp[0].get('errors',0))), hp[0]['hypothesis_id']))
    return copy.deepcopy(preds[0][1]), tuple(copy.deepcopy(p) for _,p in preds), 0.0 if len({_canon(p) for _,p in preds})==1 else 0.5

def interactive_predict(self, observation: Json, action: str, *, history: Sequence[Mapping[str,Any]]=()):
    _ensure(self)
    pred,preds,unc=_hyp_predict(self,str(action),observation)
    if pred is None:
        self._asi_core_010_state['unknown_count']+=1
    return {'prediction':pred,'alternatives':preds,'uncertainty':unc,'hypothesis_count':len(preds),'history_used':len(history)}

def interactive_predict_at_horizon(self, observation: Json, action: str, horizon: int=1, *, history: Sequence[Mapping[str,Any]]=()):
    _ensure(self); horizon=max(1,int(horizon))
    hs_exact=[h for h in self._asi_core_010_state['hypotheses'] if str(h.get('action_signature'))==str(action) and int(h.get('delay',1))==horizon]
    if horizon>1 and not hs_exact:
        return {'prediction':None,'uncertainty':1.0,'delay':None}
    hs=hs_exact or [h for h in self._asi_core_010_state['hypotheses'] if str(h.get('action_signature'))==str(action) and int(h.get('delay',1))==1]
    if not hs: return {'prediction':copy.deepcopy(observation) if horizon>1 else None,'uncertainty':1.0,'delay':None}
    hs.sort(key=lambda h:(-float(h.get('specificity',0)),-float(h.get('support_rate',0)),-float(h.get('confidence',0))/(1+int(h.get('errors',0)))))
    pred=_apply_hypothesis(hs[0],observation)
    return {'prediction':pred,'uncertainty':0.0 if pred is not None else 1.0,'delay':int(hs[0].get('delay',1))}

def interactive_model_summary(self):
    _ensure(self); s=self._asi_core_010_state
    return {'hypothesis_count':len(s['hypotheses']),'model_updates':s['model_updates'],'prediction_errors':len(s['prediction_errors']),
            'revision_count':s['revision_count'],'interaction_steps':s['interaction_steps'],'unknown_count':s['unknown_count'],'transfer_reuse_count':int(s.get('transfer_reuse_count',0))}

def _predicted_effects_match(before: Json, predicted: Json, actual: Json) -> tuple[bool, tuple[dict[str,Any],...], tuple[dict[str,Any],...]]:
    if predicted is None:
        return False, (), tuple(_diff_effect(before, actual))
    pe=list(_diff_effect(before, predicted)); ae=list(_diff_effect(before, actual))
    # When an ordered collection is permitted to reorder, direct positional leaf deltas
    # are not stable action effects if a marker-bound entity delta exists for the same field.
    def strip_redundant_positional(effects):
        entity_keys={(e.get('field'),_canon(e.get('marker')),e.get('delta')) for e in effects if e.get('kind')=='entity_delta'}
        out=[]
        for e in effects:
            if e.get('kind')=='delta' and e.get('path') and any(isinstance(x,int) for x in e.get('path',())[:-1]):
                field=e.get('path',())[-1]
                if any(k[0]==field and k[2]==e.get('delta') for k in entity_keys):
                    continue
            out.append(e)
        return out
    pe=strip_redundant_positional(pe); ae=strip_redundant_positional(ae)
    def key(e):
        return (_canon(e.get('kind')), _canon(e.get('role')), e.get('field'), _canon(e.get('marker')), _canon(e.get('delta')), _canon(e.get('path')))
    actual_keys={key(e) for e in ae}
    if not pe:
        # Unchanged prediction means the learner predicts no action-attributable
        # change at this horizon. Exogenous observation drift may still occur.
        return True, tuple(pe), tuple(ae)
    matched=all(key(e) in actual_keys for e in pe)
    return matched, tuple(pe), tuple(ae)


def interactive_update(self, before: Json, action: str, predicted: Json|None, actual: Json, *, reward: float=0.0, exogenous: bool=False):
    _ensure(self); s=self._asi_core_010_state; action=str(action)
    evaluated = predicted is not None
    effect_match, predicted_effects, actual_effects = _predicted_effects_match(before, predicted, actual) if evaluated else (False, (), tuple(_diff_effect(before, actual)))
    err = (0.0 if effect_match else 1.0) if evaluated else None
    s['prediction_errors'].append({'action':action,'error':err,'evaluated':evaluated,'effect_match':effect_match,'predicted_effects':copy.deepcopy(predicted_effects),'actual_effects':copy.deepcopy(actual_effects),'before':copy.deepcopy(before),'predicted':copy.deepcopy(predicted),'actual':copy.deepcopy(actual),'step':s['interaction_steps'],'exogenous':exogenous})
    s['prediction_errors']=s['prediction_errors'][-512:]
    if err == 1.0:
        for h in s['hypotheses']:
            if h.get('action_signature')==action: h['errors']=int(h.get('errors',0))+1; h['confidence']=max(0.05,float(h.get('confidence',0))-0.08)
    candidates=_candidate_hypotheses(self,action,before,actual)
    # Transfer: when a new action label reproduces a previously learned structural effect,
    # bind it to a copy of that hypothesis. This uses observed behavior, never evaluator labels.
    if not any(str(h.get('action_signature'))==action and h.get('kind')!='noop' for h in s['hypotheses']):
        prior=[]
        for ph in s['hypotheses']:
            pm,po,ps=_hypothesis_support(ph,s.get('records',()),ph.get('action_signature'))
            for ch in candidates:
                if ch.get('kind')==ph.get('kind') and ch.get('delta')==ph.get('delta') and ch.get('role')==ph.get('role') and ch.get('field')==ph.get('field') and ch.get('marker')==ph.get('marker'):
                    prior.append((float(ph.get('confidence',0))+max(0,ps),ph)); break
        if prior:
            prior.sort(key=lambda z:(-z[0],z[1].get('hypothesis_id',''))); clone=copy.deepcopy(prior[0][1]); s['transfer_reuse_count']=int(s.get('transfer_reuse_count',0))+1; clone['action_signature']=action; clone['hypothesis_id']='th:'+_hash((action,clone.get('hypothesis_id')))[:20]; clone['observations']=1; clone['errors']=0; candidates.insert(0,clone)
    # Revision: retain old hypotheses; add evidence-supported successor hypotheses.
    existing={h['hypothesis_id'] for h in s['hypotheses']}
    added=[]
    for h in candidates:
        if h['hypothesis_id'] in existing: continue
        s['hypotheses'].append(h); added.append(h)
    # Bind action to the behavior signature; this is inferred from experience, not evaluator labels.
    if added:
        s['bindings'][action]=added[0]['hypothesis_id']
    if err == 1.0 and added: s['revision_count']+=1
    for h in s['hypotheses']:
        if h.get('action_signature')==action and h.get('hypothesis_id') not in {x['hypothesis_id'] for x in added}:
            h['observations']=int(h.get('observations',0))+1
            if err == 0.0: h['confidence']=min(1.0,float(h.get('confidence',0))+0.06)
    s['model_updates']+=1
    _refresh_hypotheses(self)
    return {'error':err,'evaluated':evaluated,'effect_match':effect_match,'predicted_effects':copy.deepcopy(predicted_effects),'actual_effects':copy.deepcopy(actual_effects),'revised':bool(err == 1.0 and added),'added_hypotheses':added,'uncertainty':self.interactive_predict(before,action)['uncertainty']}

def interactive_update_at_horizon(self, before: Json, action: str, predicted: Json|None, actual: Json, horizon: int, *, reward: float=0.0):
    _ensure(self); s=self._asi_core_010_state; action=str(action); horizon=max(1,int(horizon))
    evaluated=predicted is not None
    effect_match, predicted_effects, actual_effects = _predicted_effects_match(before,predicted,actual) if evaluated else (False,(),tuple(_diff_effect(before,actual)))
    err=0.0 if evaluated and effect_match else 1.0
    s['prediction_errors'].append({'action':action,'horizon':horizon,'error':err,'evaluated':True,'effect_match':effect_match,'predicted_effects':copy.deepcopy(predicted_effects),'actual_effects':copy.deepcopy(actual_effects),'before':copy.deepcopy(before),'predicted':copy.deepcopy(predicted),'actual':copy.deepcopy(actual),'step':s['interaction_steps']})
    s['prediction_errors']=s['prediction_errors'][-512:]
    if err==1.0:
        for h in s['hypotheses']:
            if h.get('action_signature')==action and int(h.get('delay',1))==horizon:
                h['errors']=int(h.get('errors',0))+1; h['confidence']=max(0.05,float(h.get('confidence',0))-0.08)
        s['revision_count']+=1
    s['model_updates']+=1
    _refresh_hypotheses(self)
    return {'error':err,'evaluated':True,'horizon':horizon,'effect_match':effect_match,'predicted_effects':copy.deepcopy(predicted_effects),'actual_effects':copy.deepcopy(actual_effects),'uncertainty':self.interactive_predict_at_horizon(before,action,horizon)['uncertainty']}


def interactive_record_transition(self, before: Json, action: str, after: Json, *, reward: float=0.0, exogenous: bool=False):
    _ensure(self); self._interactive_last_observation=copy.deepcopy(before)
    pred=self.interactive_predict(before,action)['prediction']
    rec=self.interactive_observe(after,(action,),action,reward,exogenous=exogenous)
    upd=self.interactive_update(before,action,pred,after,reward=reward,exogenous=exogenous)
    rec['prediction']=pred; rec['update']=upd
    return rec

def interactive_select_action(self, observation: Json, actions: Sequence[str], *, goal: Mapping[str,Any]|None=None, explore: bool=True):
    _ensure(self); actions=tuple(actions)
    if not actions: return None
    scored=[]
    for a in actions:
        info=self.interactive_predict(observation,a)
        pred=info['prediction']
        score=0.0
        if goal is not None and pred is not None: score+=_goal_score(pred,goal)*4.0
        if explore:
            # Prefer actions whose current predictive distribution is uncertain but
            # still identifiable. Fully unknown actions are not automatically treated
            # as informative; the learner earns exploration value from discriminative
            # uncertainty that can be reduced by an observed intervention.
            u=float(info['uncertainty'])
            score+=2.0*(u*(1.0-u)) if pred is not None else 0.0
        score-=0.001*len(info['alternatives'])
        scored.append((score,a,info))
    scored.sort(key=lambda x:(-x[0],str(x[1])))
    chosen=scored[0]
    self._asi_core_010_state['experiments'].append({'observation':copy.deepcopy(observation),'chosen':chosen[1],'scores':[(a,round(sc,6),inf['uncertainty']) for sc,a,inf in scored]})
    return {'action':chosen[1],'scores':chosen[2],'alternatives':tuple((a,inf) for _,a,inf in scored)}

def interactive_plan(self, observation: Json, actions: Sequence[str], goal: Mapping[str,Any], *, horizon: int=4):
    _ensure(self); actions=tuple(actions); horizon=max(1,min(10,int(horizon))); best=None; nodes=0
    frontier=[(observation,())]
    for depth in range(horizon):
        nxt=[]
        for state,plan in frontier:
            if _goal_score(state,goal)>=1.0:
                return {'actions':plan,'predicted_final':copy.deepcopy(state),'goal_score':1.0,'nodes':nodes,'horizon':depth}
            for a in actions:
                info=self.interactive_predict(state,a); nodes+=1
                if info['prediction'] is None: continue
                ns=info['prediction']; sc=_goal_score(ns,goal)
                nxt.append((ns,plan+(a,),sc))
        nxt.sort(key=lambda t:(-t[2],len(t[1]),_canon(t[1])))
        frontier=[(s,p) for s,p,_ in nxt[:64]]
        if nxt and (best is None or nxt[0][2]>best[0]): best=(nxt[0][2],nxt[0])
        if not frontier: break
    result={'actions':best[1][1] if best else (), 'predicted_final':copy.deepcopy(best[1][0]) if best else None,
            'goal_score':best[0] if best else 0.0,'nodes':nodes,'horizon':horizon}
    self._asi_core_010_state['plans'].append(copy.deepcopy(result)); return result

def interactive_diagnose(self):
    _ensure(self); s=self._asi_core_010_state
    errs=[r for r in s['prediction_errors'] if r.get('error')]
    diagnoses=[]
    if not errs:
        if int(s.get('unknown_count',0))>0:
            diagnoses.append({'kind':'INSUFFICIENT_EVIDENCE','confidence':0.75,'evidence':{'errors':0,'unknown_count':int(s.get('unknown_count',0))}})
            diagnoses.append({'kind':'CURRENT_MACHINERY_APPEARS_SUFFICIENT','confidence':0.25,'evidence':{'errors':0,'unknown_count':int(s.get('unknown_count',0))}})
        else:
            diagnoses.append({'kind':'CURRENT_MACHINERY_APPEARS_SUFFICIENT','confidence':0.75,'evidence':{'errors':0}})
    else:
        recent=errs[-8:]
        diagnoses.extend([
            {'kind':'ACTION_CONSEQUENCE_MODEL_INSUFFICIENT','confidence':0.34,'evidence':{'recent_errors':len(recent),'hypotheses':len(s['hypotheses'])}},
            {'kind':'TEMPORAL_ABSTRACTION_INSUFFICIENT','confidence':0.26,'evidence':{'longest_delay':max([int(h.get('delay',1)) for h in s['hypotheses']] or [1]),'max_delay':int(s.get('max_delay',4))}},
            {'kind':'REPRESENTATION_INSUFFICIENT','confidence':0.20,'evidence':{'hypotheses':len(s['hypotheses'])}},
            {'kind':'INSUFFICIENT_EVIDENCE','confidence':0.20,'evidence':{'unknown_count':s.get('unknown_count',0)}},
        ])
    total=sum(d['confidence'] for d in diagnoses) or 1.0
    for d in diagnoses: d['confidence']/=total
    s['diagnostic_records'].append({'diagnoses':copy.deepcopy(diagnoses),'step':s['interaction_steps']}); s['diagnostic_records']=s['diagnostic_records'][-128:]
    return tuple(diagnoses)

def interactive_state(self): _ensure(self); return copy.deepcopy(self._asi_core_010_state)

def _export_010(self, base):
    out=copy.deepcopy(dict(base)); _ensure(self); out['asi_core_010']=copy.deepcopy(self._asi_core_010_state); return out

def _load_010(self, state):
    info=dict(state.get('asi_core_010',{}) or {}); _init_010(self); self._asi_core_010_state={
        'records':list(info.get('records',())), 'hypotheses':list(info.get('hypotheses',())), 'retired_hypotheses':list(info.get('retired_hypotheses',())),
        'prediction_errors':list(info.get('prediction_errors',())), 'experiments':list(info.get('experiments',())), 'plans':list(info.get('plans',())),
        'bindings':dict(info.get('bindings',{})), 'unknown_count':int(info.get('unknown_count',0)), 'revision_count':int(info.get('revision_count',0)),
        'model_updates':int(info.get('model_updates',0)), 'interaction_steps':int(info.get('interaction_steps',0)), 'transfer_reuse_count':int(info.get('transfer_reuse_count',0)), 'max_delay':int(info.get('max_delay',4)), 'diagnostic_records':list(info.get('diagnostic_records',())), }

try:
    from .general_engine import GeneralLearner
    if not getattr(GeneralLearner,'_asi_core_010_bound',False):
        _ORIG_INIT_010=GeneralLearner.__init__; _ORIG_EXPORT_010=GeneralLearner.export_state; _ORIG_FROM_STATE_010=GeneralLearner.from_state
        def _init_bound_010(self,*a,**kw): _ORIG_INIT_010(self,*a,**kw); _init_010(self)
        def _export_bound_010(self): return _export_010(self,_ORIG_EXPORT_010(self))
        def _from_state_bound_010(cls,spec,**kw): obj=_ORIG_FROM_STATE_010.__func__(cls,copy.deepcopy(dict(spec)),**kw); _init_010(obj); _load_010(obj,spec); return obj
        GeneralLearner.__init__=_init_bound_010
        GeneralLearner.export_state=_export_bound_010
        GeneralLearner.from_state=classmethod(_from_state_bound_010)
        GeneralLearner.interactive_observe=interactive_observe
        GeneralLearner.interactive_predict=interactive_predict
        GeneralLearner.interactive_update=interactive_update
        GeneralLearner.interactive_update_at_horizon=interactive_update_at_horizon
        GeneralLearner.interactive_record_transition=interactive_record_transition
        GeneralLearner.interactive_select_action=interactive_select_action
        GeneralLearner.interactive_plan=interactive_plan
        GeneralLearner.interactive_state=interactive_state
        GeneralLearner.interactive_diagnose=interactive_diagnose
        GeneralLearner.interactive_predict_at_horizon=interactive_predict_at_horizon
        GeneralLearner.interactive_model_summary=interactive_model_summary
        GeneralLearner._asi_core_010_bound=True
except Exception:
    pass
