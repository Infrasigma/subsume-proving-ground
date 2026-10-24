"""ASI-CORE-011: generic lifelong memory, context discovery and interference control.

All cognition is attached to the single GeneralLearner. The implementation is
incremental and bounded so continual operation does not repeatedly run the full
CORE-010 hypothesis search over the entire history.
"""
from __future__ import annotations
import copy, json, math, collections
from hashlib import sha256
from typing import Any, Mapping, Sequence

from .general_engine import GeneralLearner
from .interactive_learning import _canon, _ordinal_map, _structural_slot, _diff_effect, _apply_hypothesis, _replace

Json=Any

def _shape(x:Json):
    if isinstance(x,dict): return ('dict',tuple(sorted((str(k),_shape(v)) for k,v in x.items())))
    if isinstance(x,list): return ('list',len(x),tuple(_shape(v) for v in x[:4]))
    if isinstance(x,tuple): return ('tuple',len(x),tuple(_shape(v) for v in x[:4]))
    if isinstance(x,bool): return 'bool'
    if isinstance(x,int) and not isinstance(x,bool): return 'int'
    if isinstance(x,float): return 'float'
    if isinstance(x,str): return 'str'
    if x is None: return 'null'
    return type(x).__name__

def _hash(x): return sha256(json.dumps(x,sort_keys=True,separators=(',',':'),default=str).encode()).hexdigest()

def _effect_signature(before,after):
    out=[]
    for e in _diff_effect(before,after):
        k=e.get('kind')
        if k=='entity_delta': out.append((k,str(e.get('field')),e.get('delta')))
        elif k in {'delta','toggle','set'}: out.append((k,_canon(e.get('role')),e.get('delta'),_canon(e.get('after'))))
        elif k in {'reverse_nested','rotate1_nested','permute_nested'}: out.append((k,_canon(e.get('path')),e.get('length')))
        elif k in {'reverse','rotate1'}: out.append((k,))
    return tuple(sorted(out,key=_canon))

def _effect_objects(before,after):
    if isinstance(before,dict) and '__cached_effects__' in before: return before['__cached_effects__']
    return [e for e in _diff_effect(before,after) if e.get('kind') in {'delta','toggle','set','reverse','rotate1','reverse_nested','rotate1_nested','permute_nested','entity_delta'}]

def _feature_values(obs):
    vals=[]
    for _,p,v,k in _ordinal_map(obs):
        # Only small discrete/categorical features are eligible as learned context cues.
        if isinstance(v,(str,bool)) or (isinstance(v,int) and not isinstance(v,bool) and abs(v)<=8):
            vals.append((tuple(_structural_slot(obs,p,v)),copy.deepcopy(v)))
    return vals

def _behavioral_role(obs,path,value):
    # Surface-independent role learned from observable structural statistics: ordinal numeric/categorical leaf.
    leaves=_ordinal_map(obs); idx=next((i for i,p,_,_ in leaves if tuple(p)==tuple(path)),-1)
    if isinstance(value,(int,float)) and not isinstance(value,bool): return ('number_leaf',idx)
    return ('leaf',idx,_shape(value))

def _context_features(obs):
    vals=[]
    for i,p,v,k in _ordinal_map(obs):
        if isinstance(v,(str,bool)) or (isinstance(v,int) and not isinstance(v,bool) and abs(v)<=8):
            vals.append((('struct',)+tuple(_structural_slot(obs,p,v)),copy.deepcopy(v)))
            vals.append((('beh',)+tuple(_behavioral_role(obs,p,v)),copy.deepcopy(v)))
    return vals

def _abstract_effect_signature(before,after):
    out=[]
    for e in _diff_effect(before,after):
        k=e.get('kind')
        if k in {'delta','toggle','set'}:
            role=_behavioral_role(before,e.get('path',()),e.get('before'))
            out.append((k,role,e.get('delta'),_canon(e.get('after'))))
        elif k=='entity_delta':
            out.append((k,str(e.get('field')),e.get('delta')))
        elif k in {'reverse','rotate1','reverse_nested','rotate1_nested','permute_nested'}:
            out.append((k,e.get('length'),_canon(e.get('path'))))
    return tuple(sorted(out,key=_canon))

