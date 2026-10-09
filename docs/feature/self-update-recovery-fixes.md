# Self-update recovery and MCP edit fixes

## Intake (F0)

PR #346 was merged at 72cd8d62e36e536a6b18e9e10ea73d8d16294044 on explicit user direction to merge first and repair the reviewed defects afterwards. This is corrective work on branch `codex/self-update-recovery-fixes`; the checkout was clean. No release or deployment is requested.

Workflow gate: P10 feedback/iteration, implementation allowed. The frozen review at a63070d and executable PRR-002/PRR-007 counterexamples define the upstream contract; there are no missing product decisions. Acceptance requires preserved MCP stop state and refusal of ambiguous recovery before filesystem mutation.

Maintainability gate: `apply.go` has 1099 lines and combines recovery orchestration and filesystem IO; `external_mcp.go` has 508 lines. Risk is high for recovery. Allowed slice: narrow fix with a separate pure JSON/evidence validation module, small call-site changes, and focused regression tests. Broad refactoring is not required. Existing update/handler tests and reviewer reproductions cover the affected boundary.
