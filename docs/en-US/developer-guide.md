# Developer Guide

[中文](../zh-CN/developer-guide.md)

This guide is for contributors extending CyberStrikeAI. The project is a Go single-service application with a static frontend, SQLite persistence, Agent/MCP orchestration, and optional high-risk security subsystems.

## Project Layout

```text
cmd/server/              service entrypoint
internal/app/            app wiring, routes, MCP tool registration
internal/handler/        HTTP handlers
internal/database/       SQLite access
internal/security/       auth, rate limits, shell execution
internal/mcp/            MCP server and external MCP manager
internal/multiagent/     Eino single-agent, multi-agent, middleware
internal/workflow/       graph orchestration runtime
internal/knowledge/      indexing and retrieval
internal/c2/             built-in C2
internal/project/        project fact blackboard
web/static/              frontend JS/CSS/assets
web/templates/           HTML templates
tools/                   YAML command tools
roles/                   role YAML
agents/                  multi-agent Markdown definitions
skills/                  Agent Skills
docs/                    documentation
```

## Development Startup

```bash
go run ./cmd/server --config config.yaml
```

The frontend is static. Most JS/CSS/template changes only require a browser refresh.

## One-Click Update: Letting an Installation Update Its Own Source

On a deployed machine the routine is "my tree moved forward by a few dozen commits -> bring this
installation up -> rebuild -> swap the binary -> restart". This used to be only `upgrade.sh`, which
pointed at one fixed upstream repository, so for anyone running a fork "upgrade" meant "overwrite
yourself with somebody else's code". The capability now lives in the platform (`internal/update`) and
the console page, the REST API and the CLI share one implementation, so the three surfaces cannot
disagree about why an update was refused.

### Three entry points

| entry | how it is used |
|---|---|
| console | Platform management -> One-click update (`#system-update`). Opening the page reads the local tree only and does not go online; **Check** is what performs the fetch; **Update** starts a job the page polls; the page also carries the "exit after updating" option and a rollback button |
| REST | `GET /api/system/update` (state on disk, no network), `POST /api/system/update/check` (fetch, then report the gap), `POST /api/system/update/apply` (`202` with `job_id`, polled through `GET /api/system/update/job`), `POST /api/system/update/rollback` |
| CLI | `./cyberstrike-ai -check-update`, `./cyberstrike-ai -update`, `./cyberstrike-ai -update-rollback`. The install root is the directory holding `--config`, or the current directory when `--config` was not given |

Permissions are `update:read` (GET) and `update:apply` (the four mutating endpoints). `update:apply`
additionally requires a global (`all` scope) session: one machine has one source tree, and an
`assigned`/`own` session must not be able to move the code everybody else is running.

### Where the code comes from

From **the remote this directory already tracks**; no third-party repository is hardcoded. The remote
is picked from the ones this tree has, in the order `mine`, `origin`, `upstream` (falling back to the
first remote when none of those names exists), and the branch defaults to what the current branch's
`@{upstream}` names (the current branch itself when there is no upstream). `upgrade.sh` is now a thin
shell over the same implementation: when this directory is a git work tree it just calls
`./cyberstrike-ai -update`.

### Choosing the source (the update section of config.yaml)

Unset, the rules above apply; set, the explicit choice wins:

```yaml
update:
  remote: origin        # an existing remote name (mine/origin/upstream/anything), or remote_url instead
  # remote_url: https://github.com/AIPentest/CyberStrikeAI.git
  branch: main          # optional; defaults to @{upstream} / the current branch
```

The console page edits and saves all three. Saving validates: remote and branch names go through the
same allowlist an update uses, and the address allowlist is https/http/ssh/git/file plus local
absolute paths - git's `ext::` transport executes commands and is rejected outright. Following the
official repository, your own fork, or somebody else's second-development repository is exactly the
choice this section expresses.

### Connecting a non-git installation (the tarball kind)

An installation that started as an unpacked Release archive has no `.git`; once a source is saved it
can be connected from the same page: the preview fetches in a throwaway repository and first lists
**the local files the target would replace** and **the operator content that will be kept**; on
confirm the directory becomes a git work tree (`git init`, origin added, the target branch checked
out), every replaced file is kept under `.update-backup/<timestamp>/overwritten/`, operator content
is stashed and restored as usual, and the binary is rebuilt. From then on it is a normal
installation with one-click update and rollback - but note there is no git history before the
connection, so the connection itself has no earlier commit to roll back to; the first real update
writes the first rollback point.

### The four refusals

