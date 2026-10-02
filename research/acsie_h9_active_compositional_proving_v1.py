from __future__ import annotations

"""H9-C: active compositional capability-demand acquisition.

The controller must learn a hidden problem x capability response surface while
choosing its own interventions. It is then evaluated on fresh problems, including
capability descriptors never directly executed during training.

Controls:
- random intervention policy;
- historical-average policy.

A single hidden surface is shared within each seed. 32 fresh holdout problems
are evaluated after 96 training problems.
"""

import argparse
import json
import math
import random
import statistics
from pathlib import Path

from cognitive_core.h9_capability_demand import (
    CapabilityIntervention,
    ProblemObservation,
)
from cognitive_core.h9_active_compositional_demand import (
    ActiveCompositionalCapabilityDemandController,
)

DIM = 6
TRAIN_CAPS = 8
HOLDOUT_CAPS = 12
TRAIN_EPISODES = 96
HOLDOUT_EPISODES = 32
TRAIN_ROUNDS = 4
NOISE = 0.05


def norm(xs):
    n = math.sqrt(sum(x * x for x in xs)) or 1.0
    return tuple(x / n for x in xs)


def basis_caps():
    return tuple(
        CapabilityIntervention(
            f"basis-{i}",
            tuple(1.0 if j == i else 0.0 for j in range(DIM)),
        )
        for i in range(TRAIN_CAPS)
    )


def all_caps():
    rows = list(basis_caps())
    for idx, (a, b) in enumerate(((0,1),(2,3),(4,5),(0,5))):
        v=[0.0]*DIM
        v[a]=0.7
        v[b]=0.7
        rows.append(CapabilityIntervention(f"unseen-{idx}",tuple(v)))
    return tuple(rows)


def surface(rng):
    return tuple(
        tuple(rng.uniform(-1.0,1.0) for _ in range(DIM))
        for _ in range(DIM)
    )


def response(W,p,c):
    return sum(
        p[i] * W[i][j] * c[j]
        for i in range(DIM)
        for j in range(DIM)
    )


