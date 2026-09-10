#!/usr/bin/env python3
import hashlib, json, sys

# Phase 2.1 production conformance surface. This is fixture-driven only: it never
# constructs or evaluates the definitive 100-task endpoint.

ROLE_ORDER = ["target", "intervention", "contrast", "context"]
CONSEQUENCES = {"ENABLES(target)", "NONENABLES(contrast,target)"}
PREDICATES = ["ACTION","AFTER","AVAILABLE","BEFORE","BLOCKED","ENABLES","INTERVENES","NONENABLES","OBSERVED_EFFECT","SAME_LOCAL_CONTEXT"]


def canon(x):
    return json.dumps(x, ensure_ascii=False, sort_keys=True, separators=(",", ":"), allow_nan=False).encode()

def sha(x): return hashlib.sha256(x).hexdigest()

def token(namespace, seed, index):
    b = namespace.encode()+b"\0"+seed.to_bytes(8,"big")+b"\0"+index.to_bytes(8,"big")
    return hashlib.sha256(b).hexdigest()[:16]

def rng_word(namespace, seed, counter):
    b = namespace.encode()+b"\0"+seed.to_bytes(8,"big")+b"\0"+counter.to_bytes(8,"big")
    return int.from_bytes(hashlib.sha256(b).digest()[:8],"big")

def fisher(values, namespace, seed):
    a=list(values); c=0
    for i in range(len(a)-1,0,-1):
        # Rejection-free bounded mapping is used only by this conformance helper;
        # task generation fixtures freeze the resulting semantic candidate list.
        j=rng_word(namespace,seed,c)%(i+1); c+=1; a[i],a[j]=a[j],a[i]
    return a

def canonical_candidate(rule):
    r=dict(rule)
    atoms=[]
    for a in r["atoms"]:
        atoms.append({"predicate":a["predicate"],"args":list(a["args"])})
    atoms=sorted({canon(a).decode() for a in atoms})
    atoms=[json.loads(a) for a in atoms]
    return {"atoms":atoms,"consequence":r["consequence"]}

def validate_candidate(rule):
    if rule.get("consequence") not in CONSEQUENCES: return False
    atoms=rule.get("atoms",[])
    if not 1 <= len(atoms) <= 6: return False
    seen=set()
    for a in atoms:
        if a.get("predicate") not in PREDICATES: return False
        args=a.get("args")
        if not isinstance(args,list) or any(x not in ROLE_ORDER for x in args): return False
        seen.add(canon(a))
    return len(seen)==len(atoms)

def facts(history):
    out=[]
    for i,o in enumerate(history):
        av=o["available_actions"]
        for x in av: out.append({"p":"AVAILABLE","a":[x],"at":i})
        if o.get("last_action") is not None: out.append({"p":"ACTION","a":[o["last_action"]],"at":i})
        if i>0 and o.get("last_action") is not None:
            prev=history[i-1]["available_actions"]; cur=av; x=o["last_action"]
            for target in sorted(set(prev+cur)):
                if target not in prev and target in cur:
                    out.append({"p":"INTERVENES","a":[x,target],"at":i-1})
                    out.append({"p":"OBSERVED_EFFECT","a":[x,"ENABLES("+target+")"],"at":i-1})
                if target not in prev and target not in cur:
                    out.append({"p":"NONENABLES","a":[x,target],"at":i-1})
            status=(o.get("last_result") or {}).get("status")
            if status=="BLOCKED": out.append({"p":"BLOCKED","a":[x],"at":i})
    # SAME_LOCAL_CONTEXT is derived only between actual action occurrences.
    acts=[]
    for i,o in enumerate(history):
        if o.get("last_action") is not None:
            acts.append((o["last_action"],i,canon([o["state"]["location"],o["available_actions"]])))
    for ai in range(len(acts)):
        for bi in range(ai+1,len(acts)):
            if acts[ai][2]==acts[bi][2]: out.append({"p":"SAME_LOCAL_CONTEXT","a":[acts[ai][0],acts[bi][0]],"at":min(acts[ai][1],acts[bi][1])})
    return sorted(out,key=canon)

def applicable(rule, fs):
    # Fixture-level applicability: a rule is applicable when each atom has a fact
    # with matching predicate/arity after role substitution supplied by fixture.
    binding=rule.get("binding",{})
    for a in rule["atoms"]:
        args=[binding.get(x,x) for x in a["args"]]
        if not any(f["p"]==a["predicate"] and f["a"]==args for f in fs): return False
    return True

def attribution(case):
    order=["PROTOCOL_VIOLATION","UNATTRIBUTABLE","CONFLICT","RETRIEVAL_ONLY","PREDICTION_ONLY","COINCIDENTAL","KA_TRANSFER"]
    present=set(case.get("evidence",[]))
    if case.get("outside_domain"): return "PROTOCOL_VIOLATION"
    if "invalid" in present: return "UNATTRIBUTABLE"
    if "conflict" in present: return "CONFLICT"
    if "retrieval" in present and "prediction" not in present: return "RETRIEVAL_ONLY"
    if "prediction" in present and "intervention" not in present: return "PREDICTION_ONLY"
    if "coincidental" in present: return "COINCIDENTAL"
    if {"retrieval","prediction","intervention","consequence","verification","counterfactual"} <= present: return "KA_TRANSFER"
    return "UNATTRIBUTABLE"

def cost(case):
    e=case["attempts"]
    status=case["terminal"]
    if status=="SUCCESS": return e
    if status in ("INTERACTION_CAP_EXHAUSTED","DECISION_CUTOFF_EXHAUSTED"): return e
    if status in ("ILLEGAL_ACTION","PROTOCOL_VIOLATION","INVALID_TASK","INSTRUMENTATION_FAILURE","ENVIRONMENT_ERROR"): return None
    return None

def novelty(a,b):
    sa={canon(x) for x in a}; sb={canon(x) for x in b}
    u=len(sa|sb)
    return 1.0 if u==0 else 1.0-len(sa&sb)/u

def run(f):
    h=f["history"]
    fs=facts(h)
    c=canonical_candidate(f["candidate"])
    if not validate_candidate(c): raise ValueError("invalid candidate")
    ret=applicable({**c,"binding":f.get("binding",{})},fs)
    pred=c["consequence"] if ret else None
    return {
      "input_hash":sha(canon(f)),
      "event_ledger":h,
      "facts":fs,
      "candidate":c,
      "candidate_valid":True,
      "retrieval":{"applicable":ret,"candidate":c if ret else None},
      "prediction":{"consequence":pred,"valid":pred in CONSEQUENCES if pred else False},
      "action":f.get("next_action"),
      "cost":cost(f["cost"]),
      "attribution":attribution(f["attribution"]),
      "novelty":novelty(f["novelty_a"],f["novelty_b"]),
      "task_tokens":{"action":token("ACTION_TOKEN",f["seed"],0),"location":token("LOCATION_TOKEN",f["seed"],0)},
      "rng_probe":[rng_word("TASK_GENERATION",f["seed"],i) for i in range(4)],
    }

if __name__=="__main__":
    obj=json.load(sys.stdin); print(canon(run(obj)).decode())
