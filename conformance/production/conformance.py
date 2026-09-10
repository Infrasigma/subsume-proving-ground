#!/usr/bin/env python3
import hashlib, json, re, sys
from itertools import permutations, product

ROLES=("target","intervention","contrast","context")
CONSEQUENCES=("ENABLES(target)","NONENABLES(contrast,target)")
PRED_ARITY={"BLOCKED":1,"AVAILABLE":1,"ACTION":1,"INTERVENES":2,"BEFORE":2,"AFTER":2,"OBSERVED_EFFECT":2,"ENABLES":2,"NONENABLES":2,"SAME_LOCAL_CONTEXT":2}
TOKEN_RE=re.compile(r"^[0-9a-f]{16}$")

def cj(x): return json.dumps(x,ensure_ascii=False,sort_keys=True,separators=(",",":"),allow_nan=False).encode()
def digest(x): return hashlib.sha256(cj(x)).hexdigest()
def stream(ns,seed,counter,condition="",purpose=""):
    return (ns.encode()+b"\0"+int(seed).to_bytes(8,"big")+b"\0"+condition.encode()+b"\0"+purpose.encode()+b"\0"+int(counter).to_bytes(8,"big"))
def word(ns,seed,counter,condition="",purpose=""):
    return int.from_bytes(hashlib.sha256(stream(ns,seed,counter,condition,purpose)).digest()[:8],"big")
def bounded(ns,seed,counter,n,condition="",purpose=""):
    if n<=0: raise ValueError("n must be positive")
    limit=(1<<64)//n*n; c=counter
    while True:
        r=word(ns,seed,c,condition,purpose)
        if r<limit: return r%n,c+1
        c+=1
def fy(values,ns,seed,condition="",purpose=""):
    a=list(values); c=0
    for i in range(len(a)-1,0,-1):
        j,c=bounded(ns,seed,c,i+1,condition,purpose); a[i],a[j]=a[j],a[i]
    return a

def token(ns,seed,index,condition="",purpose=""):
    return hashlib.sha256(stream(ns,seed,index,condition,purpose)).hexdigest()[:16]

def validate_obs(o):
    if set(o)!=set(("state","available_actions","last_action","last_result","terminated")): return False
    if set(o["state"])!={"location"} or not TOKEN_RE.fullmatch(o["state"]["location"]): return False
    if sorted(o["available_actions"])!=o["available_actions"] or len(set(o["available_actions"]))!=len(o["available_actions"]): return False
    if any(not TOKEN_RE.fullmatch(x) for x in o["available_actions"]): return False
    if o["last_action"] is not None and not TOKEN_RE.fullmatch(o["last_action"]): return False
    if o["last_result"] is not None and set(o["last_result"])!={"status"}: return False
    if o["last_result"] is not None and o["last_result"]["status"] not in {"BLOCKED","ACCEPTED","SUCCESS","ILLEGAL_ACTION","ENVIRONMENT_ERROR"}: return False
    return isinstance(o["terminated"],bool)

def atom(a): return {"predicate":a["predicate"],"args":list(a["args"])}
def alpha(rule):
    # Canonicalize role aliases solely by first appearance; fixed role names remain canonical.
    used=[]
    for a in rule["atoms"]:
        for x in a["args"]:
            if x in ROLES and x not in used: used.append(x)
    mapping={x:ROLES[i] for i,x in enumerate(used)}
    # Consequence roles are canonical protocol roles and are not renamed into arbitrary roles.
    return {"atoms":[{"predicate":a["predicate"],"args":[mapping.get(x,x) for x in a["args"]]} for a in rule["atoms"]],"consequence":rule["consequence"]}

def canonical(rule):
    r=alpha(rule)
    atoms=sorted({cj(atom(a)).decode() for a in r["atoms"]})
    return {"atoms":[json.loads(x) for x in atoms],"consequence":r["consequence"]}

