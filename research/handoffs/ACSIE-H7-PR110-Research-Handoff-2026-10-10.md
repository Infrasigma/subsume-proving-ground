# ACSIE / PR110 / H7 Research Handoff
**As of:** 2026-10-10, approximately 08:26 UTC  
**Audience:** next research agent; proceed autonomously within the guardrails below  
**Disposition:** a real, bounded Gen6 engineering breakthrough; full PR110 qualification remains **INCONCLUSIVE / DO_NOT_PROMOTE**. **AGI=NOT_DEMONSTRATED; ASI=NOT_DEMONSTRATED; GENERAL_CAPABILITY_NOT_DEMONSTRATED.**

## 1. Mission and non-negotiable doctrine

The scientific question is whether ACSIE can acquire, retain, compose and improve increasingly general executable cognitive procedures **without** an external cognitive model, hidden target information, benchmark/task-family routing, manually supplied procedures for the target task family, or other leakage.

The project doctrine is **“Break the hypothesis, not the project.”** Keep independently supported mechanisms after a hypothesis fails. A bounded synthetic success is not AGI/ASI, nor evidence of open-ended self-extension by itself.

Hard constraints:
- No ChatGPT/OpenAI/Claude/Gemini/LLM controller in ACSIE runtime; no pretrained model, embeddings, external APIs, hidden prompts/strategy, manual task-family procedures, target identity leakage, task/benchmark ID routing, holdout/transfer/OOD selection leakage, threshold weakening or fabricated evidence.
- Research via GitHub/web/papers may be used as human/agent research input. Native runtime integrity must retain `target_access_during_discovery=false`.
- Do not weaken the frozen scientific gate. Qualification convention is 5 fixed seeds × 12 generations; one seed failing the gate blocks promotion. Diagnostics do not consume or replace that budget.
- Keep scientific and infrastructure/resource/software/integrity outcomes separate. `CI SUCCESS` is not science pass; unit/twin tests are not qualification.
- Every scientific claim must bind to ACSIE SHA, proving SHA, workflow/run/job, artifact ID/digest, stdout/stderr, manifest, split/gate config and integrity flags. Use scientific statuses `PASSED/FAILED/BLOCKED/CRASHED/INVALID/INCONCLUSIVE`.
- Do not merge, reset, force-push, or alter canonical main in order to make the experiment appear successful. Keep experimental runtime edits on isolated research branches until qualification succeeds.

## 2. Current state in one page

### Confirmed breakthrough

The lineage-memoized treatment of the pinned PR110 B runtime **completed Generation 6** from the exact previously preserved B Gen5 checkpoint. The unoptimized continuation of the same checkpoint timed out after the 330-minute ceiling without advancing beyond `generation_next=6`; two additional unoptimized 45-minute diagnostics also stopped in `primary_process_synthesis`.

