# Self-update recovery and MCP edit fixes

## Intake (F0)

PR #346 was merged at 72cd8d62e36e536a6b18e9e10ea73d8d16294044 on explicit user direction to merge first and repair the reviewed defects afterwards. This is corrective work on branch `codex/self-update-recovery-fixes`; the checkout was clean. No release or deployment is requested.

Workflow gate: P10 feedback/iteration, implementation allowed. The frozen review at a63070d and executable PRR-002/PRR-007 counterexamples define the upstream contract; there are no missing product decisions. Acceptance requires preserved MCP stop state and refusal of ambiguous recovery before filesystem mutation.

Maintainability gate: `apply.go` has 1099 lines and combines recovery orchestration and filesystem IO; `external_mcp.go` has 508 lines. Risk is high for recovery. Allowed slice: narrow fix with a separate pure JSON/evidence validation module, small call-site changes, and focused regression tests. Broad refactoring is not required. Existing update/handler tests and reviewer reproductions cover the affected boundary.

## Requirements (F1, confirmed by the user's repair request)

- R1 / PRR-007: editing an existing MCP without an explicit enable/disable field preserves its runtime activation state. Explicit activation fields and Start/Stop remain supported; a new server defaults to enabled. Environment expansion remains isolated from persisted raw values.
- R2 / PRR-002: incomplete, null, duplicate-key, malformed, or contradictory recovery evidence cannot authorize a second swap, wrong rollback binary, or deletion of recovery records. Valid current records and the documented legacy marker retain safe recovery.
- Non-goals: redesigning update UX, changing release policy, or unrelated refactoring. The two nonblocking review observations are outside this corrective slice unless required by the evidence validation boundary.