def _model_from_effect(action,e,before,*,context_role=None,context_value=None,delay=1,source=None):
    return {
        'model_id':'m:'+_hash((action,e.get('kind'),e.get('role'),e.get('field'),e.get('delta'),e.get('marker'),e.get('path'),e.get('after'),context_role,context_value,delay))[:24],
        'action_signature':str(action),'kind':e.get('kind'),'role':list(e.get('role') or ()), 'behavioral_role':list(_behavioral_role(before,e.get('path',()),e.get('before'))),
        'delta':e.get('delta'),'field':e.get('field'),'marker':copy.deepcopy(e.get('marker')),'entity':e.get('entity',0),
        'after_value':copy.deepcopy(e.get('after')),'after':copy.deepcopy(e.get('after')),'path':list(e.get('path',())),
        'length':e.get('length'),'delay':int(delay),'context_role':list(context_role) if context_role else None,
        'context_value':copy.deepcopy(context_value) if context_role else None,'support':1,'successes':0,'failures':0,
        'uses':0,'confidence':0.5,'surface_shape':_shape(before),'abstract':_canon((e.get('kind'),e.get('role'),e.get('field'),e.get('delta'),e.get('path'))),
        'source_model':source,
    }

def _matches(model,obs):
    learned_shape=model.get('surface_shape')
    surface_match = learned_shape is None or _canon(_shape(obs)) == _canon(learned_shape)
    role=tuple(model.get('context_role') or ())
    if role:
        want=_canon(model.get('context_value')); found=False
        for r,v in _context_features(obs):
            rr=tuple(r[1:]) if isinstance(r,tuple) and r and r[0]=='struct' else tuple(r)
            if rr==role:
                found=True
                if _canon(v)!=want:return None
        if not found:return None
    # Preserve exact structural execution for known surfaces.
    if surface_match:
        direct=_apply_hypothesis(model,obs)
        if direct is not None: return direct
    # Surface-independent numeric behavior is a fallback for genuinely new surfaces.
    if model.get('kind')=='delta' and model.get('behavioral_role') and model.get('abstraction_available',True):
        br=tuple(model.get('behavioral_role')); idx=int(br[1]) if len(br)>=2 and br[0]=='number_leaf' else -1
        nums=[(p,v) for _,p,v,k in _ordinal_map(obs) if isinstance(v,(int,float)) and not isinstance(v,bool)]
        if 0<=idx<len(nums):
            p,v=nums[idx]
            try:return _replace(obs,p,v+model.get('delta',0))
            except Exception:return None
    return None


def _context_effect_key(e):
    return (_canon(e.get('kind')),_canon(e.get('role')),_canon(e.get('field')),_canon(e.get('marker')),_canon(e.get('delta')),_canon(e.get('path')))

def _context_effects(rows, key=None):
    counts={}
    for r in rows:
        seen=set()
        for e in r.get('effects',_effect_objects(r['before'],r['after'])):
            k=_context_effect_key(e)
            if k in seen: continue
            seen.add(k); counts[k]=counts.get(k,0)+1
    return counts

def _context_discoverable_rows(surface_rows):
    """Return action rows annotated with effects that are repeatable for that action and absent enough elsewhere."""
    by_action={}
    for r in surface_rows:
        by_action.setdefault(str(r['action']),[]).append(r)
    out=[]
    for action,rows in by_action.items():
        if len(rows)<4: continue
        other=[r for a,rs in by_action.items() if a!=action for r in rs]
        counts={}; other_counts={}
        for r in rows:
            for e in r.get('effects',_effect_objects(r['before'],r['after'])):
                counts[_context_effect_key(e)]=counts.get(_context_effect_key(e),0)+1
        for r in other:
            seen=set()
            for e in r.get('effects',_effect_objects(r['before'],r['after'])):
                k=_context_effect_key(e)
                if k not in seen: other_counts[k]=other_counts.get(k,0)+1
                seen.add(k)
        for r in rows:
            sig=[]
            for e in r.get('effects',_effect_objects(r['before'],r['after'])):
                k=_context_effect_key(e); mr=counts.get(k,0)/max(1,len(rows)); orr=other_counts.get(k,0)/max(1,len(other))
                if mr>=0.55 and mr-orr>=0.25 and orr<=0.50:
                    sig.append((e.get('kind'),_canon(e.get('role')),e.get('field'),e.get('delta'),e.get('path')))
            out.append((action,r,tuple(sorted(sig,key=_canon))))
    return out

