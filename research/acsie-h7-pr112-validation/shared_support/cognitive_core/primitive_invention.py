"""Generic executable primitive synthesis for the common cognitive substrate.

ASI-CORE-003 constructs primitives as typed programs over task-independent low-level
operations. No task-specific primitive names are embedded in the constructor.
"""
from __future__ import annotations
from dataclasses import dataclass, asdict
from hashlib import sha256
from typing import Any, Sequence
import copy
import json
import math

Json = Any


def _canon(x: Any) -> str:
    return json.dumps(x, sort_keys=True, separators=(",", ":"), ensure_ascii=True)


def _eval_expr(tokens: Sequence[str], x: Json) -> Any:
    stack: list[Any] = []
    try:
        for tok in tokens:
            if tok == "INPUT":
                stack.append(copy.deepcopy(x))
            elif tok.startswith("CONST:"):
                stack.append(int(tok.split(":", 1)[1]))
            elif tok == "SORT":
                a = stack.pop()
                if not isinstance(a, list): return None
                stack.append(sorted(a))
            elif tok == "DROP_LAST":
                a = stack.pop()
                if not isinstance(a, list) or not a: return None
                stack.append(a[:-1])
            elif tok == "HEAD":
                a = stack.pop()
                if not isinstance(a, list) or not a: return None
                stack.append(a[0])
            elif tok == "TAIL":
                a = stack.pop()
                if not isinstance(a, list) or not a: return None
                stack.append(a[-1])
            elif tok == "LEN":
                a = stack.pop()
                if not isinstance(a, (list, tuple, str, dict)): return None
                stack.append(len(a))
            elif tok == "HALF":
                a = stack.pop()
                if not isinstance(a, (int, float)) or isinstance(a, bool): return None
                stack.append(math.floor(a / 2))
            elif tok == "DEC":
                a = stack.pop()
                if not isinstance(a, (int, float)) or isinstance(a, bool): return None
                stack.append(a - 1)
            elif tok == "INDEX":
                idx = stack.pop(); seq = stack.pop()
                if not isinstance(seq, list) or not isinstance(idx, (int, float)) or isinstance(idx, bool): return None
                idx = int(idx)
                if idx < 0 or idx >= len(seq): return None
                stack.append(seq[idx])
            elif tok == "ADD":
                b = stack.pop(); a = stack.pop()
                if not all(isinstance(v,(int,float)) and not isinstance(v,bool) for v in (a,b)): return None
                stack.append(a+b)
            elif tok == "SUB":
                b = stack.pop(); a = stack.pop()
                if not all(isinstance(v,(int,float)) and not isinstance(v,bool) for v in (a,b)): return None
                stack.append(a-b)
            elif tok == "DIV2":
                a = stack.pop()
                if not isinstance(a,(int,float)) or isinstance(a,bool): return None
                stack.append(a/2)
            elif tok == "MOD2":
                a = stack.pop()
                if not isinstance(a,(int,float)) or isinstance(a,bool): return None
                stack.append(a%2)
            elif tok == "EQ0":
                a = stack.pop()
                if not isinstance(a,(int,float)) or isinstance(a,bool): return None
                stack.append(a==0)
            elif tok == "IFZERO":
                false_v = stack.pop(); true_v = stack.pop(); cond = stack.pop()
                if not isinstance(cond, (bool,int,float)): return None
                stack.append(true_v if cond == 0 else false_v)
            else:
                return None
    except (IndexError, TypeError, ValueError, OverflowError):
        return None
    return stack[0] if len(stack)==1 else None


@dataclass(frozen=True)
class GeneratedPrimitive:
    primitive_id: str
    kind: str
    parameters: tuple[tuple[str, Any], ...] = ()
    program: tuple[str, ...] = ()
    construction_trials: int = 0

    def apply(self, x: Json) -> Json | None:
        if self.kind != "synthesized_expression":
            return None
        return _eval_expr(self.program, x)

    def spec(self) -> dict[str, Any]:
        return asdict(self)


@dataclass(frozen=True)
class _Expr:
    typ: str
    tokens: tuple[str, ...]
    values: tuple[Any, ...]
    cost: int


def _sig(values: Sequence[Any]) -> str:
    return _canon(tuple(values))


def _initial(examples: Sequence[tuple[Json, Json]]) -> list[_Expr]:
    xs=[x for x,_ in examples]
    return [
        _Expr("list", ("INPUT",), tuple(copy.deepcopy(x) for x in xs), 1),
        _Expr("list", ("INPUT","SORT"), tuple(_eval_expr(("INPUT","SORT"),x) for x in xs), 2),
        _Expr("num", ("INPUT","LEN"), tuple(len(x) if isinstance(x,(list,tuple,str,dict)) else None for x in xs), 2),
        _Expr("num", ("CONST:0",), (0,)*len(xs), 1),
        _Expr("num", ("CONST:1",), (1,)*len(xs), 1),
        _Expr("num", ("CONST:2",), (2,)*len(xs), 1),
    ]


def synthesize_generic_primitive(
    examples: Sequence[tuple[Json, Json]], *, max_depth: int = 7, beam: int = 256
) -> tuple[GeneratedPrimitive | None, int]:
    """Construct a new executable primitive from generic typed pipelines.

    The search uses only generic operations and no task-specific primitives. It deliberately
    searches program structure rather than semantic labels and validates a candidate on a
    held-out suffix of the demonstrations before retaining it.
    """
    if not examples:
        return None, 0
    cut=max(1,len(examples)//2) if len(examples)>1 else 1
    train,hold=examples[:cut],examples[cut:]
    target=tuple(y for _,y in train)
    frontier=[("list",("INPUT",))]
    trials=0
    # Typed unary transition graph; no hidden semantic operation is present.
    transitions={
        "list":(("SORT","list"),("DROP_LAST","list"),("HEAD","num"),("TAIL","num"),("LEN","num")),
        "num":(("HALF","num"),("DEC","num"),("DIV2","num"),("MOD2","num")),
    }
    seen={("list",("INPUT",))}
    for depth in range(1,max_depth+1):
        nxt=[]
        for typ,tokens in frontier:
            trials+=1
            values=tuple(_eval_expr(tokens,x) for x,_ in train)
            if typ=="num" and values==target:
                hv=tuple(_eval_expr(tokens,x) for x,_ in hold)
                if not hold or hv==tuple(y for _,y in hold):
                    pid="prim:gen:"+sha256(_canon(tokens).encode()).hexdigest()[:16]
                    return GeneratedPrimitive(pid,"synthesized_expression",(),tokens,trials),trials
            for op,out_typ in transitions.get(typ,()):
                nt=tokens+(op,)
                state=(out_typ,nt)
                if state not in seen:
                    seen.add(state); nxt.append(state)
        frontier=nxt[:beam]
        if not frontier:
            break
    return None,trials

def construct_open_primitive(examples: Sequence[tuple[Json, Json]], *, max_depth: int = 7, beam: int = 64) -> tuple[GeneratedPrimitive | None, int]:
    return synthesize_generic_primitive(examples, max_depth=max_depth, beam=beam)


def construct_with_evidence(examples: Sequence[tuple[Json, Json]]) -> tuple[GeneratedPrimitive | None, int]:
    """Compatibility alias: ASI-CORE-003 always uses generic primitive synthesis."""
    return synthesize_generic_primitive(examples, max_depth=7, beam=64)
