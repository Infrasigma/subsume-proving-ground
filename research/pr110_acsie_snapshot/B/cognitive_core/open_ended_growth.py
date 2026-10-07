    def invent_primitive(
        self,
        train: Sequence,
        holdout: Sequence,
        transfer: Sequence,
        semantics: str = "delta",
    ) -> CognitivePrimitive | None:
        if len(train) < 6:
            return None

        discovery_rows = tuple(train)
        max_depth = max(3, int(self.meta_policy["max_depth"]) + 1)
        result = find_exact_expression(
            self,
            discovery_rows,
            max_depth,
            require_macro=bool(self.generation > 0 and self.primitives),
            max_alternatives=32,
        )
        if result is None:
            self.last_primitive_candidate_frontier = ()
            return None

        def error(rows: Sequence, expression: Mapping[str, object]) -> float:
            if not rows:
                return float("inf")
            vals = []
            pmap = self._pmap()
            for trace in rows:
                try:
                    vals.append(abs(eval_expr(expression, trace.inputs, pmap) - trace.target))
                except Exception:
                    return float("inf")
            return sum(vals) / len(vals)

        specs = list(result.alternatives) or [(result.expression, result.used_primitives)]
        frontier: list[CognitivePrimitive] = []
        for expression, used in specs[:32]:
            pid = "prim:" + digest((expression, semantics))[:20]
            if pid in self.primitives:
                continue
            primitive = CognitivePrimitive(
                pid,
                copy.deepcopy(expression),
                tuple(sorted(set().union(*(t.inputs.keys() for t in discovery_rows)))),
                semantics,
                self.generation,
                tuple(used),
                tuple(sorted({t.family for t in list(train) + list(transfer)})),
                error(train, expression),
                error(holdout, expression),
                error(transfer, expression),
                ast_nodes(expression),
                {
                    "origin": "open_ended_behavioral_dp_search",
                    "schema_given": False,
                    "external_model": False,
                    "selection_source": "discovery_and_selection_only",
                    "ood_used_for_selection": False,
                    "validation_evaluated_after_selection": True,
                    "validation_role": "admission_only",
                    "candidate_frontier_size": min(32, len(specs)),
                    "direct_parent_ids": list(used),
                    "search_stats": dict(result.stats),
                },
            )
            self.primitives[pid] = primitive
            self.archive[pid] = ArchiveRecord(primitive)
            frontier.append(primitive)

        self.last_search_stats = dict(result.stats)
        self.last_primitive_candidate_frontier = tuple(frontier)
        return frontier[0] if frontier else None

    def accept_primitive_frontier(
        self,
        *,
        transfer_rows: Sequence,
        ood_rows: Sequence,
        novelty: float,
        resource_cost: float,
    ) -> dict[str, object]:
        frontier = tuple(self.last_primitive_candidate_frontier)
        if not frontier:
            return {"accepted": False, "primary_primitive": None, "candidate_count": 0, "validated_candidate_count": 0}

        def error(rows: Sequence, expression: Mapping[str, object]) -> float:
            if not rows:
                return float("inf")
            vals = []
            pmap = self._pmap()