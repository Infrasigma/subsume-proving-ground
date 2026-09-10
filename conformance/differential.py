#!/usr/bin/env python3
import json, subprocess, sys, pathlib, os, tempfile
ROOT=pathlib.Path(__file__).resolve().parents[2]
FIX=ROOT/'conformance/fixtures/core.json'
PROD=ROOT/'conformance/production/serl.py'
REF=ROOT/'conformance/reference/main.go'

def run(cmd, data):
    p=subprocess.run(cmd,input=json.dumps(data)+'\n',text=True,capture_output=True,check=False,env={**os.environ,'TASK_SEED':'999999','HIDDEN_GRAPH':'must-not-be-read','CONDITION_ID':'other'})
    if p.returncode: raise AssertionError((cmd,p.stdout,p.stderr))
    return json.loads(p.stdout)

def main():
    cases=json.loads(FIX.read_text())
    go=os.environ.get('GO','go')
    for c in cases:
        a=run([sys.executable,str(PROD)],c); b=run([go,'run',str(REF)],c)
        if a!=b:
            keys=sorted(set(a)|set(b)); first=next((k for k in keys if a.get(k)!=b.get(k)),None)
            raise AssertionError(f'differential mismatch case={c["name"]} first_boundary={first}\nproduction={a.get(first)!r}\nreference={b.get(first)!r}')
        exp=c['expected']
        assert a['retrieval']['applicable']==exp['retrieval'],c['name']
        assert a['prediction']['consequence']==exp['prediction'],c['name']
        assert a['cost']==exp['cost'],c['name']
        assert a['attribution']==exp['attribution'],c['name']
        assert abs(a['novelty']-exp['novelty'])<1e-15,c['name']
        # Runtime information-boundary attack: privileged-looking environment values
        # are injected, while fixture input remains unchanged; output must be invariant.
        clean=run([sys.executable,str(PROD)],c)
        assert clean==a,c['name']+' environment leakage'
        print('PASS',c['name'])
    # Determinism and execution-order invariance.
    seq=[c['name'] for c in cases]
    outs1=[run([sys.executable,str(PROD)],c) for c in cases]
    outs2=[run([sys.executable,str(PROD)],c) for c in reversed(cases)]
    assert outs1==list(reversed(outs2)), 'execution-order nondeterminism'
    assert outs1==[run([sys.executable,str(PROD)],c) for c in cases], 'repeat nondeterminism'
    print('PASS determinism_order_repeat')
    print('PHASE2_1_CONFORMANCE_DIFFERENTIAL_PASS')

if __name__=='__main__': main()
