#!/usr/bin/env python3
# H9-E execution marker: posterior-sampling acquisition proof
from __future__ import annotations
import argparse, math, os, random, statistics
from dataclasses import dataclass
from typing import Sequence
EXPECTED_ACSIE_REF="2c305db9fab8d6fbedd47262cb3da950479f705b"

def exact_sign_test(diffs:Sequence[float])->tuple[float,int]:
    nz=[d for d in diffs if abs(d)>1e-12]; n=len(nz)
    if not n: return 1.0,0
    k=sum(d>0 for d in nz); low=min(k,n-k)
    p=min(1.0,2.0*sum(math.comb(n,i) for i in range(low+1))/(2.0**n))
    return p,n

@dataclass(frozen=True)
class Candidate:
    signature:str
    features:tuple[float,...]
    cost:float=1.0

@dataclass(frozen=True)
class Problem:
    signature:str
    features:tuple[float,...]

def candidates():
    rows=[("b0",(1,0,0,0,0,0)),("b1",(0,1,0,0,0,0)),("b2",(0,0,1,0,0,0)),
          ("b3",(0,0,0,1,0,0)),("b4",(0,0,0,0,1,0)),("b5",(0,0,0,0,0,1)),
          ("b6",(1,1,0,0,0,0)),("b7",(0,0,1,1,0,0)),("u0",(1,0,1,0,0,1)),
          ("u1",(0,1,0,1,1,0)),("u2",(1,0,0,1,0,1)),("u3",(0,1,1,0,1,0))]
    return tuple(Candidate(k,tuple(float(x) for x in v)) for k,v in rows)

def make_surface(seed):
    rng=random.Random(seed); bias=rng.uniform(-0.08,0.08)
    linear=[rng.uniform(-0.16,0.16) for _ in range(6)]
    interaction=[[rng.uniform(-0.28,0.28) for _ in range(6)] for _ in range(6)]
    def latent(problem,candidate):
        p,c=problem.features,candidate.features
        return bias+sum(linear[i]*p[i] for i in range(6))+sum(interaction[i][j]*p[i]*c[j] for i in range(6) for j in range(6))
    return latent

def sample_problem(rng,prefix,idx):
    return Problem(prefix+"-"+str(idx),tuple(rng.uniform(-1.0,1.0) for _ in range(6)))

def build_controller(module):
    return module.PosteriorSamplingCapabilityDemandController(observation_variance=0.03**2,prior_variance=1.0,safety_epsilon=0.02)

def observe(controller,base,problem,candidate,value):
    controller.observe(base.ProblemObservation(problem.signature,problem.features),
                       base.CapabilityIntervention(candidate.signature,candidate.features,candidate.cost),
                       improvement=max(0.0,value),regression=max(0.0,-value),accepted=value>=0.0)

def select(controller,base,problem,pool):
    xs=tuple(base.CapabilityIntervention(c.signature,c.features,c.cost) for c in pool)
    chosen,_=controller.select(base.ProblemObservation(problem.signature,problem.features),xs)
    if chosen is None: raise RuntimeError("controller returned no intervention")
    return next(c for c in pool if c.signature==chosen.signature)

def run_seed(seed,module,base):
    rng=random.Random(seed); noise=random.Random(seed*7919+17); surface=make_surface(seed); pool=candidates()
    active=build_controller(module); random_policy=build_controller(module); random_history=[]
    training=[sample_problem(rng,"train",i) for i in range(96)]
    for _ in range(4):
        for problem in training:
            a=select(active,base,problem,pool); r=pool[rng.randrange(len(pool))]
            shared=noise.gauss(0.0,0.03); av=surface(problem,a)+shared; rv=surface(problem,r)+shared
            observe(active,base,problem,a,av); observe(random_policy,base,problem,r,rv); random_history.append((problem,r,rv))
    historical=build_controller(module)
    for problem,r,rv in random_history: observe(historical,base,problem,r,rv)
    holdout=[sample_problem(rng,"holdout",i) for i in range(32)]
    active_scores=[]; random_scores=[]; hist_scores=[]; unseen=runseen=hunseen=0
    for problem in holdout:
        a=select(active,base,problem,pool); r=pool[rng.randrange(len(pool))]; h=select(historical,base,problem,pool)
        shared=noise.gauss(0.0,0.03)
        active_scores.append(surface(problem,a)+shared); random_scores.append(surface(problem,r)+shared); hist_scores.append(surface(problem,h)+shared)
        unseen+=int(a.signature.startswith("u")); runseen+=int(r.signature.startswith("u")); hunseen+=int(h.signature.startswith("u"))
    ar=[a-r for a,r in zip(active_scores,random_scores)]; ah=[a-h for a,h in zip(active_scores,hist_scores)]
    pr,nr=exact_sign_test(ar); ph,nh=exact_sign_test(ah)
    return {"seed":seed,"active_mean":statistics.fmean(active_scores),"random_mean":statistics.fmean(random_scores),"historical_mean":statistics.fmean(hist_scores),
            "active_vs_random_delta":statistics.fmean(ar),"active_vs_historical_delta":statistics.fmean(ah),
            "p_active_vs_random":pr,"n_active_vs_random":nr,"p_active_vs_historical":ph,"n_active_vs_historical":nh,
            "unseen_selection_rate":unseen/32,"random_unseen_rate":runseen/32,"historical_unseen_rate":hunseen/32}

def main():
    p=argparse.ArgumentParser(); p.add_argument("--seeds",default="2026100301,2026100302,2026100303,2026100304,2026100305"); p.add_argument("--strict",action="store_true"); a=p.parse_args()
    import cognitive_core.h9_posterior_sampling_demand as module
    import cognitive_core.h9_capability_demand as base
    actual=os.environ.get("ACSIE_REF",""); print("ACSIE_REF="+actual); print("EXPECTED_ACSIE_REF="+EXPECTED_ACSIE_REF)
    if actual!=EXPECTED_ACSIE_REF: print("STATUS=INVALID"); return 2
    rows=[run_seed(int(x),module,base) for x in a.seeds.split(",") if x.strip()]
    for row in rows: print(row)
    gate=all(r["active_mean"]>r["random_mean"] and r["active_mean"]>r["historical_mean"] and r["p_active_vs_random"]<=0.05 and r["p_active_vs_historical"]<=0.05 and r["n_active_vs_random"]>=16 and r["n_active_vs_historical"]>=16 for r in rows)
    print("FIVE_SEED_GATE="+("PASS" if gate else "FAIL"))
    return 0 if (gate or not a.strict) else 2

if __name__=="__main__": raise SystemExit(main())

# execution marker: runner router installed on proving-ground main

# execution marker: exact-SHA branch-fetch repair installed