def _context_max_run(surface_rows, role, value):
    max_run=run=0
    for r in sorted(surface_rows,key=lambda x:int(x.get('step',0))):
        target=tuple(role) if tuple(role) and tuple(role)[0]=='struct' else ('struct',)+tuple(role)
        vals=[v for rr,v in _context_features(r['before']) if tuple(rr)==target]
        hit=bool(vals) and _canon(vals[0])==_canon(value)
        if hit: run+=1; max_run=max(max_run,run)
        else: run=0
    return max_run

def _apply_context_models(self):
    s=self._asi_core_011_state
    recs=[r for r in s['episodic'] if not r.get('exogenous')]
    surfaces={}
    for r in recs: surfaces.setdefault(_canon(_shape(r['before'])),[]).append(r)
    for surface_rows in surfaces.values():
        if len(surface_rows)<12: continue
        actions=sorted({str(r['action']) for r in surface_rows})
        for action in actions:
            rows=[r for r in surface_rows if str(r['action'])==action]
            other=[r for r in surface_rows if str(r['action'])!=action]
            if len(rows)<6 or len(other)<2: continue
            groups={}
            role_values={}
            for r in surface_rows:
                for role,val in _context_features(r['before']):
                    role_values.setdefault(role,set()).add(_canon(val))
            for r in rows:
                for role,val in _context_features(r['before']):
                    # Prefer compact observable context variables. This removes high-cardinality
                    # state values while preserving genuinely persistent cues such as a mode/probe.
                    if len(role_values.get(role,set()))>3: continue
                    groups.setdefault(role,{}).setdefault(_canon(val),[]).append(r)
            stable=[]
            for role,gdict in groups.items():
                if not (isinstance(role,tuple) and role and role[0]=='struct'): continue
                for val,context_rows in gdict.items():
                    if len(context_rows)<3 or _context_max_run(surface_rows,role,json.loads(val))<3: continue
                    cc=_context_effects(context_rows,None); oc=_context_effects(other,None)
                    candidates=[]
                    for e in context_rows[0].get('effects',_effect_objects(context_rows[0]['before'],context_rows[0]['after'])):
                        k=_context_effect_key(e); mr=cc.get(k,0)/len(context_rows); orr=oc.get(k,0)/max(1,len(other))
                        if mr>=0.75 and mr-orr>=0.25 and orr<=0.50: candidates.append(e)
                    filtered=[]
                    for e in candidates:
                        path=tuple(e.get('path') or ())
                        if path and e.get('kind') in {'delta','toggle','set'}:
                            action_classes=sorted({str(rr['action']) for rr in surface_rows})
                            broad=[]
                            for aa in action_classes:
                                ars=[rr for rr in surface_rows if str(rr['action'])==aa]
                                hits=sum(any(tuple(x.get('path') or ())==path for x in rr.get('effects',_effect_objects(rr['before'],rr['after']))) for rr in ars)
                                if ars and hits/len(ars)>=0.60: broad.append(aa)
                            if len(action_classes)>=3 and len(broad)>=2 and len(broad)/len(action_classes)>=0.67:
                                continue
                        filtered.append(e)
                    if filtered:
                        sig=tuple(sorted([(e.get('kind'),e.get('role'),e.get('field'),e.get('delta'),e.get('path')) for e in filtered], key=_canon))
                        stable.append((role,val,context_rows,sig))
            for i,(role,val,context_rows,sig) in enumerate(stable):
                if not any(z[0]==role and _canon(z[3])!=_canon(sig) for j,z in enumerate(stable) if j!=i): continue
                r=None
                for rr in context_rows:
                    rr_sig=tuple(sorted([(e.get('kind'),_canon(e.get('role')),e.get('field'),e.get('delta'),e.get('path')) for e in rr.get('effects',_effect_objects(rr['before'],rr['after'])) if _context_effect_key(e) in _context_effects(context_rows,None)], key=_canon))
                    if _canon(rr_sig)==_canon(sig): r=rr; break
                if r is None: r=context_rows[0]
                sig_targets={(_canon(a),_canon(ro),f,d,_canon(pa)) for a,ro,f,d,pa in sig}
                for e in r.get('effects',_effect_objects(r['before'],r['after'])):
                    k=(_canon(e.get('kind')),_canon(e.get('role')),e.get('field'),e.get('delta'),_canon(e.get('path')))
                    if k not in sig_targets: continue
                    if str(e.get('kind')) not in {'delta','entity_delta','toggle','set','reverse','rotate1','reverse_nested','rotate1_nested','permute_nested'}: continue
                    m=_model_from_effect(action,e,r['before'],context_role=tuple(role[1:]),context_value=json.loads(val),source='context-discovery')
                    m['surface_shape']=_shape(r['before']); m['causal_specificity']=1.0
                    self._asi_core_011_upsert(m,_key(m))


