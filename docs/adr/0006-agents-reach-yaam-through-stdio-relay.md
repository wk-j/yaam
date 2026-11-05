---
status: accepted
---

# Agents reach yaam through a stdio relay, not HTTP

One long-running yaam process, a single instance serving every project, holds the web UI, Approval, the Agent Log,
the Execution Log and the database connections. An Agent does not call that process over HTTP: its `.mcp.json`
runs `yaam mcp`, which speaks MCP over stdio and relays every message to the main process. Every Agent host
supports stdio; the relay knows the directory it was started in, so the Agent Log names each Agent by its client
name and project directory, such as Claude Code in `~/Source/shop`, with no setup; and Claude Code waits 30 minutes
on a silent stdio tool call against 5 minutes over HTTP, which matters for a Request waiting for Approval
([ADR 0003](0003-write-waits-inside-agent-tool-call.md)).

## Considered Options

- Agents call an HTTP MCP endpoint on the main process directly: rejected; yaam would know only the client name the
  Agent reports, so two projects open in Claude Code would look alike in the Agent Log unless each project set a
  header of its own, and Claude Code cuts a silent HTTP tool call after five minutes.
