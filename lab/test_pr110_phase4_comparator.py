import copy
import json
import unittest
from pathlib import Path

from lab.pr110_phase4_comparator import compare_checkpoints


ROOT = Path(__file__).resolve().parents[1] / "evidence" / "phase4_input"
OLD = ROOT / "old-gen6.checkpoint.json"
REPAIRED = ROOT / "repaired-gen6.checkpoint.json"


@unittest.skipUnless(OLD.exists() and REPAIRED.exists(), "formal artifact not materialized")
class PR110Phase4ComparatorRegression(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.old = json.loads(OLD.read_text(encoding="utf-8"))
        cls.repaired = json.loads(REPAIRED.read_text(encoding="utf-8"))

    def assert_baseline_passes(self):
        result = compare_checkpoints(self.old, self.repaired)
        self.assertTrue(result["scientifically_neutral_at_boundary"], result)

    def test_allowed_runtime_and_lineage_order_pass(self):
        self.assert_baseline_passes()

    def test_retained_capability_mutation_fails(self):
        mutated = copy.deepcopy(self.repaired)
        mutated["retained"][0]["primitive_id"] += ":MUTATED"
        result = compare_checkpoints(self.old, mutated)
        self.assertFalse(result["scientifically_neutral_at_boundary"])

    def test_target_digest_mutation_fails(self):
        mutated = copy.deepcopy(self.repaired)
        mutated["generations_out"][-1]["target_digest"] = "deadbeef"
        result = compare_checkpoints(self.old, mutated)
        self.assertFalse(result["scientifically_neutral_at_boundary"])

    def test_candidate_identity_mutation_fails(self):
        mutated = copy.deepcopy(self.repaired)
        processes = mutated["learner_state"]["processes"]
        process_id = next(iter(processes))
        process = processes.pop(process_id)
        process["process_id"] = process_id + ":MUTATED"
        processes[process["process_id"]] = process
        result = compare_checkpoints(self.old, mutated)
        self.assertFalse(result["scientifically_neutral_at_boundary"])

    def test_reuse_mutation_fails(self):
        mutated = copy.deepcopy(self.repaired)
        mutated["generations_out"][-1]["probe_reuse_rate"] = 0.5
        result = compare_checkpoints(self.old, mutated)
        self.assertFalse(result["scientifically_neutral_at_boundary"])

    def test_lineage_membership_mutation_fails(self):
        mutated = copy.deepcopy(self.repaired)
        search_stats = mutated["generations_out"][-1]["search_stats"]
        ids = search_stats["selected_structural_lineage_ids"]
        ids[0] = ids[0] + ":MUTATED"
        result = compare_checkpoints(self.old, mutated)
        self.assertFalse(result["scientifically_neutral_at_boundary"])


if __name__ == "__main__":
    unittest.main()
