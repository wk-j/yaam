# Yaam end-to-end tests

A real Chromium browser exercises the built Go/React application and an explicitly supplied test PostgreSQL database. Nothing mocks the UI, HTTP API, or database. The first case covers **add Connection → Test Connection → SELECT 1 in Editor → result grid → Execution Log**. It does not cover MCP, approval/rejection, transactions, or editing rows.

## Run

From the repository root:

```sh
npm ci --prefix e2e --ignore-scripts
make build
node e2e/runner.mjs list
KRYPTON_E2E_RESULT=.krypton/e2e/smoke-result.json node e2e/runner.mjs run web-query-happy-path
node e2e/runner.mjs history
```

`make build` uses the existing web npm build and builds `bin/yaam`. Rebuild after changing application code: the runner checks for the binary and assets but does not build them or prove they are current. Never use `make install` for tests.

### Manual prerequisites

The runner never installs tools, browsers, or PostgreSQL:

- macOS with `/usr/bin/security`, Go 1.25+, Node.js/npm compatible with the web project, and Xcode command line tools/cgo.
- Chromium matching the local Playwright version. If unavailable, the user must provision it manually: `cd e2e && npx playwright install chromium`.
- An explicitly selected disposable/test PostgreSQL database with a dedicated role allowing passwordless TCP access and `SELECT 1`. Use no production database; do not change an existing service's authentication merely to run this test.

Set these environment variables yourself (the runner does not infer a database from `YAAM_TEST_PG`, personal configuration, or ambient `PG*` settings):

| Variable | Meaning |
|---|---|
| `YAAM_E2E_ALLOW_TEST_DB` | Must be `1` to confirm the target is a disposable/test database |
| `YAAM_E2E_PGHOST` | TCP hostname or IP address, not a connection URL |
| `YAAM_E2E_PGPORT` | Port, 1–65535 |
| `YAAM_E2E_PGDATABASE` | Test database name (letters, digits, `_`, `-`) |
| `YAAM_E2E_PGUSER` | Test role name (letters, digits, `_`, `-`) |
| `KRYPTON_E2E_RESULT` | Optional destination for a normalized Result; relative paths resolve against the invocation's working directory |

There is **no password input** for the first case. Neither `.pgpass` nor `PGPASSWORD` is used for the probe; the app child receives no ambient `PG*` variables except `PGPASSFILE=/dev/null`. A future password-based case needs an explicit design for `YAAM_E2E_PGPASSWORD` and Keychain cleanup; do not add a password to a URL, argument, Result, or log.

The runner launches its own `bin/yaam serve -addr 127.0.0.1:0`, private `/tmp/yaam-e2e-*` home, fresh browser context, and unique Connection name. It never calls `yaam install`/`yaam open`, attaches to an installed instance, or uses `~/.yaam`. A Keychain presence check refuses any name with an existing item; the case writes/deletes no Keychain passwords. The isolated home and its Connection config/log database are removed after the owned app stops. On ordinary interruption the runner stops its children; Krypton cancellation additionally terminates the launch's whole process group. Forced termination/power loss may leave private `/tmp/yaam-e2e-*` state; remove only confirmed stale run-owned directories manually.

Optional case parameter:

```sh
node e2e/runner.mjs run web-query-happy-path --param connectionName=krypton-test
```

`connectionName=new` (the default and `fresh` value) generates a UUID-based name. An explicit name must be valid in yaam and have no existing Keychain item. The SQL is always `SELECT 1 AS krypton_e2e_value`; no database objects are created or changed.

In Krypton:

```text
#e2e run web-query-happy-path
#e2e run web-query-happy-path --fresh
#e2e status
```

### Results and evidence

| Verdict | Meaning | Exit code |
|---|---|---|
| `PASS` | Every expected result held | 0 |
| `FAIL` | A browser expectation did not hold | 1 |
| `ERROR` | Invalid arguments, browser action/harness failure, missing outcome, or runner defect | 1 |
| `BLOCKED` | A required prerequisite is unavailable before the browser workflow starts | 2 |

Readiness includes explicit database opt-in, macOS/built app, local dependencies, Chromium launch, a passwordless database probe, absence of a conflicting Keychain item, and authenticated app readiness. Missing dependencies/database/browser give an honest `BLOCKED` Result, not a false pass or skipped suite. Output-directory permission failures or an uncatchable process kill cannot guarantee a Result.