def validate_rule(r):
    if r.get("consequence") not in CONSEQUENCES: return False
    atoms=r.get("atoms")
    if not isinstance(atoms,list) or not 1<=len(atoms)<=6: return False
    seen=set()
    for a in atoms:
        if not isinstance(a,dict) or a.get("predicate") not in PRED_ARITY: return False
        args=a.get("args")
        if not isinstance(args,list) or len(args)!=PRED_ARITY[a["predicate"]]: return False
        if any(x not in ROLES for x in args): return False
        if cj(atom(a)) in seen: return False
        seen.add(cj(atom(a)))
    # All consequence roles must be bound in the antecedent.
    flat={x for a in atoms for x in a["args"]}
    needed={"target"} if r["consequence"]=="ENABLES(target)" else {"target","contrast"}
    return needed<=flat

def legal_candidates(max_k=2):
    # Exhaustive conformance enumerator over the frozen typed atom grammar. max_k is
    # supplied only by non-endpoint closure fixtures; no production experiment uses it.
    templates=[]
    for p,n in PRED_ARITY.items():
        templates.append([{"predicate":p,"args":list(args)} for args in product(ROLES,repeat=n)])
    atoms=[x for group in templates for x in group]
    seen={}
    for k in range(1,max_k+1):
        for tup in product(atoms,repeat=k):
            if len({cj(a) for a in tup})!=k: continue
            for c in CONSEQUENCES:
                r={"atoms":list(tup),"consequence":c}; cr=canonical(r)
                if validate_rule(cr): seen[cj(cr)]=cr
    return [seen[k] for k in sorted(seen)]

def compile_facts(history):
    facts=[]
    for i,o in enumerate(history):
        for x in o["available_actions"]: facts.append({"p":"AVAILABLE","a":[x],"at":[i]})
        if o["last_action"] is not None: facts.append({"p":"ACTION","a":[o["last_action"]],"at":[i]})
        if i:
            prev=set(history[i-1]["available_actions"]); cur=set(o["available_actions"]); x=o["last_action"]
            if x is not None:
                for t in sorted(prev|cur):
                    if t not in prev and t in cur:
                        facts.append({"p":"INTERVENES","a":[x,t],"at":[i-1]})
                        facts.append({"p":"OBSERVED_EFFECT","a":[x,"ENABLES("+t+")"],"at":[i-1]})
                    elif t not in prev and t not in cur:
                        facts.append({"p":"NONENABLES","a":[x,t],"at":[i-1]})
                if o["last_result"] and o["last_result"]["status"]=="BLOCKED": facts.append({"p":"BLOCKED","a":[x],"at":[i]})
    # Normalize duplicate facts while retaining all provenance.
    grouped={}
    for f in facts:
        key=cj({"p":f["p"],"a":f["a"]})
        grouped.setdefault(key,{"p":f["p"],"a":f["a"],"at":[]})["at"]+=f["at"]
    for f in grouped.values(): f["at"]=sorted(set(f["at"]))
    return [grouped[k] for k in sorted(grouped)]

def grounded_atoms(rule,binding,facts):
    for a in rule["atoms"]:
        args=[binding[x] for x in a["args"]]
        if not any(f["p"]==a["predicate"] and f["a"]==args for f in facts): return False
    return True

def acquire(a_histories,pre=None):
    pre=pre or []
    pre_ids={cj(c) for c in pre}; records={}
    for seed,h in a_histories:
        fs=compile_facts(h); entities=sorted({x for o in h for x in o["available_actions"]}|{o["last_action"] for o in h if o["last_action"] is not None})
        for r in legal_candidates(2):
            needed=sorted({x for a in r["atoms"] for x in a["args"]})
            if len(needed)>len(entities): continue
            supported=False; prov=None
            for vals in permutations(entities,len(needed)):
                b=dict(zip(needed,vals))
                if grounded_atoms(r,b,fs): supported=True; prov={"seed":seed,"binding":b,"facts_hash":digest(fs)}; break
            if supported and cj(r) not in pre_ids:
                key=cj(r); rec=records.setdefault(key,{"rule":r,"support_seeds":[]})
                rec["support_seeds"].append(seed); rec.setdefault("provenance",[]).append(prov)
    out=[]
    for k,v in records.items():
        seeds=sorted(set(v["support_seeds"]))
        if len(seeds)>=2:
            v["support_seeds"]=seeds; v["rule_hash"]=digest(v["rule"]); out.append(v)
    out.sort(key=lambda x:(cj(x["rule"]),x["rule_hash"]))
    return out