def _causal_relevance(self,action,e,rows_override=None):
    s=self._asi_core_011_state; recs=rows_override if rows_override is not None else s['episodic']
    def k(x): return (_canon(x.get('kind')),_canon(x.get('role')),_canon(x.get('field')),_canon(x.get('marker')),_canon(x.get('delta')),_canon(x.get('path')))
    target=k(e); local=[r for r in recs if not r.get('exogenous') and _canon(_shape(r['before']))==_canon(_shape(e.get('_before_shape',{}))) ] if False else []
    base_rows=[r for r in recs if not r.get('exogenous')]
    # Prefer evidence from the same observable surface as the candidate transition. This avoids
    # diluting a real local causal contrast when the lifelong stream contains many unrelated worlds.
    before_shape = getattr(self,'_asi_core_011_causal_before_shape',None)
    if before_shape is not None:
        local=[r for r in base_rows if _canon(_shape(r['before']))==_canon(before_shape)]
    else: local=base_rows
    mine=[r for r in local if str(r['action'])==str(action)]; other=[r for r in local if str(r['action'])!=str(action)]
    if len(mine)<3 or len(other)<3:
        mine=[r for r in base_rows if str(r['action'])==str(action)]; other=[r for r in base_rows if str(r['action'])!=str(action)]
    if len(mine)<3 or len(other)<3: return 0.0,0.0,0.0
    def hit(rows): return sum(any(k(x)==target for x in r.get('effects', _effect_objects(r['before'],r['after']))) for r in rows)/max(1,len(rows))
    mr,orr=hit(mine),hit(other); return mr,orr,mr-orr

def _validate_models(self):
    # Retention is the default; conflict is represented by competing models and resolved
    # from realized consequences rather than erased by recency.
    return None


def _upsert_effect_models(self,before,action,after):
    s=self._asi_core_011_state; effs=_effect_objects(before,after)
    old_shape=getattr(self,'_asi_core_011_causal_before_shape',None); self._asi_core_011_causal_before_shape=_shape(before)
    try:
        pairs=[(_causal_relevance(self,action,e),e) for e in effs]
    finally:
        self._asi_core_011_causal_before_shape=old_shape
    for (mr,orr,spec),e in pairs:
        if len(self._asi_core_011_state['episodic'])<8 or mr<0.40 or spec<0.40 or orr>0.25: continue
        # A path that changes under most action classes is better explained as
        # an exogenous/clock-like observable drift than as an action consequence.
        path=tuple(e.get('path') or ())
        if path and e.get('kind') in {'delta','toggle','set'}:
            same_surface=[r for r in s['episodic'] if not r.get('exogenous') and _canon(_shape(r['before']))==_canon(_shape(before))]
            action_classes=sorted({str(r['action']) for r in same_surface})
            changed_by=[]
            for aa in action_classes:
                ars=[r for r in same_surface if str(r['action'])==aa]
                hits=sum(any(tuple(x.get('path') or ())==path for x in r.get('effects',_effect_objects(r['before'],r['after']))) for r in ars)
                if ars and hits/len(ars)>=0.60: changed_by.append(aa)
            if len(changed_by)>=2 and len(action_classes)>=3 and len(changed_by)/len(action_classes)>=0.67:
                continue
        m=_model_from_effect(action,e,before); m['causal_specificity']=spec; key=_key(m); _upsert(self,m,key)
    # When a new action label realizes the same structural consequence as retained knowledge,
    # preserve an explicit reusable abstraction and mark the source rather than guessing beforehand.
    for m in list(s['models']):
        if m.get('action_signature')==str(action): continue
        for e in effs:
            if m.get('abstract')==_canon((e.get('kind'),e.get('role'),e.get('field'),e.get('delta'),e.get('path'))):
                s['transfer_hits']+=1; break

