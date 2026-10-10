# Russian layout overflow

## F0 Intake / workflow and maintainability gates

User reported Russian overflow with a dashboard screenshot. Clean main checkout; dedicated branch codex/russian-layout-overflow.
Workflow: P10 feedback → P7 narrow fix → P9 visual QA. Existing template, translations and screenshot supply upstream UI contract; no missing product/API decisions. Implementation allowed.
Scope: dashboard severity/status/risk presentation and shared sidebar labels. Acceptance: complete readable labels without overlapping adjacent cells at desktop and narrow widths, with existing themes/navigation preserved.
Maintainability: style.css 47,163 lines and index.html are existing large composition files. High size risk; CSS presentation only, no new responsibility. Allowed narrow_fix; no broad refactor required. Validate actual template/styles/translations in Chromium, plus diff check. Assumption: wrapping is preferable to truncating operational status labels; no translation changes.
Lifecycle: F0–F6 documentation here, scoped commits pushed per phase; Unreleased changelog, no merge/release/deploy requested.
