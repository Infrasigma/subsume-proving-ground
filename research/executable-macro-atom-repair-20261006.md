# Executable macro-atom repair experiment

ACSIE repair PR: #109
ACSIE SHA: 259e5dfd041e638c0fa2ff2146b615507f2874ea
Parent runtime SHA: 7e905d73c15ece4b8b5d7cb73218bac34c2e1314

Hypothesis: discovery-row behavioral dominance can erase retained depth-1 executable macro atoms, even though they remain part of the future learned search vocabulary. The repair excludes depth-1 macro atoms from dominance pruning while leaving composed-state dominance unchanged.

Scientific gate remains frozen at 5 seeds x 12 generations with unchanged acceptance, closure, probe, growth, depth, lineage, and integrity criteria.

This file is provenance-only and has no runtime effect.