def _learn_delay_models(self):
    """Learn delayed effects with within-surface causal lag contrasts.

    Immediate changes that occur under every action (for example an observable
    clock) are treated as exogenous to the action. A source transition is
    considered immediate-action-noop only when its effects have no positive
    action-specific contrast.
    """
    s=self._asi_core_011_state
    maxd=int(s.get('delay_budget',4))
    rows=list(s['episodic'])[-min(80,int(s.get('episodic_budget',80))):]
    if len(rows)<8: return
    by_step={int(r['step']):r for r in rows}
    surfaces={}
    for r in rows: surfaces.setdefault(_canon(_shape(r['before'])),[]).append(r)
    for surface_rows in surfaces.values():
        surface_steps={int(r['step']) for r in surface_rows}
        actions=sorted({str(r['action']) for r in surface_rows})
        if len(actions)<2: continue
        by_action={a:[r for r in surface_rows if str(r['action'])==a] for a in actions}
        # Identify immediate effects that discriminate an action from alternatives.
        causal_keys={}
        for action,ars in by_action.items():
            other=[r for a,rs in by_action.items() if a!=action for r in rs]
            if len(ars)<3 or len(other)<3: continue
            rates={}; ctr={}
            for r in ars:
                for e in r.get('effects',_effect_objects(r['before'],r['after'])):
                    k=_context_effect_key(e); rates[k]=rates.get(k,0)+1
            for r in other:
                seen=set()
                for e in r.get('effects',_effect_objects(r['before'],r['after'])):
                    k=_context_effect_key(e)
                    if k not in seen: ctr[k]=ctr.get(k,0)+1
                    seen.add(k)
            causal_keys[action]={k for k,n in rates.items() if n/len(ars)>=0.60 and n/len(ars)-ctr.get(k,0)/len(other)>=0.30 and ctr.get(k,0)/len(other)<=0.40}
        for action in actions:
            source_pool=by_action.get(action,[])
            sources=[r for r in source_pool if not ({_context_effect_key(e) for e in r.get('effects',_effect_objects(r['before'],r['after']))} & causal_keys.get(action,set()))]
            controls=[r for a,rs in by_action.items() if a!=action for r in rs if not ({_context_effect_key(e) for e in r.get('effects',_effect_objects(r['before'],r['after']))} & causal_keys.get(a,set()))]
            if len(sources)<3 or len(controls)<3: continue
            for d in range(2,maxd+1):
                pairs=[]; control_targets=[]
                for r in sources:
                    st=int(r['step']); needed=[st+i for i in range(d)]
                    if not all(x in by_step and x in surface_steps for x in needed): continue
                    target=by_step.get(st+d-1)
                    if target is not None: pairs.append((r,target))
                for r in controls:
                    st=int(r['step']); needed=[st+i for i in range(d)]
                    if not all(x in by_step and x in surface_steps for x in needed): continue
                    target=by_step.get(st+d-1)
                    if target is not None: control_targets.append(target)
                if len(pairs)<3 or len(control_targets)<3: continue
                effect_keys=set()
                for _,target in pairs:
                    effect_keys.update(_context_effect_key(e) for e in target.get('effects',_effect_objects(target['before'],target['after'])))
                for ek in effect_keys:
                    hs=sum(any(_context_effect_key(e)==ek for e in target.get('effects',_effect_objects(target['before'],target['after']))) for _,target in pairs)
                    hc=sum(any(_context_effect_key(e)==ek for e in target.get('effects',_effect_objects(target['before'],target['after']))) for target in control_targets)
                    support=hs/len(pairs); control=hc/len(control_targets); specificity=support-control
                    if support<0.75 or specificity<0.50 or control>0.25: continue
                    chosen=None
                    for src,target in reversed(pairs):
                        e=next((e for e in target.get('effects',_effect_objects(target['before'],target['after'])) if _context_effect_key(e)==ek),None)
                        if e is not None: chosen=(src,e); break
                    if chosen is None: continue
                    src,e=chosen
                    m=_model_from_effect(action,e,src['before'],delay=d,source='lag-contrast')
                    m['causal_specificity']=specificity; m['temporal_support']=support; m['temporal_control_rate']=control
                    self._asi_core_011_upsert(m,_key(m))


