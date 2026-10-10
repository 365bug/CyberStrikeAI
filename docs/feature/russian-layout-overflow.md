# Russian layout overflow

## F0 Intake / workflow and maintainability gates

User reported Russian overflow with a dashboard screenshot. Clean main checkout; dedicated branch codex/russian-layout-overflow.
Workflow: P10 feedback → P7 narrow fix → P9 visual QA. Existing template, translations and screenshot supply upstream UI contract; no missing product/API decisions. Implementation allowed.
Scope: dashboard severity/status/risk presentation and shared sidebar labels. Acceptance: complete readable labels without overlapping adjacent cells at desktop and narrow widths, with existing themes/navigation preserved.
Maintainability: style.css 47,163 lines and index.html are existing large composition files. High size risk; CSS presentation only, no new responsibility. Allowed narrow_fix; no broad refactor required. Validate actual template/styles/translations in Chromium, plus diff check. Assumption: wrapping is preferable to truncating operational status labels; no translation changes.
Lifecycle: F0–F6 documentation here, scoped commits pushed per phase; Unreleased changelog, no merge/release/deploy requested.

## F1 Requirements

Small corrective scope inferred from explicit bug report; no unresolved product decision or separate draft requirements gate needed.
REQ/AC-001: Russian status labels stay inside their cards with no overlap, including false-positive labels.
REQ/AC-002: Risk badge, severity names and navigation labels remain fully readable by wrapping within available space.
REQ/AC-003: Chinese/English, light/dark, desktop/tablet/mobile and collapsed sidebar retain usable layout and click targets. Data/loading/error rendering and API contracts unchanged.

## F2 Design

Use existing shared styles with language-independent wrapping, bounded flex children and auto-fit grid tracks. Status panel uses container width to decide whether progress fits alongside cards; avoid viewport-only decisions when dashboard sits beside a right column. Keep existing colors, semantics, event handlers and translations. Sidebar labels wrap with stable nonshrinking icons; collapsed rules continue hiding labels.

## F3 Implementation Plan

Touch style.css existing navigation/dashboard rules and index.html stylesheet cache key. Constrain and wrap risk/urgent/status/severity/batch labels; make status grid auto-fit and progress responsive to actual panel width. Verify real extracted dashboard/sidebar template with actual i18n dictionaries in headless Chromium at 320/390/768/1024/1440/1920/2938px, light/dark and all three languages; inspect screenshots. Check overlap and label containment, collapsed/sidebar states, nonzero large values and diff whitespace. Record evidence and Unreleased fix. Rollback via reverting scoped implementation commit; no backend changes.

## F4 Implementation

Shared navigation labels now wrap instead of ellipsis. Risk header wraps badge/label, urgent and batch labels are bounded and wrap. Severity legend gives remaining space to complete label text and intrinsic space to numeric columns. Status cards auto-fit with a 110px preferred minimum and bounded multiline labels; progress stacks until chart container has at least 1000px. Container queries at 760/480px adapt risk/chart/legend to the real panel width. Header actions wrap; main grid no longer expands from min-content. Updated stylesheet cache key.
Validation before code commit: headless Chrome 42 language/theme/width scenarios passed for measured label containment, no overlapping status cards and main card element boundaries; existing mobile i18n 8/8 tests and diff check passed.