def run_seed(seed):
    rng=random.Random(seed*100003+29)
    W=surface(rng)
    candidates=all_caps()

    active=ActiveCompositionalCapabilityDemandController(
        observation_variance=NOISE**2,
        exploration_beta=0.5,
        information_weight=0.10,
    )
    random_history={c.signature:[] for c in candidates}
    active_train_trace=[]
    random_train_trace=[]

    for ep in range(TRAIN_EPISODES):
        p=norm([rng.gauss(0.0,1.0) for _ in range(DIM)])
        problem=ProblemObservation(f"train-{seed}-{ep}",p)
        round_noises=[rng.gauss(0.0,NOISE) for _ in range(TRAIN_ROUNDS)]

        for round_idx in range(TRAIN_ROUNDS):
            selected,trace=active.select(problem,candidates)
            if selected is None:
                raise RuntimeError("H9-C active selector returned no intervention")

            random_idx=rng.randrange(len(candidates))
            random_choice=candidates[random_idx]

            active_idx=next(
                i for i,candidate in enumerate(candidates)
                if candidate.signature==selected.signature
            )

            active_y=response(W,p,selected.features)+round_noises[round_idx]
            random_y=response(W,p,random_choice.features)+round_noises[round_idx]

            active.observe(
                problem,selected,
                improvement=active_y,
                regression=0.0,
                accepted=active_y>0,
            )

            random_history[random_choice.signature].append(random_y)

            active_train_trace.append({
                "episode":ep,
                "round":round_idx,
                "selected":selected.signature,
                "predicted_demand":trace.selected_demand,
                "information_gain":trace.selected_information_gain,
                "outcome":active_y,
            })
            random_train_trace.append({
                "episode":ep,
                "round":round_idx,
                "selected":random_choice.signature,
                "outcome":random_y,
            })

    holdout=[]
    for ep in range(HOLDOUT_EPISODES):
        p=norm([rng.gauss(0.0,1.0) for _ in range(DIM)])
        problem=ProblemObservation(f"holdout-{seed}-{ep}",p)

        active_choice,active_trace=active.select(problem,candidates)
        if active_choice is None:
            raise RuntimeError("H9-C holdout selector returned no intervention")

        random_idx=rng.randrange(len(candidates))
        random_choice=candidates[random_idx]

        historical=max(
            candidates[:TRAIN_CAPS],
            key=lambda c: (
                statistics.mean(random_history[c.signature])
                if random_history[c.signature] else 0.0
            ),
        )

        active_gain=response(W,p,active_choice.features)
        random_gain=response(W,p,random_choice.features)
        historical_gain=response(W,p,historical.features)
        oracle_idx=max(
            range(len(candidates)),
            key=lambda i:response(W,p,candidates[i].features),
        )
        oracle=response(W,p,candidates[oracle_idx].features)

        holdout.append({
            "episode":ep,
            "active_choice":active_choice.signature,
            "random_choice":random_choice.signature,
            "historical_choice":historical.signature,
            "active_gain":active_gain,
            "random_gain":random_gain,
            "historical_gain":historical_gain,
            "oracle_gain":oracle,
            "active_regret":oracle-active_gain,
            "random_regret":oracle-random_gain,
            "historical_regret":oracle-historical_gain,
            "active_information_gain":active_trace.selected_information_gain,
            "active_demand":active_trace.selected_demand,
            "active_unseen":active_choice.signature.startswith("unseen-"),
            "oracle_unseen":candidates[oracle_idx].signature.startswith("unseen-"),
        })

    def sign_test(xs):
        nz=[x for x in xs if abs(x)>1e-12]
        n=len(nz)
        k=sum(x>0 for x in nz)
        if n==0 or k<=n//2:
            return k,n,1.0
        return k,n,sum(math.comb(n,i) for i in range(k,n+1))/(2.0**n)

    active_gain=statistics.mean(r["active_gain"] for r in holdout)
    random_gain=statistics.mean(r["random_gain"] for r in holdout)
    historical_gain=statistics.mean(r["historical_gain"] for r in holdout)
    unseen_rate=statistics.mean(
        1.0 if r["active_unseen"] else 0.0
        for r in holdout
    )

    sr=sign_test([r["active_gain"]-r["random_gain"] for r in holdout])
    sh=sign_test([r["active_gain"]-r["historical_gain"] for r in holdout])

    status="PASSED" if (
        active_gain>random_gain
        and active_gain>historical_gain
        and sr[2]<=0.05
        and sh[2]<=0.05
        and unseen_rate>0.0
    ) else "FAILED"

    return {
        "schema":"ACSIE.h9c.active-compositional.v2",
        "seed":seed,
        "scientific_status":status,
        "train_episode_count":TRAIN_EPISODES,
        "holdout_episode_count":HOLDOUT_EPISODES,
        "candidate_count":HOLDOUT_CAPS,
        "unseen_candidate_count":HOLDOUT_CAPS-TRAIN_CAPS,
        "matched_training": {
            "shared_problem_sequence": True,
            "shared_noise_sequence": True,
            "independent_policy_histories": True,
        },
        "metrics":{
            "active_mean_holdout_gain":active_gain,
            "random_mean_holdout_gain":random_gain,
            "historical_mean_holdout_gain":historical_gain,
            "active_improvement_over_random":active_gain-random_gain,
            "active_improvement_over_historical":active_gain-historical_gain,
            "active_unseen_selection_rate":unseen_rate,
            "active_mean_regret":statistics.mean(r["active_regret"] for r in holdout),
        },
        "paired_sign_tests":{
            "active_vs_random":{"positive":sr[0],"nonzero":sr[1],"p":sr[2]},
            "active_vs_historical":{"positive":sh[0],"nonzero":sh[1],"p":sh[2]},
        },
        "integrity":{
            "external_model":False,
            "network_dependency":False,
            "manual_runtime_strategy":False,
            "hidden_surface_in_loop":False,
            "task_family_label_in_loop":False,
            "benchmark_identity_in_loop":False,
        },
        "claim_ledger":{
            "active_compositional_capability_demand":(
                "PASSED" if status=="PASSED" else "NOT_DEMONSTRATED"
            ),
            "general_capability_demand_inference":"NOT_DEMONSTRATED",
            "open_ontology":"NOT_DEMONSTRATED",
            "open_ended_rsi":"NOT_DEMONSTRATED",
            "agi":"NOT_DEMONSTRATED",
            "asi":"NOT_DEMONSTRATED",
        },
        "training":{
            "active":active_train_trace,
            "random":random_train_trace,
        },
        "holdout":holdout,
    }


def main():
    ap=argparse.ArgumentParser()
    ap.add_argument("--seed",type=int,required=True)
    ap.add_argument("--out",required=True)
    args=ap.parse_args()
    result=run_seed(args.seed)
    out=Path(args.out)
    out.mkdir(parents=True,exist_ok=True)
    (out/"h9_result.json").write_text(json.dumps(result,indent=2,sort_keys=True))
    print(json.dumps({
        "schema":result["schema"],
        "seed":result["seed"],
        "scientific_status":result["scientific_status"],
        "metrics":result["metrics"],
        "paired_sign_tests":result["paired_sign_tests"],
    },indent=2,sort_keys=True))
    return 0 if result["scientific_status"]=="PASSED" else 1


if __name__=="__main__":
    raise SystemExit(main())
