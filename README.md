# yaam

A local, web-based SQL client for PostgreSQL that AI agents use over MCP. Agents query your databases through
yaam; every query they send shows up in the web UI, and anything that writes waits for you to approve it.

- **Reads run at once; Writes wait for Approval.** Plain `SELECT`, `SHOW` and `EXPLAIN` (without `ANALYZE`) run
  immediately; every other statement waits until you approve or reject it in the web UI.
- **One Request is one transaction.** An agent's SQL commits or rolls back as a whole, with a time limit per
  Connection; `BEGIN`/`COMMIT`/`ROLLBACK` are refused.
- **Two logs.** The Agent Log shows every call an agent makes; the Execution Log shows all SQL that ran, yours
  included. Neither stores result rows.
- **A SQL client for you too.** Editor with schema completion, editable result grid, light and dark themes.

macOS only. Passwords live in the macOS Keychain; the web UI listens on `127.0.0.1` only.

## Install

Needs Go 1.25+, Node.js with npm, and the Xcode command line tools (cgo for PostgreSQL's parser).

```sh
git clone https://github.com/wk-j/yaam.git && cd yaam
make install    # builds, installs ~/go/bin/yaam, runs `yaam serve` at login (launchd)
yaam open       # opens the web UI
```

## Use

1. In the web UI, **Connections → Add Connection**: name (such as `shop-dev`), host, port, database, role,
   password.
2. Give a project's agents that Connection: copy the line from the Connection into the project's `.mcp.json`.

   ```json
   {
     "mcpServers": {
       "shop-dev": { "command": "yaam", "args": ["mcp", "shop-dev"] }
     }
   }
   ```

   The agent gets two tools, `run_sql` (SQL plus a one-line purpose) and `search_schema`.
3. Keep the web UI open and approve or reject Writes under **Approvals**; turn on its notifications to hear
   about them while the tab is in the background.

A database behind a bastion: open the tunnel yourself (`ssh -L 15432:db.internal:5432 bastion`) and point the
Connection at `localhost:15432`.

## Commands

```
yaam serve [-addr 127.0.0.1:7341]   run the main process (web UI, Approval, logs)
yaam mcp <connection>               stdio MCP relay for an agent's .mcp.json
yaam open                           open the web UI in the browser
yaam install                        run yaam serve at login (launchd)
yaam uninstall                      stop running yaam serve at login
```

State lives in `~/.yaam` (`config.toml`, `yaam.db`, `serve.log`); set `YAAM_HOME` to move it.

## Develop

```sh
make build    # web UI (npm) then bin/yaam
make test     # go test ./...
YAAM_TEST_PG=localhost:5432:postgres:postgres go test ./internal/db   # tests that need PostgreSQL
```

Terms are defined in [GLOSSARY.md](GLOSSARY.md); design decisions are in [docs/adr](docs/adr).