def retrieve(k,history):
    fs=compile_facts(history); entities=sorted({x for o in history for x in o["available_actions"]}|{o["last_action"] for o in history if o["last_action"] is not None})
    matches=[]
    for rec in k:
        r=rec["rule"]; roles=sorted({x for a in r["atoms"] for x in a["args"]},key=lambda x:ROLES.index(x));
        for vals in product(entities,repeat=len(roles)):
            b=dict(zip(roles,vals))
            if grounded_atoms(r,b,fs): matches.append((len(r["atoms"]),len(roles),cj(r),rec,b)); break
    matches.sort(key=lambda x:(-x[0],-x[1],x[2]))
    if not matches: return {"status":"NONE"}
    top=matches[0]; conflicts=[m for m in matches if m[0:2]==top[0:2] and m[3]["rule"]["consequence"]!=top[3]["rule"]["consequence"]]
    if conflicts: return {"status":"CONFLICT","candidates":[x[3]["rule_hash"] for x in [top,*conflicts]]}
    return {"status":"RETRIEVED","rule_hash":top[3]["rule_hash"],"binding":top[4]}

def attribution(e):
    if e.get("outside_domain"): return "PROTOCOL_VIOLATION"
    s=set(e.get("evidence",[]))
    if "invalid" in s or not s: return "UNATTRIBUTABLE"
    if "conflict" in s: return "CONFLICT"
    if "retrieval" in s and "prediction" not in s: return "RETRIEVAL_ONLY"
    if "prediction" in s and "intervention" not in s: return "PREDICTION_ONLY"
    if "coincidental" in s: return "COINCIDENTAL"
    need={"retrieval","grounding","prediction","decisive_action","intervention","consequence","verification","counterfactual"}
    return "KA_TRANSFER" if need<=s else "UNATTRIBUTABLE"

def cost(attempts,terminal):
    if terminal in {"SUCCESS","INTERACTION_CAP_EXHAUSTED","DECISION_CUTOFF_EXHAUSTED"}: return attempts
    return None

def novelty(a,b):
    A=[]; B=[]
    for x in a: A.append(cj(x))
    for x in b: B.append(cj(x))
    ca={x:A.count(x) for x in set(A)}; cb={x:B.count(x) for x in set(B)}
    u=sum(max(ca.get(k,0),cb.get(k,0)) for k in set(ca)|set(cb)); i=sum(min(ca.get(k,0),cb.get(k,0)) for k in set(ca)|set(cb))
    return 1.0 if u==0 else 1.0-i/u

def controls(history):
    # All controls are executable decision surfaces on the same visible history.
    actions=history[-1]["available_actions"]
    return {
      "K0":sorted(actions)[0] if actions else None,
      "KR":sorted(actions)[0] if actions else None,
      "KS":sorted(actions)[0] if actions else None,
      "KP":sorted(actions)[0] if actions else None,
      "REPLAY":None,
    }

def run(f):
    for o in f["history"]:
        if not validate_obs(o): raise ValueError("invalid observation")
    fs=compile_facts(f["history"]); c=canonical(f["candidate"]); valid=validate_rule(c)
    if not valid: raise ValueError("invalid candidate")
    ret=retrieve([{ "rule":c,"rule_hash":digest(c)}],f["history"])
    pred=c["consequence"] if ret["status"]=="RETRIEVED" else None
    return {
      "input_hash":digest(f),"facts":fs,"candidate":c,"candidate_valid":True,
      "retrieval":ret,"prediction":{"consequence":pred,"valid":pred in CONSEQUENCES if pred else False},
      "cost":cost(f["attempts"],f["terminal"]),"attribution":attribution(f["attribution"]),
      "novelty":novelty(f["novelty_a"],f["novelty_b"]),"controls":controls(f["history"]),
      "tokens":{"action":token("LABEL_PERMUTATION",f["seed"],0,purpose="action_token_pool"),"location":token("LABEL_PERMUTATION",f["seed"],0,purpose="location_token_pool")},
      "rng":[word("TASK_GENERATION",f["seed"],i,purpose="family_b_dependency_subset") for i in range(4)],
    }

def main():
    f=json.load(sys.stdin); print(cj(run(f)).decode())
if __name__=="__main__": main()