| situation | reason | meaning and what to do |
|---|---|---|
| product **source** modified locally (a path outside the protected list) | `local_source_edits` | An update is a download, not a conflict decision. The file names are listed; commit or restore them first |
| branch **diverged** from its remote (ahead and behind at once) | `diverged` | That is a merge decision; the counts are reported so you can resolve it by hand |
| no Go toolchain on this box | `no_toolchain` | **The source still updates**; only the binary is not swapped. Install `go` and press update again to finish |
| this directory is not a git work tree | `not_a_repo` (`installed: false` in the status) | A tarball installation has no remote to fast-forward; update it through the release package |

`apply` is asynchronous, so it always answers `202` first and the refusals above arrive in the polled
`job.failure` (`reason` / `message` / `items`). `rollback` is synchronous and maps refusals to status
codes (`local_source_edits`, `diverged`, `no_state`, `moved_since_update`, `no_binary` -> 409, anything
else -> 400). Other failures (`merge_failed`, `build_failed`, `swap_failed`) carry git's or the
compiler's own output; a failed build leaves the previous binary in place while the source has already
moved and the state file is written, so it stays rollback-able.

### How operator content survives

`roles/ skills/ tools/ agents/ knowledge_base/ data/ log/ venv/ config.yaml .env` belong to the
**operator** (`update.Protected` is the judge). Before the merge, only those of those paths the
update actually writes to are copied into `.update-backup/<timestamp>/` and removed from the tree -
your extra directories that upstream never touched are not disturbed - and they are put back
afterwards, which is what "update the code, keep my work" means. The result names every file in
`keptContent` instead of quietly dropping it: where both you and upstream changed the same file, the
version that lands is yours, and the result says so.

### Rollback

`./cyberstrike-ai -update-rollback`, or the button on the page: `git reset --hard` back to the commit
the update came from, the binary kept as `cyberstrike-ai.prev` is put back, and `.update-state.json`
is removed. It refuses when there is no update record (`no_state`), no kept binary (`no_binary`), the
recorded commit is not in this repository (`bad_state`), **HEAD has moved since that update**
(`moved_since_update` - a rollback undoes the update, not the work done after it), or source is
modified locally (`local_source_edits`).

### The two restart cases

The process only stands down (graceful `Shutdown`, then exit 0) when the request explicitly says
`restart: true` **and** a restart hook was wired at startup. Whether it actually comes back is up to a
supervisor (systemd, `run.sh`), and the page says exactly that rather than promising a boot that may
not happen. Otherwise the API only reports `needsRestart` and the old binary keeps running. Asking for
a restart with no hook wired is a `400`, instead of stopping the service and claiming it restarted.

## Adding a Business Module

Do not add only a handler. A complete module usually needs:

1. Data model and SQLite migration.
2. Handler: parameters, errors, pagination/filtering.
3. Audit: management actions.
4. Monitor: long-running execution state.
5. MCP: whether Agents should call it.
6. HITL: approval boundary for MCP tools.
7. OpenAPI: update `/api/openapi/spec`.
8. Frontend: i18n, states, empty/error UI.
9. Tests: DB, handler, edge cases.
10. Docs: config, usage, troubleshooting, safety impact.

Missing one of these usually becomes a later usability or safety bug.

## Error Response Design

Prefer stable JSON:

```json
{
  "error": "machine_readable_code",
  "message": "human-readable explanation"
}
```

Frontend needs stable fields, users need actionable messages, and logs need detailed internal errors.

## Long-Running Tasks

For scanning, indexing, batch tasks, C2, or external operations, answer:

- Can it be cancelled?
- Can progress be queried?
- Can it be retried?
- Where is the result stored?
- Does state survive page refresh?
- Does it block the HTTP request?

If not, use task tables, event streams, or monitoring.

## Extending Tools

Prefer `tools/*.yaml` for command tools. Use Go built-in tools when the tool needs internal state or structured integration.

Built-in tools should define clear input schemas, handle timeouts and errors, and respect HITL for risky actions.

## Frontend Changes

Use existing helpers such as `apiFetch`, modal utilities, notifications, and i18n. Update both `web/static/i18n/zh-CN.json` and `web/static/i18n/en-US.json` for new visible text.

Avoid putting secrets or provider keys in frontend code.

## Test Priority

High-value tests:

- config hot-apply;
- HITL branches;
- shell timeout/no-output;
- external MCP recovery;
- KB indexing and post-processing;
- WebShell OS/encoding detection;
- SQLite migration compatibility.

## Source Anchors

- App wiring: `internal/app/app.go`
- Config apply: `internal/handler/config.go`
- OpenAPI: `internal/handler/openapi.go`
- Tool executor: `internal/security/executor.go`
- Skill package: `internal/skillpackage/`
- One-click update: `internal/update/` (status, apply, rollback), HTTP surface in
  `internal/handler/update.go`, CLI switches in `cmd/server/update_cli.go`, thin shell in
  `upgrade.sh`
