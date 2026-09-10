#!/usr/bin/env python3
import json, subprocess, sys, pathlib, os
ROOT=pathlib.Path(__file__).resolve().parents[1]
LEGACY_FIX=ROOT/'conformance/fixtures/core.json'; LEGACY_PROD=ROOT/'conformance/production/serl.py'; LEGACY_REF=ROOT/'conformance/reference/main.go'
ORIGINAL_ATTR_FIX=ROOT/'conformance/fixtures/evidence/negative_nonenablement_missing_entity_original.json'
CLOSURE_FIX=ROOT/'conformance/fixtures/closure.json'; CLOSURE_PROD=ROOT/'conformance/production/conformance.py'; CLOSURE_REF=ROOT/'conformance/reference/closure.go'

def run(cmd,data):
    p=subprocess.run(cmd,input=json.dumps(data)+'\n',text=True,capture_output=True,check=False,env={**os.environ,'TASK_SEED':'999999','HIDDEN_GRAPH':'must-not-be-read','CONDITION_ID':'other','SEMANTIC_X':'must-not-be-read','SIMULATOR_STATE':'must-not-be-read','B_SOLUTION':'must-not-be-read'})
    if p.returncode: raise AssertionError((cmd,p.stdout,p.stderr))
    raw=p.stdout.strip().encode(); return json.loads(raw),raw

def fact_semantics(x): return sorted(json.dumps(f,ensure_ascii=False,sort_keys=True,separators=(',',':')) for f in x)
def boundary_bytes(x):
    y={k:v for k,v in x.items() if k!='input_hash'}
    return json.dumps(y,ensure_ascii=False,sort_keys=True,separators=(',',':'),allow_nan=False).encode()
def compare(a,b,name,raw_a,raw_b):
    if fact_semantics(a.get('facts',[])) != fact_semantics(b.get('facts',[])):
        raise AssertionError(f'differential mismatch case={name} first_boundary=facts semantic_mismatch\nproduction={a.get("facts")!r}\nreference={b.get("facts")!r}')
    aa={k:v for k,v in a.items() if k!='input_hash'}; bb={k:v for k,v in b.items() if k!='input_hash'}
    if aa!=bb:
        keys=sorted(set(aa)|set(bb)); first=next((k for k in keys if aa.get(k)!=bb.get(k)),None)
        raise AssertionError(f'differential mismatch case={name} first_boundary={first}\nproduction={aa.get(first)!r}\nreference={bb.get(first)!r}')
    can_a=boundary_bytes(a); can_b=boundary_bytes(b)
    if can_a != can_b:
        raise AssertionError(f'canonical serialization mismatch case={name} first_boundary=bytes\nproduction={can_a!r}\nreference={can_b!r}')

def legacy():
    cases=json.loads(LEGACY_FIX.read_text())
    for c in cases:
        prod,prod_raw=run([sys.executable,str(LEGACY_PROD)],c); ref,ref_raw=run(['go','run',str(LEGACY_REF)],c); compare(prod,ref,c['name'],prod_raw,ref_raw)
        exp=c['expected']
        assert prod['retrieval']['applicable']==exp['retrieval'],c['name']
        assert prod['prediction']['consequence']==exp['prediction'],c['name']
        assert prod['cost']==exp['cost'],c['name']
        assert prod['attribution']==exp['attribution'],c['name']
        assert abs(prod['novelty']-exp['novelty'])<1e-15,c['name']
        if c['name']=='negative_nonenablement_missing_entity':
            assert type(prod['novelty']) is int and type(ref['novelty']) is int,'numeric canonical regression'
        clean,clean_raw=run([sys.executable,str(LEGACY_PROD)],c); assert clean==prod and clean_raw==prod_raw,c['name']+' environment leakage'
        print('PASS legacy_regression',c['name'])
    # Regression B: preserve the exact contradictory historical fixture, but
    # treat its stale attribution evidence as adversarial input. Actual retrieval
    # is false, so KA_TRANSFER must remain impossible.
    original=json.loads(ORIGINAL_ATTR_FIX.read_text())
    op,oraw=run([sys.executable,str(LEGACY_PROD)],original); rr,rraw=run(['go','run',str(LEGACY_REF)],original)
    compare(op,rr,'original_attribution_attack',oraw,rraw)
    assert op['retrieval']['applicable'] is False and op['attribution']=='UNATTRIBUTABLE','attribution invariant regression'
    print('PASS attribution_invariant_regression')

def closure():
    cases=json.loads(CLOSURE_FIX.read_text())
    for c in cases:
        prod,prod_raw=run([sys.executable,str(CLOSURE_PROD)],c); ref,ref_raw=run(['go','run',str(CLOSURE_REF)],c); compare(prod,ref,c['name'],prod_raw,ref_raw)
        assert prod['candidate_valid'] is True,c['name']; assert prod['prediction']['valid'] is True,c['name']; print('PASS closure_differential',c['name'])
    forward=[run([sys.executable,str(CLOSURE_PROD)],c) for c in cases]; reverse=[run([sys.executable,str(CLOSURE_PROD)],c) for c in reversed(cases)]
    assert forward==list(reversed(reverse)),'closure execution-order nondeterminism'; assert forward==[run([sys.executable,str(CLOSURE_PROD)],c) for c in cases],'closure repeat nondeterminism'; print('PASS closure_determinism_order_repeat')
def main():
    legacy(); closure(); print('PHASE2_1_CONFORMANCE_DIFFERENTIAL_PASS')
if __name__=='__main__': main()
