"""Independent CORE-011 environment generators. No generator metadata is exposed to the learner."""
from __future__ import annotations
from dataclasses import dataclass
import copy, random
from typing import Any

Json=Any
GENERATOR_VERSION='asi-core-011-independent-stream-generators-v3-delayed-horizon-corrected'

@dataclass
class Phase:
    name:str
    length:int
    action_space:tuple[str,...]
    state:dict[str,Any]
    seed:int
    t:int=0
    pending:list[tuple[int,int]]|None=None
    def __post_init__(self):
        if self.pending is None: self.pending=[]
    def observe(self):
        s=self.state; k=self.name
        noise=(self.seed*17+self.t*31)%11
        if k in ('alpha','beta'):
            return {'state':list(s['xy']),'tick':[self.t%4],'probe':[s['probe']],'noise':[noise]}
        if k=='gamma':
            return {'a':[s['xy'][0]],'b':[s['xy'][1]],'mark':[s['probe']],'noise':[noise]}
        if k=='delta':
            rows=[{'id':'left','value':s['xy'][0],'junk':(self.t*3+self.seed)%13},{'id':'right','value':s['xy'][1],'junk':(self.t*5+self.seed)%13}]
            if (self.seed+self.t)%2: rows.reverse()
            return {'items':rows,'noise':[noise]}
        if k=='epsilon':
            return {'q':[s['x']],'clock':self.t,'flag':[0]}
        if k=='zeta':
            return {'state':list(s['xy']),'tick':[self.t%4],'probe':[0],'noise':[noise]}
        raise KeyError(k)
    def step(self,action:str):
        before=self.observe(); s=self.state; k=self.name
        # Delayed consequences mature before the current action, so the learner must
        # predict at exact horizons rather than mistake immediate inactivity for no-op truth.
        for due,d in list(self.pending):
            if due==self.t: s['x']+=d
        self.pending=[z for z in self.pending if z[0]!=self.t]
        if k in ('alpha','beta'):
            mode=0 if k=='alpha' else 1
            if action=='u': s['xy'][0]+= (1 if mode==0 else 3)
            elif action=='v': s['xy'][1]+= (-1 if mode==0 else -2)
            elif action=='probe': s['probe']=1 if mode==0 else 2
        elif k=='gamma':
            if action=='u': s['xy'][0]+=1
            elif action=='v': s['xy'][1]-=1
            elif action=='probe2': s['probe']=1
        elif k=='delta':
            if action=='u': s['xy'][0]+=1
            elif action=='v': s['xy'][1]-=1
        elif k=='epsilon':
            if action=='d2': self.pending.append((self.t+1,2))
            elif action=='d3': self.pending.append((self.t+2,-1))
        elif k=='zeta':
            mode=s['mode']
            if action=='u': s['xy'][0]+=1 if mode==0 else -1
            elif action=='v': s['xy'][1]+=2 if mode==0 else 0
            elif action=='s': s['xy']=[s['xy'][1],s['xy'][0]]
            if self.t in s['switches']: s['mode']=1-s['mode']
        self.t+=1
        return self.observe()


def make_phase(name:str, seed:int, length:int):
    r=random.Random(seed*1009+len(name)*17)
    x=r.randint(-4,5); y=r.randint(-4,5)
    if name in ('alpha','beta','gamma','delta'):
        actions={'alpha':('u','v','probe'),'beta':('u','v','probe'),'gamma':('u','v','probe2'),'delta':('u','v')}[name]
        state={'xy':[x,y],'probe':0}
    elif name=='epsilon': actions=('d2','d3','wait'); state={'x':x,'mode':0};
    elif name=='zeta': actions=('u','v','s'); state={'xy':[x,y],'probe':0,'mode':0,'switches':(length//2,)}
    else: raise KeyError(name)
    return Phase(name,length,actions,state,seed)

STREAM_TEMPLATE=(
    'alpha','beta','gamma','alpha','delta','beta','epsilon','alpha',
    'zeta','gamma','beta','alpha'
)

def stream(seed:int,lengths:tuple[int,...]|None=None):
    if lengths is None: lengths=tuple(10+(seed+i)%5 for i in range(len(STREAM_TEMPLATE)))
    phases=[]; for_i=0
    for i,(name,n) in enumerate(zip(STREAM_TEMPLATE,lengths)):
        phases.append(make_phase(name,seed*100+i,n))
    return phases
