"""CPU-first model-independent cognitive substrate."""
from .general_engine import GeneralLearner, Skill, CognitiveState, Bottleneck
__all__ = ["GeneralLearner","Skill","CognitiveState","Bottleneck"]
from .meta_language import GenericOperator, MetaLanguageError
from .general_engine import CapabilityGap, OperatorArtifact, OperatorUtility, StrategyPatch, LanguageVersion, GrowthRecord
__all__ += ["GenericOperator","MetaLanguageError","CapabilityGap","OperatorArtifact","OperatorUtility","StrategyPatch","LanguageVersion","GrowthRecord"]
from . import open_language as _core005_open_language
from . import self_improvement as _core006_self_improvement
from .self_improvement import ArchitectureSpec, SelfModel, ImprovementHypothesis, ArchitecturalPatch, ImprovementRecord
__all__ += ["ArchitectureSpec", "SelfModel", "ImprovementHypothesis", "ArchitecturalPatch", "ImprovementRecord"]
# ASI-CORE-009: generic open-capability acquisition, diagnosis, active evidence and retention.
from . import open_capability as _core009_open_capability
# ASI-CORE-010: generic action-conditioned interaction learning and planning.
from . import interactive_learning as _core010_interactive_learning
# ASI-CORE-011: generic lifelong memory, context discovery, consolidation and retrieval.
from . import continual_learning as _core011_continual_learning
from . import continual_learning as _core011_continual_learning

# CORE-013: substrate-owned executable cognitive-process evolution.
from . import cognitive_evolution as _core013_cognitive_evolution
from .cognitive_evolution import CognitiveProcedure, ProcedureDecision
__all__ += ["CognitiveProcedure", "ProcedureDecision"]

# CORE-014: recursive cognitive compiler — persistent process language, experiments, and transfer gating.
from .recursive_cognitive_compiler import RecursiveCognitiveCompiler, CognitivePrimitive, ProcessCandidate, Evaluation, Trace
__all__ += ["RecursiveCognitiveCompiler", "CognitivePrimitive", "ProcessCandidate", "Evaluation", "Trace"]

# CORE-015: autonomous typed multi-effect semantic operator discovery and installation.
from . import semantic_operator_discovery as _core015_semantic_operator_discovery
from .semantic_operator_discovery import SemanticOperator, SemanticOperatorDiscovery, SemanticOperatorLanguage, Effect
__all__ += ["SemanticOperator", "SemanticOperatorDiscovery", "SemanticOperatorLanguage", "Effect"]


# CORE-016: trace-induced open semantic operator classes.
from . import open_semantic_substrate as _core016_open_semantic_substrate
from .open_semantic_substrate import (
    OpenSemanticSubstrate,
    OperatorClassSpec,
    OperatorExperience,
    StatefulHistoryRouter,
    apply_discovered_class_to_learner,
)
__all__ += [
    "OpenSemanticSubstrate",
    "OperatorClassSpec",
    "OperatorExperience",
    "StatefulHistoryRouter",
    "apply_discovered_class_to_learner",
]

# Native independent cognitive substrate.
from .native_independent_core import NativeCognitiveCore, LearningPolicy, LearningKernel, ChangeDecision
__all__ += ["NativeCognitiveCore", "LearningPolicy", "LearningKernel", "ChangeDecision"]

# CORE-017: failure-directed capability repair and compositional program expansion.
from .adaptive_capability_repair import (
    FailureDiagnosis, RepairAttempt, RepairResult, RepairDrivenCognitiveCore,
    diagnose_failure, synthesize_extended_program, apply_extended_program,
)
__all__ += [
    "FailureDiagnosis", "RepairAttempt", "RepairResult", "RepairDrivenCognitiveCore",
    "diagnose_failure", "synthesize_extended_program", "apply_extended_program",
]