def _recompute_abstractions(self):
    s=self._asi_core_011_state; amap={}
    for m in s['models']:
        key=(m.get('kind'),m.get('field'),m.get('delta'),_canon(m.get('role')),m.get('delay'))
        aid='a:'+_hash(key)[:20]
        a=amap.get(key)
        if a is None:
            a={'abstraction_id':aid,'key':key,'actions':[m.get('action_signature')],'uses':0,'successes':0}
            amap[key]=a
        elif m.get('action_signature') not in a['actions']:
            a['actions'].append(m.get('action_signature'))
        m['abstraction_id']=aid
    s['abstractions']=list(amap.values())[:int(s['abstraction_budget'])]


def _init_011(self):
    if hasattr(self,'_asi_core_011_state'): return
    self._asi_core_011_state={'schema':'asi-core-011.v34','episodic':[],'models':[],'abstractions':[],
        'episodic_budget':80,'model_budget':96,'abstraction_budget':64,'delay_budget':4,'step':0,
        'active_by_action':{},'consolidations':0,'discarded_models':0,'retrievals':0,'unknown_retrievals':0,
        'surprise':0,'ambiguous':0,'active_disambiguations':0,'retrieval_cost':0,'transfer_hits':0,
        'context_discoveries':0,'recovery_switches':0}

def _ensure(self): _init_011(self)

def _key(m): return (str(m.get('action_signature')),str(m.get('kind')),_canon(m.get('role')),str(m.get('field')),_canon(m.get('delta')),_canon(m.get('context_role')),_canon(m.get('context_value')),int(m.get('delay',1)))

def _upsert(self,m,key):
    s=self._asi_core_011_state; existing={_key(x):x for x in s['models']}; old=existing.get(key)
    if old is None:
        s['models'].append(m); s['consolidations']+=1
    else: old['support']=int(old.get('support',1))+1
    # compact, learned-value memory policy: preferentially preserve models with successful use and contextual specificity.
    if len(s['models'])>int(s['model_budget']):
        ranked=sorted(s['models'],key=lambda x:(-(int(x.get('successes',0))*3+int(x.get('support',1))+int(bool(x.get('context_role')))),int(x.get('failures',0)),str(x.get('model_id'))))
        s['models']=ranked[:int(s['model_budget'])]; s['discarded_models']+=max(0,len(ranked)-int(s['model_budget']))
    _recompute_abstractions(self)

GeneralLearner._asi_core_011_model_key=_key
GeneralLearner._asi_core_011_upsert=_upsert

# Capture CORE-010 methods before installing the lifelong layer.
_ORIG_INIT=GeneralLearner.__init__
_ORIG_EXPORT=GeneralLearner.export_state
_ORIG_FROM_STATE=GeneralLearner.from_state


def _predict(self,observation,action):
    _ensure(self); s=self._asi_core_011_state; action=str(action)
    rows=[]
    for m in s['models']:
        if str(m.get('action_signature'))!=action or int(m.get('delay',1))!=1: continue
        mm=m
        if _canon(_shape(observation))!=_canon(m.get('surface_shape')):
            aid=m.get('abstraction_id'); mm=dict(m); mm['abstraction_available']=bool(aid and any(a.get('abstraction_id')==aid for a in s.get('abstractions',[])))
        p=_matches(mm,observation)
        if p is not None: rows.append((float(m.get('confidence',0.5))+0.1*float(m.get('successes',0)),m,p))
    rows.sort(key=lambda z:(-int(bool(z[1].get('context_role'))),-z[0],str(z[1].get('model_id'))))
    active=s['active_by_action'].get(action)
    contextual=any(bool(m.get('context_role')) for _,m,_ in rows)
    if active and not contextual:
        for i,(sc,m,p) in enumerate(rows):
            if m.get('model_id')==active:
                rows.insert(0,rows.pop(i)); break
    if rows:
        # Without an established active variant, conflicting top models are honestly ambiguous.
        if not contextual and not active and len(rows)>1 and _canon(rows[0][2])!=_canon(rows[1][2]):
            s['ambiguous']+=1; return {'prediction':None,'uncertainty':0.5,'alternatives':tuple(copy.deepcopy(x[2]) for x in rows[:6]),'source':'consolidated'}
        m,p=rows[0][1],rows[0][2]; m['uses']=int(m.get('uses',0))+1; s['retrievals']+=1
        return {'prediction':copy.deepcopy(p),'uncertainty':0.0,'alternatives':tuple(copy.deepcopy(x[2]) for x in rows[:6]),'source':'consolidated','model_id':m.get('model_id')}
    s['unknown_retrievals']+=1
    return {'prediction':None,'uncertainty':1.0,'alternatives':(),'source':'unknown'}


