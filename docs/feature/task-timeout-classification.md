# Task timeout classification fix

## F0 — Intake and gates

- Branch: `codex/task-timeout-classification`; base: `deda642b`, clean and synchronized with origin/main on 2026-10-15.
- Workflow: P10 feedback → P8 backend correction → P9 verification. User authorized fixing the diagnosed bug and merging main.
- Existing evidence: customer screenshot and local handler analysis; customer deployed revision and raw error remain unknown.
- Workflow gate: implementation allowed. No UI/API/schema redesign required; required upstream contract is the existing task deadline and user-approved correction.
- Maintainability gate: narrow_fix; handlers are 318–673 lines, mix orchestration and persistence, moderate coupling. No broad refactor needed. Extract one small timeout predicate and cover context semantics and persisted batch outcome.
- Scope: single-agent, multi-agent, workflow, and batch task deadline classification. Unrelated robot request handling remains outside scope.