- **Treatment run:** [38033213264](https://github.com/Infrasigma/subsume-proving-ground/actions/runs/38033213264)
- **Evidence artifact:** ID `11664044731`, ZIP SHA-256 `9db37c017bc06c62708451faf5da250de08d3bb706ed36a6f35a8dcc4bee5542`
- **Original checkpoint:** raw SHA-256 `f21654220651001f3ac63f67a983a516ff8ba236732bb58e47f611c41dd87ced`; input state digest `e284699d06649788b58071783651f087c57e0511ee10f93948b878d68f49e28a`; `generation_next=6`
- **Treatment result:** exit code 0; seven completed rows; durable checkpoint advanced to `generation_next=7`; state digest `cfe3391d48284e41416ef3241050b43f4be6df9fc6bfc04dc35f2c0726d3a37b`; 83 phase starts / 83 phase timings; no unmatched/open phase; one Gen6 progress record. Elapsed workflow continuation was about 74.8 minutes.
- **Gen6 local metrics:** accepted as `ACCEPTED_FRONTIER`; closure reuse 1.0; probe reuse 0.75; closure/probe success 1.0; lineage gate true; OOD error 0; transfer error 0; primitive found; retained count 7; resource cost 14; novelty about 0.07143.
- **Search cost remains high:** 121,218 generated expressions; 12,231 unique states; 112,458 target candidates; 6,641 target matches; maximum dominance frontier 6,145; 94,696 duplicate states; 28,478 dominance-pruned states. The selected search status was `TARGET_FOUND`.
- **Hot-path telemetry:** 15 executable-lineage graph walks; 9 provenance-lineage graph walks; 176,601,808 executable macro-set union-cache hits in Gen6 search statistics. Two timed primary-process-synthesis calls took about 1,530.12 sec and 1,466.05 sec (approximately 25.5 and 24.4 minutes each).

Interpretation: this is a decisive engineering result for the Gen6 stall, not proof of a complete open-ended trajectory. A diagnostic continuation under the pinned B runtime found and accepted one Gen6 primitive while retaining the observed reuse/lineage conditions. It has not yet proven Gen7–Gen11, all 12 generations, the paired A/B causal comparison, or the frozen 5×12 gate.

### Equivalence and timing result

The reference-vs-memoized twin test now uses a deterministic **32-node executable-parent chain** and 12 fixed discovery rows, running five timed repeats per arm. Latest successful run: [38033935977](https://github.com/Infrasigma/subsume-proving-ground/actions/runs/38033935977); artifact ID `11663254166`, ZIP digest `sha256:4430277de17aea414911ae40b1ef2946dce0dfb995c32b4481ebce43a5a350a9`.

For both A and B, selected expression, directly used primitive, alternatives and every non-cache search statistic matched between uncached and memoized runs.
- **A:** median 0.091364 sec → 0.070083 sec, 1.304× speedup (~23.3% lower median time). 32 primitive lineage graph walks and 146 macro-set union-cache hits in the timed fixture.
- **B:** median 65.678514 sec → 18.370333 sec, 3.575× speedup (~72.0% lower median time). 32 executable-lineage graph walks; 992 executable primitive-cache hits; 511,900 executable macro-set union-cache hits; two provenance primitive graph walks.
- Timing includes Python process startup, but is a synthetic benchmark. This is not a full Gen6 wall-time ratio. The exact checkpoint continuation above is the stronger real-runtime evidence.

See also:
- Earlier 8-node twin: [38033528970](https://github.com/Infrasigma/subsume-proving-ground/actions/runs/38033528970), artifact ID `11662843002`.
- Latest manifest on the experiment branch: `research/pr110_acsie_snapshot/lineage-memoization-manifest.json`.
- Test: `lab/pr110_lineage_memoization_equivalence.py`.

### Still open / don't overclaim

- PR110’s overall status is still `INCONCLUSIVE / DO_NOT_PROMOTE`.
- This run is a controlled checkpoint continuation; original checkpoint-creating process did not pin `PYTHONHASHSEED`, while continuation explicitly pins it to 0. It is not a byte-identical full-process replay.
- The frozen 5×12 scientific budget is untouched.
- Gen6 is one seed and one generation. Do not call it AGI, ASI, open-ended self-extension, or full PR110 success.
- The lineage-cache workflow accidentally launched a second identical run while its trigger was being changed: [38033262358](https://github.com/Infrasigma/subsume-proving-ground/actions/runs/38033262358). At last live inspection (about 08:26 UTC) it remained in progress. It is a duplicate, **not** an independent replication. The workflow trigger is now manual-only. Recheck its state; avoid further duplicate compute if cancellation is available.

## 3. Canonical repositories, refs and PRs

### Canonical main refs (last verified unchanged)
- ACSIE main: `9e12988c576b7fa16909b527edff6b677f549fe5`
- Proving-ground main: `c105bb42661e0ea7c53752996f386119e4392600`

### Active PRs
- [ACSIE PR #110](https://github.com/Infrasigma/ACSIE/pull/110), title “research: preserve explicit direct reuse against composite dominance”; open, draft, not merged; head `04f959e06a813aeba1c2c7a74f5841333354b27a`, base ACSIE main.
- [Proving-ground PR #266](https://github.com/Infrasigma/subsume-proving-ground/pull/266), title “research: execute PR110 A/B from pinned ACSIE snapshots”; open, draft, not merged; head `c0db56983a92807524280b652dadced7fa07bd84`, base proving-ground main.
- Current instrumentation/memoization work is on isolated branch `research/pr110-gen6-phase-timing-20261010`; no runtime patch has been merged into main.

### Pinned PR110 A/B snapshots and checkpoint
- A runtime SHA: `7e905d73c15ece4b8b5d7cb73218bac34c2e1314`
- B runtime SHA: `af0c7ef503c51a5bc7872717b263a01f495c855f`
- Proving/evaluator SHA: `33259fc5c69d87b99c604f0050508c433a279daf`
- Fixed seed in the checkpoint diagnostic: `2026100305`
- Original preserved B input checkpoint from workflow run `37570567635`, digest/metadata are recorded in the manifest above.
- Original B lab `behavioral_search.py` blob before memoization: `90616f062298b52eb72b95cffeeb754c626ec4ed`
- Memoized treatment lab blob: `ab658a2e49adf479bf9ea4892dfef930624c6e7b`
- A lab blob before memoization: `6145789aaf4e6a4c4d1cf78ec93e6f94b1df2411`
- Memoized A lab blob: `a159f394afce54b8a95e507ed0e2f2e632b69b5d`

The lab snapshots retain a deterministic telemetry adjustment (`list(set(...))` → `sorted(set(...))`) and add per-search-call lineage memoization. Upstream source identities are still recorded separately in `research/pr110_acsie_snapshot/manifest.json`.

## 4. PR110 hypothesis and what was learned

### Main PR110 hypothesis

The runtime may conflate **provenance lineage** with actual **executable macro lineage**. Behaviorally equivalent representatives may dominate or displace a direct executable macro representation, causing recursive composition to lose explicit executable vocabulary. Desired dominance should preserve, rather than conflate:
- direct executable-macro supersets,
- transitive executable-lineage supersets,
- expanded executable depth no worse,
- expanded executable node count no worse.

Core distinction: `equivalence ≠ dominance`; a provenance alias is not an executable macro dependency. Do not treat the fact that two expressions behave alike on discovery rows as proof they are interchangeable for future recursive composition.

### New working hypothesis: lineage computation is a major part of the Gen6 cost

Phase-localization established that the unoptimized continuation repeatedly stopped in `primary_process_synthesis`:
- [Unoptimized 45-minute replay 1](https://github.com/Infrasigma/subsume-proving-ground/actions/runs/38032555973), artifact ID `11664255561`
- [Unoptimized 45-minute replay 2](https://github.com/Infrasigma/subsume-proving-ground/actions/runs/38032579393), artifact ID `11664030795`
- Each still had `generation_next=6`, no Gen6 progress, 69 phase starts / 68 timings, and the final unmatched phase was `primary_process_synthesis`, `probe_index=1`. Their last durable checkpoint digest remained the original `e284699d...`.
- The original unoptimized exact continuation [run 37989963926](https://github.com/Infrasigma/subsume-proving-ground/actions/runs/37989963926) timed out after 330 minutes, exit 124, with `generation_next=6`.

The memoization treatment caches the result of primitive lineage graph walks and repeated macro-set unions within a single `find_exact_expression` call. It does not alter expression generation, ranking, dominance, acceptance, lineage gates, or target access. Twin checks show the same expression/alternatives and all non-cache search stats on synthetic workloads. Its strongest evidence is the exact checkpoint continuation moving from Gen5 boundary to completed Gen6.

However, 176.6 million macro-set union cache hits indicate that the treatment still invokes the memoized lookup path an enormous number of times. The cache removes repeated graph traversal, but key building/hashing, semantic candidate generation, and very large frontiers remain candidates for the next bottleneck. Two primary process synth phases each took about 25 minutes even in the treatment.

## 5. Compression hypothesis: H7 — current separate line, and failures

Current H7 hypothesis: `REDUNDANT_EXECUTABLE_REPRESENTATIVE_COMPRESSION` — reduce redundant executable representatives while preserving enough executable lineage for recursive reuse. Keep it separate from the memoization hypothesis and from H3's alternate search width work.

Prior Phase 6 forensic conclusion:
- `SCALABILITY_CAUSE=LINEAGE_PRESERVATION_EXPLOSION`.
- At the B Gen5→Gen6 boundary, the lineage-preserving frontier was vastly larger than A (reported frontier max 1,548 vs 6 for the earlier comparison; later H7 Gen6 audit reached 6,145).
- Behavioural equivalence classes were close; ignoring lineage could remove roughly 88–89% of representatives, but their lineage overlap Jaccard was only about 0.58–0.60.
- A previous timing observer was faulty, so lineage timing was initially undetermined; cache hypothesis was not justified at that time. The current phase instrumentation plus new exact continuation now supports memoization as a useful optimization.

Separate H7 runtime evidence:
- The alternate repaired H7 runtime SHA `9b0c42eb4394aee457f8b7adecc45a3f4ea7c4be` completed a bounded Gen6 unbounded-control path: recursive reuse true, lineage true, closure reuse 1.0, probe reuse 0.75, 15 lineage IDs represented and 0 lost, 121,218 generated expressions, max frontier 6,145. This was **not** the exact PR110 A/B causal result.
- Online K=2 and K=4 representative caps reduced cost but failed the recursive/lineage gates despite finding a target. Therefore blunt hard-cap compression is rejected.
- Offline counterfactual tried K=1/2/4/8 and unbounded representative sets using greedy marginal executable-lineage coverage inside behavioral-equivalence classes. K=2/K=4 can lose lineage and must not be promoted. A K setting alone is not a safe compression rule.
- H3 alternate-width 8→16→32 experiments are a separate line. Do not combine H3, H7 compression and lineage cache in one causal test.

Safe future H7 direction is not “pick fewer candidates regardless of what is lost.” Any candidate compression must preserve the executable-lineage coverage relevant to future composition, preserve the lineage antichain needed for recursive reuse, keep deterministic ordering, and pass the original no-loss/no-routing/holdout gates. Start with proof-oriented offline filtering and diagnostics; only then add online policy.

## 6. Earlier accepted mechanisms and historical failures

These results are real bounded mechanisms; they do not establish AGI/ASI.

### Native integrity — H41
- Source SHA: `aedf4e2fa4adfe174eebf5d63133a14d85c4f753`
- Workflow: `36426158534`; artifact: `10971688189`; artifact SHA-256: `2351793583596051817bb69282cba117247ee954075bcebcf78785ba79fa8542`
- 5/5 integrity seeds passed, including no holdout contamination, task routing, external model, network dependency, or manually supplied strategy. Maintain these guarantees in future work.

### Recursive and semantic compiler mechanisms
- CORE-013 H003: 20/20 first-order eligibility; 20/20 second-order/meta eligibility; 20/20 warm-minus-cold.
- CORE-013 H004: 20/20 construction, held-out, fresh-process persistence and deployment.
- CORE-014: recursive cognitive compiler.
- CORE-015: semantic operator discovery.
- CORE-016: stateful operator-class discovery. Important caveat: `stateful_policy_router` remains a bounded human-authored interpreter primitive, not autonomous discovery of arbitrary policies.

### L2-C acquisition/retention work
- PR #47 merged at `22549d316d2d6aff504d6b0929741161ff5228e`: fixed `[:128]` candidate truncation that excluded `eq(sign(y), sign(z))` (candidate index 144).
- PR #51 H-R12 `deea26284d40e82a098e27ee14ad178a8c366f03`: 5/5; p5–p7 acquisition; holdout/transfer 1.0; zero newcomer intrusion/route changes.
- PR #53: early quarantine, exclude named newcomer before `_candidates()`; 5/5.
- PR #56: quotient Boolean complements and discovery-only cross-validation.
- Capstone `fe20763d01f286f6f84f83cfa9b037e27ce7ac7d`, workflow `36603541682`, artifact `11052198496`, digest `378a...4416b`: frontier penetration true; sequential retention true; integrity okay; external-model flag false; but candidate acquisition false. Semantic-partition-equivalent acquisition was demonstrated; exact canonical AST acquisition was not. V3 lesson: newcomer non-intrusion does not prove incumbent invariance. `update_const(0.2354)` inner STGP gate was false.

### Recursive improvement and capability-demand failures
- I1–I5: 10/10 PASS; head `a944730...`, workflow `37022133050`, artifact `11233534125`; mean regret improvement 0.209375. Recursive improvement accepted; capability-demand/I2/I3/quarantine/ontology still provisional.
- H6-R1 failed: `model_better=false`, p approximately 0.88–0.98.
- H9-C and H9-D failed: unseen-selection result about 0.81–0.97.
- H9-E was deprioritized; later H9-E capability-demand test PR #86 (`9b80411c1df70badb14bfeda38a151b640f76531`) and proving-ground issue #154 failed the frozen five-seed gate: active mean beat random on average, but active mean was worse than historical on all five seeds; 3/5 p-values ≤ .05 did not satisfy the required superiority condition. Do not merge/promote it. H9-F is a future hypothesis only.

## 7. Failed hypotheses and lessons

1. **“More target matching/candidate acquisition automatically means capability.”** Failed in the L2-C capstone: frontier penetration did not imply successful canonical acquisition. Keep exact acquisition claims distinct from partition equivalence.
2. **“Newcomer non-intrusion proves incumbent invariance.”** False. Verify incumbents and routes separately.
3. **H6-R1 model-better signal.** Failed by the p-values/criterion; don't rehabilitate with a weaker threshold.
4. **H9-C/D unseen-selection.** Failed; the unseen-selection scores do not meet the gate.
5. **H9-E capability demand.** Failed the fixed seed-wise superiority condition despite some significant p-values. All seeds must satisfy the preregistered decision rule.
6. **PR110 exact canonical candidate acquisition.** Not shown by prior semantic-partition success.
7. **H7 blunt K=2/K=4 hard cap.** Failed lineage/recursive-reuse gates. Lower runtime cost is not success if executable lineage is lost.
8. **Unoptimized PR110 B Gen5→Gen6 runtime.** Failed as a run after 330-minute timeout and repeated 45-minute stalls. The phase is now localized to `primary_process_synthesis`.
9. **Lineage memoization fully solves scale.** Not yet demonstrated. It makes Gen6 complete, but residual synthesis takes tens of minutes and the exact continuation has not yet completed Gen7–11 or a full 12-generation paired replay.
10. **Treat green workflows as scientific pass.** Never valid. The green continuation is a successful diagnostic engineering run only.

## 8. Exact next steps for the receiving agent

### Step A — preserve, verify, and summarize the Gen6 breakthrough
1. Recheck current state of duplicate run [38033262358](https://github.com/Infrasigma/subsume-proving-ground/actions/runs/38033262358); do not count it as an independent replication. The manual-only trigger is already in place.
2. Download artifact `11664044731` from run `38033213264`. Validate input checkpoint SHA, state digest, treatment blob identity, output checkpoint state digest `cfe3391d...`, `generation_next=7`, and the Gen6 event fields above. Preserve raw stdout/stderr and exact artifact digest. The GitHub Actions job logs contain the emitted `memoized-diagnostic.json` summary if artifact extraction is temporarily blocked.
3. Recheck current main refs and PR states. Both canonical mains must stay unchanged.

### Step B — do a controlled continuation, not another blind 330-minute replay
1. Use the immutable completed Gen6 treatment checkpoint from the artifact and the exact memoized B lab blob `ab658a2e49adf479bf9ea4892dfef930624c6e7b`; keep evaluator/proving SHA `33259fc5c69d87b99c604f0050508c433a279daf`, seed `2026100305`, `PYTHONHASHSEED=0`, and explicit checkpoint metadata.
2. Continue a single next generation (Gen7, i.e. expected checkpoint boundary `generation_next=8`) with a hard time limit and per-phase timers. Do not silently extend the budget or skip gates. If it stalls, archive last matched/unmatched phases, cache counters, frontier/search counters, memory, checkpoint digest, stdout/stderr.
3. Inspect the 330-minute baseline raw artifact alongside memoized Gen6. The exact comparison is that unoptimized continuation exited 124 at `generation_next=6`, while the memoized checkpoint continuation exited 0 at `generation_next=7` in about 74.8 min. This is strong single-run evidence, not a multi-run performance distribution.
4. Do not infer full-trajectory win from Gen6. Assess subsequent generations, including whether closure/probe reuse, lineage closure, retention and acceptance remain valid.

### Step C — finish the real causal PR110 question
1. Run the paired A/B causal replay with all identities explicit and comparable; use unchanged A as reference and the memoized B treatment only. Keep the deterministic telemetry patch isolated and recorded. The checkpoint process hash-seed caveat must be addressed in a fresh controlled replay if exact reproducibility is required.
2. Collect selected generation evidence for 5, 6, 9, 10 and 11 and all trajectory/final-gate fields. Existing A historical metrics show later traps: Gen10 closure reuse 0.9818 with closure/probe trap counts 1/1; Gen11 closure reuse 0.9545 with closure/probe traps 3/1. A final `finite_open_ended_growth_gate=false`, closure reuse mean ~0.9545, probe reuse ~0.5. The treatment must be assessed against that complete picture, not just Gen6.
3. If the single-seed continuation looks good, do not immediately call PR110 qualified. Preserve the frozen 5-seed × 12-generation gate and run it without changing thresholds, seed list, holdout, routing or evaluator. One seed failure blocks promotion.
4. Record `PASSED/FAILED/BLOCKED/CRASHED/INVALID/INCONCLUSIVE` separately for scientific gate and for infrastructure/runtime.

### Step D — reduce the next cost source only after profiling
1. Gen6 telemetry shows 176.6M executable macro-set union-cache hits and two process synth calls over 24 minutes each. Instrument cost per phase, cache entries/bytes, time building/sorting macro-set keys, candidate count, frontier distribution and duplicate/unique state counts. Do not presume graph traversal is the only residual bottleneck.
2. Consider cache-key interning/bitsets or safely reusing already-canonical macro-set keys only as a new hypothesis. Prove outputs, order, alternatives, search statistics and cache integrity match before/after. Benchmark on fixed synthetic and checkpoint workloads; keep code and results separate from H7 compression.
3. Keep H7 representative compression separate. Do not use K=2/K=4 hard caps. Any adaptive compression must preserve the executable-lineage antichain/coverage required by future compositions and show zero lost required lineage under held-out/fresh-process tests before online use.
4. Do not mix H3 width (8→16→32), H7 compression and memoization in a single experiment.

### Step E — prerequisites for any eventual ASI argument
Even a successful 5×12 result would still be bounded synthetic evidence. To justify increasingly broad intelligence claims the project must demonstrate, with strict leakage controls:
- autonomous acquisition of novel executable procedures (not only semantic partitioning or supplied macros),
- durable retention across fresh process/deployment and adversarial/task shift,
- compositional transfer to genuinely new task families and OOD settings,
- recursive improvement without expert target-family procedures or external model/controller,
- reliable calibration and capability demand that beats the required baselines on each preregistered seed,
- scalable resource use and reproducibility across independent runs,
- full native runtime integrity/no hidden target access/no routing/no external model,
- a claim ledger and evidence bundle supporting every stage, with no conflation of bounded competence, AGI or ASI.

There is no valid shortcut from one successful Gen6 continuation to ASI.

## 9. Current hypothesis ledger

| Hypothesis / line | Current classification | Evidence / next decision |
|---|---|---|
| Lineage graph-walk memoization is semantics-neutral on the tested search | **PASSED on deterministic synthetic twin tests** | Same expression, primitive, alternatives and non-cache search stats; 32-node fixture, 5 repeats. |
| Memoization materially unblocks PR110 Gen6 | **SUPPORTED by one exact-checkpoint treatment continuation** | Gen6 advanced to `generation_next=7` in ~74.8 min; original 330-min continuation timed out at 6. Replicate/continue under controlled paired seed. |
| Full PR110 treatment improves open-ended trajectory | **INCONCLUSIVE** | Need Gen7–11, selected generation metrics and full paired replay; original budget unmodified. |
| H7 blunt representative cap K=2/K=4 | **FAILED** | Lineage/recursive-reuse gates fail despite target found. |
| H7 lineage-preserving adaptive representative compression | **OPEN; unproven** | Design offline, show no lineage loss and preserve recursive gates before any online trial. |
| H3 alternative widths 8/16/32 | **SEPARATE / NOT MIXED** | Do not confound with H7 or cache experiment. |
| H6-R1 / H9-C / H9-D / H9-E | **FAILED** | Preserve failed-gate evidence; do not lower thresholds. |
| AGI / ASI / general capability | **NOT DEMONSTRATED** | Requires far broader autonomous acquisition, retention, composition, transfer and recursive improvement evidence. |

## 10. Final handoff directive

The breakthrough is specific and important: **memoization let the exact PR110 B checkpoint finish Gen6 where the uncached continuation repeatedly stalled in primary process synthesis; the 32-node equivalence benchmark also preserves outputs while improving B median time 3.575×.** The next agent should verify and preserve that evidence, progress one bounded generation at a time with phase telemetry, and then finish the paired PR110 question under frozen gates. Do not merge, promote, relax gates, label the result AGI/ASI, or spend the full qualification budget on a hypothesis that has not first passed the controlled continuation.

Primary evidence:
- Exact B Gen6 treatment: https://github.com/Infrasigma/subsume-proving-ground/actions/runs/38033213264
- Unoptimized exact checkpoint timeout (330 min): https://github.com/Infrasigma/subsume-proving-ground/actions/runs/37989963926
- Unoptimized phase-localization controls: https://github.com/Infrasigma/subsume-proving-ground/actions/runs/38032555973 and https://github.com/Infrasigma/subsume-proving-ground/actions/runs/38032579393
- 32-node twin benchmark: https://github.com/Infrasigma/subsume-proving-ground/actions/runs/38033935977
- Memoization manifest: https://github.com/Infrasigma/subsume-proving-ground/blob/research/pr110-gen6-phase-timing-20261010/research/pr110_acsie_snapshot/lineage-memoization-manifest.json
- ACSIE PR #110: https://github.com/Infrasigma/ACSIE/pull/110
- Proving-ground PR #266: https://github.com/Infrasigma/subsume-proving-ground/pull/266
