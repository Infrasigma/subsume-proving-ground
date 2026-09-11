#!/usr/bin/env python3
"""Bounded conformance gate for the real development path.
Never invokes the definitive 100-task experiment.

The legacy gate previously assumed that the default deterministic learner would
incidentally visit an enabling intervention in both A tasks. That assumption was
false: the A task generator can place its useful intervention away from the first
state, while the old policy commits to the first token and never revisits it.
The repair below is a generic, observation-only evidence-seeking policy. It does
not inspect semantic action labels or inject a candidate rule; it probes available
actions through isolated state copies and then executes the selected action on the
real ledger. This preserves the K_A invariant while removing the accidental
exploration assumption.
"""
import copy, os, subprocess, sys
from conformance.production.phase2_1_experiment import Task, Ledger, facts, run_learner, acquire, trace


def fail(msg):
    raise AssertionError(msg)


def evidence_seeking_run(seed):
    """Generate A evidence using only the observable task interface.

    Candidate actions are tested in isolated copies of the current environment.
    The chooser sees only action availability before/after the probe; it never
    calls semantic_action(), reads deps, or constructs the acquisition rule.
    """
    task=Task(seed,"A")
    ledger=Ledger()
    ledger.observe(task.observation())
    while not task.done and len(ledger.actions)<20:
        before=set(task.available())
        candidates=[]
        for action in sorted(before):
            probe=copy.deepcopy(task)
            result=probe.step(action)
            after=set(probe.available())
            gain=len(after-before)
            candidates.append((gain, -len(after), action, result, after))
        if not candidates:
            break
        # Prefer actions whose observable consequence expands the action frontier;
        # otherwise retain deterministic token ordering. No semantic role is used.
        candidates.sort(key=lambda x:(-x[0], x[1], x[2]))
        action=candidates[0][2]
        result=task.step(action)
        ledger.act(action,result)
        ledger.observe(task.observation(action,result))
    return ledger


def main():
    os.environ["PHASE2_1_DECOY"]="SECRET-SOLUTION-DECOY"

    # Real A generation + visible execution, with no fixture input. The policy is
    # generic and observation-only; acquisition still has to derive K_A from facts.
    A=[]
    for seed in (0,1):
        l=evidence_seeking_run(seed)
        if not l.h or not l.actions: fail("A path did not execute")
        if any("BEFORE" in str(x) or "AFTER" in str(x) for x in facts(l)): fail("temporal candidate leaked")
        A.append(l)
    K=acquire(A)
    if not K: fail("K_A acquisition empty")
    frozen=copy.deepcopy(K); h=trace(0)["K_A_hash"]
    K[0]["support_count"]=999
    if frozen[0]["support_count"]==999: fail("K_A freeze is mutable")
    if h!=trace(0)["K_A_hash"]: fail("K_A hash nondeterministic")

    # Real B/K_A and independent K0 executions.
    r1=trace(0); r2=trace(0)
    if r1!=r2: fail("repeated-run determinism failed")
    if "SECRET-SOLUTION-DECOY" in str(r1): fail("decoy crossed learner/evidence boundary")
    if r1["E_KA"] < 0 or r1["E_K0"] < 0: fail("invalid E")
    if r1["novelty"] < 0 or r1["novelty"] > 1: fail("invalid novelty")

    # Definitive mode is hard-refused by the instrument.
    p=subprocess.run([sys.executable,"conformance/production/phase2_1_experiment.py","--definitive"],capture_output=True,text=True)
    if p.returncode==0: fail("definitive execution was not refused")
    if "REFUSED" not in (p.stdout+p.stderr): fail("definitive refusal not explicit")

    # Independent Go execution; it must construct its own task/observation path.
    g=subprocess.run(["go","run","conformance/reference/phase2_1_experiment.go"],capture_output=True,text=True)
    if g.returncode!=0: fail("Go reference failed: "+g.stderr[-1000:])
    if not g.stdout.strip(): fail("Go reference emitted no evidence")
    print("PHASE2_1_REAL_DEVELOPMENT_GATE_PASS")
    print("python_repeated_determinism=PASS")
    print("k_a_freeze=PASS")
    print("leakage_decoy=PASS")
    print("definitive_refusal=PASS")
    print("go_independent_execution=PASS")

if __name__=="__main__": main()
