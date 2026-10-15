# Task timeout classification fix

## F0 — Intake and gates

- Branch: `codex/task-timeout-classification`; base: `deda642b`, clean and synchronized with origin/main on 2026-10-15.
- Workflow: P10 feedback → P8 backend correction → P9 verification. User authorized fixing the diagnosed bug and merging main.
- Existing evidence: customer screenshot and local handler analysis; customer deployed revision and raw error remain unknown.
- Workflow gate: implementation allowed. No UI/API/schema redesign required; required upstream contract is the existing task deadline and user-approved correction.
- Maintainability gate: narrow_fix; handlers are 318–673 lines, mix orchestration and persistence, moderate coupling. No broad refactor needed. Extract one small timeout predicate and cover context semantics and persisted batch outcome.
- Scope: single-agent, multi-agent, workflow, and batch task deadline classification. Unrelated robot request handling remains outside scope.

## F1 — Confirmed requirements

Confirmed by the user's request to fix the identified defect and merge main.

- R1: A live task must not be classified as globally timed out solely because a child operation returns DeadlineExceeded.
- R2: An expired task deadline must retain the existing timeout behavior, even if the returned error is wrapped or unrelated.
- R3: Cancellation must not be reclassified as deadline expiry, including cancellation whose cause wraps DeadlineExceeded.
- R4: Keep existing failure diagnostics, bounded execution limits, and cancellation behavior. This fix does not promise automatic continuation after every local failure or blindly replay tools.
- Acceptance: regression tests distinguish child timeout, task expiry, cancellation and wrapped HTTP errors; affected packages pass; main includes the reviewed change.

## F2 — Design

Introduce a package-local predicate using the task context's Err(), which reports DeadlineExceeded only when that context's deadline expires. Do not use the returned operation error or an arbitrary cancellation cause as evidence of task budget exhaustion.

Replace the four task-level OR checks with this predicate. Child timeouts follow the existing failed/error path with original error diagnostics and existing trace persistence. Real task expiry follows the unchanged timeout path. No new event schema or retry policy. No change to the 600-minute interactive or 6-hour batch budgets.
