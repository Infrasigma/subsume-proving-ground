from __future__ import annotations

import hashlib
import json
import unittest

from lab.pr110_gen6_lineage_identifiability_report import summarize


def make_event(*, status="EMPIRICALLY_AMBIGUOUS", used=False, changed=False, wrong_count=1):
    candidates = [
        {
            "closure_competent_on_declared_validation": True,
            "contains_expected_executable_parents": False,
            "exact_target_match_on_fresh_inputs": True,
            "max_absolute_error": 0.0,
            "mean_absolute_error": 0.0,
            "process_id": "proc:wrong-lineage",
        },
        {
            "closure_competent_on_declared_validation": True,
            "contains_expected_executable_parents": True,
            "exact_target_match_on_fresh_inputs": True,
            "max_absolute_error": 0.0,
            "mean_absolute_error": 0.0,
            "process_id": "proc:correct-lineage",
        },
    ]
    audit = {
        "schema": "ACSIE.lineage-identifiability-audit.v1",
        "status": status,
        "sample_count": 2000,
        "candidate_count": len(candidates),
        "competent_candidate_count": 2,
        "competent_wrong_lineage_exact_candidate_count": wrong_count,
        "exact_expected_lineage_candidate_count": 1,
        "input_design_sha256": "a" * 64,
        "target_output_sha256": "b" * 64,
        "used_for_selection_or_admission": used,
        "runtime_selection_changed": changed,
    }
    return {
        "event": "CLOSURE_LINEAGE_MISMATCH",
        "seed": 2026100305,
        "generation": 6,
        "expected_parent_ids": ["prim:a", "prim:b"],
        "selected_process_id": "proc:wrong-lineage",
        "selected_candidate_reuse_ok": False,
        "lineage_identifiability_audit": {**audit, "candidate_results": candidates},
    }


class LineageIdentifiabilityReportTests(unittest.TestCase):
    def test_ambiguous_audits_are_aggregated_without_formal_equivalence_claim(self):
        raw = "\n".join(json.dumps(make_event()) for _ in range(2)) + "\n"
        report = summarize(raw, source_sha256=hashlib.sha256(raw.encode()).hexdigest())
        self.assertEqual(report["finding"], "EMPIRICAL_NON_IDENTIFIABILITY_OBSERVED")
        self.assertEqual(report["event_count"], 2)
        self.assertEqual(report["candidate_instances_across_audit_rows"], 4)
        self.assertEqual(report["competent_wrong_lineage_exact_candidate_instances_recorded_sum"], 2)
        self.assertEqual(report["competent_wrong_lineage_exact_candidate_instances_recomputed_sum"], 2)
        self.assertEqual(report["distinct_wrong_lineage_exact_process_ids_seen"], 1)
        self.assertFalse(report["formal_equivalence_claim"])
        self.assertTrue(report["selection_invariance_verified"])

    def test_changed_selection_flag_is_not_read_only(self):
        report = summarize(json.dumps(make_event(changed=True)), source_sha256="deadbeef")
        self.assertFalse(report["selection_invariance_verified"])
        self.assertIsNotNone(report["selection_invariance_warning"])

    def test_nonambiguous_row_does_not_claim_universal_non_identifiability(self):
        report = summarize(json.dumps(make_event(status="NO_AMBIGUITY_OBSERVED")), source_sha256="deadbeef")
        self.assertNotEqual(report["finding"], "EMPIRICAL_NON_IDENTIFIABILITY_OBSERVED")

    def test_missing_audit_is_inconclusive(self):
        report = summarize(json.dumps({"event": "CLOSURE_LINEAGE_MISMATCH", "seed": 1, "generation": 6}), source_sha256="deadbeef")
        self.assertEqual(report["parse_status"], "INCOMPLETE_IDENTIFIABILITY_AUDIT")
        self.assertEqual(report["finding"], "INCONCLUSIVE_INCOMPLETE_EVIDENCE")

    def test_malformed_lines_are_reported(self):
        raw = "not-json\n" + json.dumps(make_event())
        report = summarize(raw, source_sha256="deadbeef")
        self.assertEqual(report["malformed_or_non_object_jsonl_lines"], 1)
        self.assertEqual(report["parse_status"], "RAW_LOG_HAS_MALFORMED_LINES")


if __name__ == "__main__":
    unittest.main()
