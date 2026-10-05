from __future__ import annotations

import tempfile
from pathlib import Path

from lab.acsie_open_ended_self_extending_proving import run_seed


def test_exact_checkpoint_roundtrip_preserves_future_trajectory():
    metadata = {"proving_sha": "checkpoint-test", "acsie_ref": "checkpoint-test"}

    with tempfile.TemporaryDirectory() as tmp:
        checkpoint = Path(tmp) / "state.json"

        uninterrupted = run_seed(
            101,
            4,
            checkpoint_metadata=metadata,
        )

        first_segment = run_seed(
            101,
            4,
            checkpoint_path=str(checkpoint),
            stop_after=2,
            checkpoint_metadata=metadata,
        )
        assert first_segment["completed_generations"] == 2
        assert checkpoint.exists()
        assert not first_segment["finite_open_ended_growth_gate"]

        resumed = run_seed(
            101,
            4,
            checkpoint_path=str(checkpoint),
            resume_from=str(checkpoint),
            checkpoint_metadata=metadata,
        )

        assert resumed["completed_generations"] == 4
        assert resumed["generations_detail"] == uninterrupted["generations_detail"]
        assert resumed["retained_count"] == uninterrupted["retained_count"]
        assert resumed["final_runtime_max_depth"] == uninterrupted["final_runtime_max_depth"]
        assert resumed["final_target_depth"] == uninterrupted["final_target_depth"]
        assert resumed["max_generated_expressions"] == uninterrupted["max_generated_expressions"]
        assert resumed["max_unique_search_states"] == uninterrupted["max_unique_search_states"]

# trigger: exact-checkpoint-roundtrip validation
