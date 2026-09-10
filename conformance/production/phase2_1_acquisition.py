#!/usr/bin/env python3
"""Evidence-driven relational acquisition layer.

The layer generates candidate schemas from observed ground facts, canonicalizes them,
checks cross-task support, and freezes only admissible reusable rules. No task answer,
fixture candidate, seed, or hidden state is an input.
"""
import itertools, hashlib, json
from collections import defaultdict
ROLES=("target","intervention","contrast","context")
CONSEQUENCES=("ENABLES(target)","NONENABLES(contrast,target)")
PREDICATES={"ACTION":1,"AVAILABLE":1,"BLOCKED":1,"INTERVENES":2,"OBSERVED_EFFECT":2,"ENABLES":2,"NONENABLES":2,"SAME_LOCAL_CONTEXT":2}

def canon(x): return json.dumps(x,sort_keys=True,separators=(",",":"),ensure_ascii=False).encode()
def digest(x): return hashlib.sha256(canon(x)).hexdigest()

def generalize(fact):
    p,args,_=fact
    m={}; nxt=0; out=[]
    for x in args:
        if x not in m:
            if nxt>=len(ROLES): return None
            m[x]=ROLES[nxt]; nxt+=1
        out.append(m[x])
    return {"predicate":p,"args":out}

def grounded(pattern, fact):
    p,a=pattern["predicate"],pattern["args"]
    if fact[0]!=p or len(a)!=len(fact[1]): return None
    b={}
    for role,val in zip(a,fact[1]):
        if role in b and b[role]!=val:return None
        b[role]=val
    return b

def apply_pattern(patterns, fs):
    bindings=[{}]
    for p in patterns:
        nxt=[]
        for f in fs:
            b=grounded(p,f)
            if b is None: continue
            for old in bindings:
                merged=dict(old); ok=True
                for k,v in b.items():
                    if k in merged and merged[k]!=v: ok=False; break
                    merged[k]=v
                if ok:nxt.append(merged)
        bindings=nxt
        if not bindings:return []
    return bindings

def consequence_observed(c, binding, fs):
    if c=="ENABLES(target)":
        target=binding.get("target"); inter=binding.get("intervention")
        return any(p=="OBSERVED_EFFECT" and a==(inter,"ENABLES",target) for p,a,_ in fs)
    contrast=binding.get("contrast"); target=binding.get("target")
    return any(p=="NONENABLES" and a==(contrast,target) for p,a,_ in fs)

def acquire(a_ledgers):
    # Candidate schemas are induced from actual observed facts. The search is deliberately
    # finite and deterministic for the development instrument; no answer schema is loaded.
    taskfacts=[]
    for l in a_ledgers:
        from conformance.production.phase2_1_experiment import facts
        taskfacts.append(facts(l))
    if len(taskfacts)<2:return []
    schema_pool=set()
    for fs in taskfacts:
        atoms=[]
        for f in fs:
            g=generalize(f)
            if g: atoms.append(g)
        # Single observed atoms and pairs sharing an induced role are candidate antecedents.
        for k in (1,2,3,4,5,6):
            if len(atoms)<k: continue
            for comb in itertools.combinations(atoms,k):
                ctuple=tuple(canon(x) for x in comb)
                if len(set(ctuple))!=k:continue
                aa=sorted((json.loads(x) for x in ctuple),key=canon)
                for c in CONSEQUENCES:
                    schema_pool.add(canon({"atoms":aa,"consequence":c}))
    retained=[]
    for raw in sorted(schema_pool):
        obj=json.loads(raw); support=[]; contradiction=[]
        for i,fs in enumerate(taskfacts):
            binds=apply_pattern(obj["atoms"],fs)
            ok=any(consequence_observed(obj["consequence"],b,fs) for b in binds)
            if ok:support.append(i)
        if len(support)>=2:
            obj["support_tasks"]=support
            obj["hash"]=hashlib.sha256(raw).hexdigest()
            retained.append(obj)
    retained.sort(key=lambda x:(canon(x["atoms"]),x["hash"]))
    return retained

def freeze(k):
    blob=canon(k); return blob,hashlib.sha256(blob).hexdigest()
