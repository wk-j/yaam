---
status: accepted
---

# A project names each Connection it uses as an MCP server of its own

A project's `.mcp.json` holds one MCP server entry per Connection, each running the relay with that Connection's
name as its parameter:

```json
"shop-dev": { "command": "yaam", "args": ["mcp", "shop-dev"] }
```

A project's Agents therefore see only the Connections its `.mcp.json` names, and `.mcp.json` is the only gate:
yaam serves whichever Connection a relay names and keeps no list of which project may use which Connection. Each
Connection's address and password are defined once in yaam and shared with the User's web UI, so `.mcp.json` holds
no secret and can be committed. Because each server is bound to one Connection, yaam's tools take no Connection
argument, and an Agent sees each Connection as a separate server.

## Considered Options

- The project supplies the database address from its own tantalum vault
  (`ta run -s SHOP_DB_URL -- yaam mcp`): rejected; the User would have to set up the same address and password
  again to use that database in the web UI.
- yaam also keeps, per Connection, a list of projects allowed to use it, set by the User in the web UI: not chosen;
  every new project would need setting up in two places.
- The first Request from a project to a Connection waits for the User to allow that pair once in the web UI: not
  chosen.

## Consequences

- A committed `.mcp.json` that names a Connection the User happens to have under the same name, such as `prod`,
  reaches that Connection, and an Agent that edits `.mcp.json` can add any Connection. In interactive sessions
  Claude Code asks once before using a new server from `.mcp.json`; `claude -p`, Agent SDK and cloud sessions load
  such servers without asking.
