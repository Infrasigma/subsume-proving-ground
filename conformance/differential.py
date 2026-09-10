#!/usr/bin/env python3
import json, subprocess, sys, pathlib, os
ROOT=pathlib.Path(__file__).resolve().parents[1]
FIX=ROOT/'conformance/fixtures/core.json'; PROD=ROOT/'conformance/production/serl.py'; REF=ROOT/'conformance/reference/main.go'

def run(cmd,data):
    p=subprocess.run(cmd,input=json.dumps(data)+'\n',text=True,capture_output=True,check=False,env={**os.environ,'TASK_SEED':'999999','HIDDEN_GRAPH':'must-not-be-read','CONDITION_ID':'other'})
    if p.returncode: raise AssertionError((cmd,p.stdout,p.stderr))
    return json.loads(p.stdout)

def main():
    cases=json.loads(FIX.read_text())
    for c in cases:
        prod=run([sys.executable,str(PROD)],c); ref=run(['go','run',str(REF)],c)
        if prod!=ref:
            keys=sorted(set(prod)|set(ref)); first=next((k for k in keys if prod.get(k)!=ref.get(k)),None)
            raise AssertionError(f'differential mismatch case={c["name"]} first_boundary={first}\nproduction={prod.get(first)!r}\nreference={ref.get(first)!r}')
        exp=c['expected']
        assert prod['retrieval']['applicable']==exp['retrieval'],c['name']
        assert prod['prediction']['consequence']==exp['prediction'],c['name']
        assert prod['cost']==exp['cost'],c['name']
        assert prod['attribution']==exp['attribution'],c['name']
        assert abs(prod['novelty']-exp['novelty'])<1e-15,c['name']
        clean=run([sys.executable,str(PROD)],c); assert clean==prod,c['name']+' environment leakage'
        print('PASS',c['name'])
    forward=[run([sys.executable,str(PROD)],c) for c in cases]
    reverse=[run([sys.executable,str(PROD)],c) for c in reversed(cases)]
    assert forward==list(reversed(reverse)),'execution-order nondeterminism'
    assert forward==[run([sys.executable,str(PROD)],c) for c in cases],'repeat nondeterminism'
    print('PASS determinism_order_repeat')
    print('PHASE2_1_CONFORMANCE_DIFFERENTIAL_PASS')
if __name__=='__main__': main()