Every terminal run keeps a stable run id and a normalized Result under `.krypton/e2e/artifacts/<runId>/`. `history` reads those Results without writing or contacting services; `list` is also read-only and needs neither test packages nor a running app. Results are also written atomically to `KRYPTON_E2E_RESULT` when supplied. Each run retains a bounded sanitized process/runner log and structured check observations; runs reaching the appropriate steps also retain screenshots of the result area and Execution Log table. All listed artifact paths are repository-relative. Raw Playwright traces, HAR, video, storage state, and full-page failure screenshots/reports are disabled to avoid retaining authentication material. The per-process bearer token remains in private temporary state/in memory; child logs are redacted before retention.

The manifest and evidence are under `.krypton`, which this repository already ignores. Keep `e2e/` source and its package lock when reviewing/committing manually; do not force-add runtime evidence or `.krypton` state. No GitHub labels, issues, comments, or external report uploads are created by the runner.

Runner checks (no browser/database required):

```sh
npm test --prefix e2e
```

The protocol tests run `BLOCKED` and invalid-parameter `ERROR` launches in disposable fixture directories, assert that real e2e history is unchanged, and remove only their temporary fixtures. They never write test results into Krypton's real history or synthesize passing browser runs. Earlier negative-test records already retained in history are left intact as historical evidence. To validate wiring after the smoke command, call Krypton's `e2e_adapter_check` with `result_path` set to `.krypton/e2e/smoke-result.json`. `valence check --root .` is an optional read-only evidence validation; uploads require a separate request.

## UI test identifiers

The web UI exposes `data-testid` on the happy-path controls and observations. Use Playwright's `getByTestId` rather than CSS classes or CodeMirror's internal DOM. Labeled form inputs keep `getByLabel` selectors so tests continue to exercise accessible field labels.

| Area | Stable identifiers |
|---|---|
| Navigation | `nav-<route>` (for example `nav-connections`, `nav-execution-log`), `sidebar-connection` |
| Connections | `connection-add`, `connection-save`, `connection-row`, `connection-test`, `connection-test-result` |
| Editor | `editor-connection`, `sql-editor-input`, `editor-run`, `editor-results`, `editor-sql-error` |
| Grid | `result-column-name`, `result-cell` |
| Execution Log | `execution-log`, `execution-log-sql`, `execution-log-connection`, `execution-log-status` |

Repeated identifiers represent collections: scope to a Connection, result area, or log row before asserting. The SQL input identifier is set through CodeMirror's content attributes, directly on its editable element. Both request-level and SQL-execution errors carry `editor-sql-error`. These attributes do not change application behavior, expose credentials, or bypass authentication/approval. Keep identifiers stable across styling/layout changes; update the test and this table together when changing the test contract.

## Adding a case

1. Add a Playwright test under `e2e/cases/` using real app interactions and an explicit, isolated test target. Use the UI test identifiers above or accessible roles/labels, not styling classes. Do not load test code from `list` or `history`.
2. Register a stable case id, title, declared `params`/`fresh`, and repository-relative source `files` in `runner.mjs`. Add `issue` only for a real supplied GitHub issue number. Update the runner's dispatch to the matching spec and case prerequisites; the initial runner dispatches only the first case.
3. Record actions as unique `kind: step` checks and each expectation as a separate `kind: expect` check linked to its step. Keep safe observed detail, expected/actual summaries, and evidence. Distinguish unmet app expectations from broken test code; never convert all errors or skipped tests into a pass.
4. Write the case outcome to the run's private `outcome.json` in `finally`. Retain only safe screenshots/observations in the run directory. Add produced evidence files to the runner's artifact list, never arbitrary private directories, tokens, or authorization headers. Do not enable raw reports, traces, or storage-state capture.
5. Check preconditions before exercising the app; missing data/services/secrets produce `BLOCKED`. Use environment variable **names** in documentation, never secret values. Extend cleanup only for data/processes owned by the run.
6. Run unit checks, list, smoke, and `e2e_adapter_check`; check artifacts exist and `history` preserves `(caseId, runId)`. Leave changes uncommitted for review.
