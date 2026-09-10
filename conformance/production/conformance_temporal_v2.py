#!/usr/bin/env python3
"""Temporal fact-semantics conformance surface.

Reuses the frozen production implementation for all scientific behavior. The
only amendment is the now-frozen canonical boundary ordering: grouped Facts
are ordered by their complete canonical Fact serialization, not by an
implementation-specific grouping key.
"""
import pathlib
import sys
import json
import importlib.util

ROOT = pathlib.Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location("serl_production_temporal_base", ROOT / "conformance/production/conformance.py")
base = importlib.util.module_from_spec(spec)
spec.loader.exec_module(base)

def compile_facts_temporal(history):
    facts = base.compile_facts(history)
    return sorted(facts, key=base.cj)

base.compile_facts = compile_facts_temporal

if __name__ == "__main__":
    base.main()