def _predict_h(self,observation,action,horizon=1):
    _ensure(self); horizon=max(1,int(horizon)); s=self._asi_core_011_state
    rows=[]
    for m in s['models']:
        if str(m.get('action_signature'))!=str(action) or int(m.get('delay',1))!=horizon: continue
        mm=m
        if _canon(_shape(observation))!=_canon(m.get('surface_shape')):
            aid=m.get('abstraction_id'); mm=dict(m); mm['abstraction_available']=bool(aid and any(a.get('abstraction_id')==aid for a in s.get('abstractions',[])))
        p=_matches(mm,observation)
        if p is not None: rows.append((float(m.get('confidence',0.5))+0.1*float(m.get('successes',0)),m,p))
    rows.sort(key=lambda z:-z[0])
    if not rows:return {'prediction':None,'uncertainty':1.0,'delay':horizon,'source':'unknown'}
    if len(rows)>1 and _canon(rows[0][2])!=_canon(rows[1][2]) and not s['active_by_action'].get(str(action)):
        s['ambiguous']+=1; return {'prediction':None,'uncertainty':0.5,'delay':horizon,'source':'consolidated'}
    rows[0][1]['uses']=int(rows[0][1].get('uses',0))+1
    return {'prediction':copy.deepcopy(rows[0][2]),'uncertainty':0.0,'delay':horizon,'source':'consolidated'}


def _feedback(self,before,action,predicted,actual):
    from .interactive_learning import _predicted_effects_match
    _ensure(self); s=self._asi_core_011_state; action=str(action); active=s['active_by_action'].get(action)
    def agrees(p):
        if p is None: return False
        ok,_,_= _predicted_effects_match(before,p,actual)
        return bool(ok)
    current_ok=agrees(predicted)
    if predicted is not None and not current_ok: s['surprise']+=1
    if active:
        for m in s['models']:
            if m.get('model_id')==active: m['successes']=int(m.get('successes',0))+int(current_ok); m['failures']=int(m.get('failures',0))+int(not current_ok)
    candidates=[]
    for m in s['models']:
        if str(m.get('action_signature'))!=action or int(m.get('delay',1))!=1: continue
        p=_matches(m,before)
        if p is not None and agrees(p): candidates.append((int(bool(m.get('context_role'))),int(m.get('successes',0)),m,p))
    if candidates:
        candidates.sort(key=lambda z:(-z[0],-z[1],str(z[2].get('model_id'))))
        _,_,m,_=candidates[0]
        # A context-specific model is selected locally; global active routing is only valid when no context cue exists.
        if active and active!=m.get('model_id'): s['recovery_switches']+=1
        if not m.get('context_role'):
            s['active_by_action'][action]=m.get('model_id')
        m['successes']=int(m.get('successes',0))+1
    return current_ok


def lifelong_predict(self,observation,action,*,history=()): return _predict(self,observation,action)
def lifelong_predict_at_horizon(self,observation,action,horizon=1): return _predict_h(self,observation,action,horizon)

