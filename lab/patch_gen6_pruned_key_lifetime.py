#!/usr/bin/env python3
"""Apply an ephemeral, identity-pinned Gen6 stale-token-map cleanup patch.

This changes only an isolated ACSIE checkout inside the proving-ground workflow.
It does not commit or modify ACSIE source.
"""
from __future__ import annotations

import argparse
import hashlib
from pathlib import Path

EXPECTED_SOURCE_SHA256 = "2ff981875397b7d0edb2fef1c33c655b0a15b8c404b13c50d13a804f77535aea"


def replace_exact(source: str, old: str, new: str, *, count: int = 1, label: str) -> str:
    observed = source.count(old)
    if observed != count:
        raise SystemExit(f"{label}: expected {count} marker(s), found {observed}: {old[:100]!r}")
    return source.replace(old, new, count)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("source", type=Path)
    args = parser.parse_args()
    path = args.source
    source = path.read_text(encoding="utf-8")
    source_sha = hashlib.sha256(source.encode()).hexdigest()
    if source_sha != EXPECTED_SOURCE_SHA256:
        raise SystemExit(f"source SHA mismatch: expected {EXPECTED_SOURCE_SHA256}, got {source_sha}")
    if "_H7_KEY_LIFETIME_STATS" in source:
        raise SystemExit("diagnostic patch marker already present")

    block = '''    _H7_RELEASE_PRUNED_KEYS = __import__("os").environ.get("ACSIE_RELEASE_PRUNED_KEYS") == "1"
    _H7_KEY_LIFETIME_STARTED = __import__("time").monotonic()
    _H7_KEY_LIFETIME_STATS = {
        "release_enabled": _H7_RELEASE_PRUNED_KEYS,
        "registered_state_tokens": 0,
        "rejected_candidate_opportunities": 0,
        "pruned_existing_opportunities": 0,
        "release_events": 0,
        "expression_key_entries_removed": 0,
        "ast_depth_entries_removed": 0,
        "expression_key_entries_would_remove": 0,
        "ast_depth_entries_would_remove": 0,
        "peak_expression_key_entries": 0,
        "peak_ast_depth_entries": 0,
        "expression_key_entries_at_last_sample": 0,
        "ast_depth_entries_at_last_sample": 0,
        "seen_expression_keys_at_last_sample": 0,
        "online_metadata_entries_at_last_sample": 0,
        "assertion_checks": 0,
        "assertion_failures": 0,
    }

    def _H7_key_lifetime_sample(event: str, reason: str | None = None) -> None:
        _H7_KEY_LIFETIME_STATS["expression_key_entries_at_last_sample"] = len(expression_key_by_state_token)
        _H7_KEY_LIFETIME_STATS["ast_depth_entries_at_last_sample"] = len(ast_depth_by_state_token)
        _H7_KEY_LIFETIME_STATS["seen_expression_keys_at_last_sample"] = len(seen_expression_keys)
        _H7_KEY_LIFETIME_STATS["online_metadata_entries_at_last_sample"] = len(_H7_online_metadata)
        _H7_KEY_LIFETIME_STATS["peak_expression_key_entries"] = max(
            _H7_KEY_LIFETIME_STATS["peak_expression_key_entries"], len(expression_key_by_state_token)
        )
        _H7_KEY_LIFETIME_STATS["peak_ast_depth_entries"] = max(
            _H7_KEY_LIFETIME_STATS["peak_ast_depth_entries"], len(ast_depth_by_state_token)
        )
        log_path = __import__("os").environ.get("ACSIE_PRUNED_KEY_LIFETIME_LOG")
        if not log_path:
            return
        row = {
            "elapsed_s": round(__import__("time").monotonic() - _H7_KEY_LIFETIME_STARTED, 3),
            "event": event,
            "reason": reason,
            "release_enabled": _H7_RELEASE_PRUNED_KEYS,
            **_H7_KEY_LIFETIME_STATS,
        }
        with open(log_path, "a", encoding="utf-8") as handle:
            handle.write(json.dumps(row, sort_keys=True, separators=(",", ":")) + "\n")
            handle.flush()

    def _H7_release_pruned_token(
        state: tuple, reason: str, *, level_bucket_absent: bool
    ) -> None:
        token = state[5]
        if reason == "rejected_candidate":
            _H7_KEY_LIFETIME_STATS["rejected_candidate_opportunities"] += 1
        elif reason == "pruned_existing":
            _H7_KEY_LIFETIME_STATS["pruned_existing_opportunities"] += 1
        else:
            raise AssertionError("unknown pruned-key release reason")

        _H7_KEY_LIFETIME_STATS["assertion_checks"] += 1
        behavior_key = (state[1], state[3])
        active_index = _H7_online_indexes.get(behavior_key)
        if active_index is not None and active_index["active"].get(token) is state:
            _H7_KEY_LIFETIME_STATS["assertion_failures"] += 1
            raise AssertionError("cannot release a token still present in its active frontier")
        if not level_bucket_absent:
            _H7_KEY_LIFETIME_STATS["assertion_failures"] += 1
            raise AssertionError("cannot release a token still present in its level composition bucket")

        has_expression_key = token in expression_key_by_state_token
        has_ast_depth = token in ast_depth_by_state_token
        _H7_KEY_LIFETIME_STATS["expression_key_entries_would_remove"] += int(has_expression_key)
        _H7_KEY_LIFETIME_STATS["ast_depth_entries_would_remove"] += int(has_ast_depth)
        if _H7_RELEASE_PRUNED_KEYS:
            _H7_KEY_LIFETIME_STATS["expression_key_entries_removed"] += int(
                expression_key_by_state_token.pop(token, None) is not None
            )
            _H7_KEY_LIFETIME_STATS["ast_depth_entries_removed"] += int(
                ast_depth_by_state_token.pop(token, None) is not None
            )
        _H7_KEY_LIFETIME_STATS["release_events"] += 1
        if _H7_KEY_LIFETIME_STATS["release_events"] % 4096 == 0:
            _H7_key_lifetime_sample("release_batch", reason)

    def _H7_note_registered_token() -> None:
        _H7_KEY_LIFETIME_STATS["registered_state_tokens"] += 1
        _H7_KEY_LIFETIME_STATS["peak_expression_key_entries"] = max(
            _H7_KEY_LIFETIME_STATS["peak_expression_key_entries"], len(expression_key_by_state_token)
        )
        _H7_KEY_LIFETIME_STATS["peak_ast_depth_entries"] = max(
            _H7_KEY_LIFETIME_STATS["peak_ast_depth_entries"], len(ast_depth_by_state_token)
        )
        if _H7_KEY_LIFETIME_STATS["registered_state_tokens"] % 4096 == 0:
            _H7_key_lifetime_sample("registration_batch")

'''
    source = replace_exact(
        source,
        '    ast_depth_by_state_token: dict[int, int] = {}\n',
        '    ast_depth_by_state_token: dict[int, int] = {}\n' + block,
        label="insert lifetime diagnostics",
    )

    old = '''        vector_cache.pop(ident, None)
        free_cache.pop(ident, None)

        # Invocation-local exact index'''
    new = '''        vector_cache.pop(ident, None)
        free_cache.pop(ident, None)
        _H7_release_pruned_token(
            state, "pruned_existing",
            level_bucket_absent=(bucket is None or bucket.get(state_key) is not state),
        )

        # Invocation-local exact index'''
    source = replace_exact(source, old, new, label="release dominated existing state keys")

    old = '''        ast_depth_by_state_token[state[5]] = depth
        expanded_depth, expanded_nodes = executable_cost(expr)'''
    new = '''        ast_depth_by_state_token[state[5]] = depth
        _H7_note_registered_token()
        expanded_depth, expanded_nodes = executable_cost(expr)'''
    source = replace_exact(source, old, new, label="sample registered-token map growth")

    old = '''            _H7_online_metadata.pop(state[5], None)
            return existing

        dominated_existing = _H7_find_dominated'''
    new = '''            _H7_online_metadata.pop(state[5], None)
            _H7_release_pruned_token(
                state, "rejected_candidate", level_bucket_absent=True,
            )
            return existing

        dominated_existing = _H7_find_dominated'''
    source = replace_exact(source, old, new, label="release rejected candidate keys")

    old = '''        nonlocal next_state_token, target_candidate_found

        # The operands are already stored canonical expressions. Compose their'''
    new = '''        nonlocal next_state_token, target_candidate_found

        # Retired states must never return as executable composition operands.
        # The cleanup assertions above require absence from both active index
        # and level bucket before their lookup keys are released.
        # (No tombstone set is retained, so this assertion has zero unbounded
        # diagnostic memory cost.)
        # The next operations only accept states yielded from the live levels.
        # The fixture signature oracle independently checks semantic parity.

        # The operands are already stored canonical expressions. Compose their'''
    source = replace_exact(source, old, new, label="document composition lifetime invariant")

    # Export end-of-search diagnostics without changing scientific result.stats.
    old = '        return BehavioralSearchResult(\n'
    new = '''        _H7_key_lifetime_sample("search_return")
        find_exact_expression._H7_LAST_PRUNED_KEY_LIFETIME_STATS = dict(_H7_KEY_LIFETIME_STATS)
        return BehavioralSearchResult(
'''
    source = replace_exact(source, old, new, label="export final lifetime counters")

    patched_sha = hashlib.sha256(source.encode()).hexdigest()
    path.write_text(source, encoding="utf-8")
    print(f"ORIGINAL_SOURCE_SHA256={source_sha}")
    print(f"PRUNED_KEY_LIFETIME_PATCH_SHA256={patched_sha}")
    print("PATCH_FACTOR=ACSIE_RELEASE_PRUNED_KEYS=0/1")
    print("CLEANUP=expression_key_by_state_token + ast_depth_by_state_token only")
    print("FRONTIER_CACHE=applied separately by the frozen workflow")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