def lifelong_record_transition(self,before,action,after,*,reward=0.0,exogenous=False):
    _ensure(self); s=self._asi_core_011_state; action=str(action); pred=_predict(self,before,action)['prediction']
    effects=_effect_objects(before,after); sig=_effect_signature(before,after); s['episodic'].append({'before':copy.deepcopy(before),'after':copy.deepcopy(after),'action':action,'reward':float(reward),'exogenous':bool(exogenous),'step':s['step'],'effects':copy.deepcopy(effects),'effect_signature':sig})
    s['step']+=1
    if len(s['episodic'])>int(s['episodic_budget']): s['episodic']=s['episodic'][-int(s['episodic_budget']):]
    _feedback(self,before,action,pred,after)
    before_models=len(s['models']); _upsert_effect_models(self,before,action,after)
    if s['step']%4==0: _learn_delay_models(self)
    if s['step']%6==0:
        _validate_models(self)
        old=len(s['models']); _apply_context_models(self); s['context_discoveries']+=max(0,len(s['models'])-old)
    _recompute_abstractions(self)
    return {'prediction':pred,'effect_signature':_effect_signature(before,after),'model_count':len(s['models']),'episodic_count':len(s['episodic'])}


def lifelong_update_feedback(self,before,action,predicted,actual,**kwargs):
    return {'error':0.0 if _feedback(self,before,action,predicted,actual) else 1.0}

def lifelong_select_action(self,observation,actions,*,goal=None,explore=True):
    _ensure(self); scored=[]
    for a in tuple(actions):
        info=_predict(self,observation,a)
        effect_alts=[]
        for pred in info.get('alternatives',()):
            try:
                sig=_effect_signature(observation,pred)
                if sig: effect_alts.append(_canon(sig))
            except Exception:
                continue
        distinct=len(set(effect_alts))
        unknown=info['prediction'] is None
        # Information value is disagreement over action consequences, not raw sensor noise.
        score=(4.0*distinct)+(0.5 if unknown and not effect_alts else 0.0)+(0.1*len(effect_alts))
        scored.append((score,str(a),info,distinct))
    scored.sort(key=lambda x:(-x[0],x[1]))
    if not scored:return None
    chosen=scored[0]; self._asi_core_011_state['active_disambiguations']+=int(chosen[3]>1)
    distinct=int(chosen[3]); bits=(math.log2(distinct) if distinct>0 else 0.0); return {'action':chosen[1],'scores':chosen[2],'information_gain':distinct,'information_gain_bits':bits,'candidates':tuple((a,i) for _,a,i,_ in scored)}

def lifelong_state(self): _ensure(self); return copy.deepcopy(self._asi_core_011_state)
def lifelong_diagnose(self):
    _ensure(self); s=self._asi_core_011_state
    if s['ambiguous']>0:return {'kind':'CONTEXT_AMBIGUITY','evidence':{'ambiguous':s['ambiguous'],'models':len(s['models'])}}
    if s['surprise']>0:return {'kind':'INTERFERENCE_OR_MODEL_CONFLICT','evidence':{'surprise':s['surprise'],'switches':s['recovery_switches']}}
    return {'kind':'CURRENT_CONTINUAL_MACHINERY_APPEARS_SUFFICIENT','evidence':{'models':len(s['models'])}}

def _export(self):
    out=copy.deepcopy(dict(_ORIG_EXPORT(self))); _ensure(self); out['asi_core_011']=copy.deepcopy(self._asi_core_011_state); return out

def _from_state(cls,spec,**kwargs):
    obj=_ORIG_FROM_STATE.__func__(cls,copy.deepcopy(dict(spec)),**kwargs); _ensure(obj); info=dict(spec.get('asi_core_011',{}) or {})
    for k,v in info.items(): obj._asi_core_011_state[k]=copy.deepcopy(v)
    return obj

def _init(self,*a,**kw): _ORIG_INIT(self,*a,**kw); _init_011(self)

GeneralLearner.__init__=_init; GeneralLearner.export_state=_export; GeneralLearner.from_state=classmethod(_from_state)
GeneralLearner.lifelong_predict=lifelong_predict; GeneralLearner.lifelong_predict_at_horizon=lifelong_predict_at_horizon
GeneralLearner.lifelong_record_transition=lifelong_record_transition; GeneralLearner.lifelong_update_feedback=lifelong_update_feedback
GeneralLearner.lifelong_select_action=lifelong_select_action; GeneralLearner.lifelong_state=lifelong_state; GeneralLearner.lifelong_diagnose=lifelong_diagnose
GeneralLearner._asi_core_011_bound=True
