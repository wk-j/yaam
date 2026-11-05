---
status: accepted
---

# The main yaam process is a launchd agent started at login

`yaam serve`, the single process of [ADR 0006](0006-agents-reach-yaam-through-stdio-relay.md), is installed once
as a macOS launchd agent that starts at login, so it is already running whenever an Agent host starts `yaam mcp`.
The relay never starts the main process itself; when it cannot reach it, it ends with an error that says so.

## Considered Options

- The relay starts `yaam serve` in the background when none is running: not chosen; the main process would begin
  life as a descendant of whichever Agent host happened to start first.
- The relay only reports an error and the User starts `yaam serve` by hand: not chosen; the User would then have to
  reconnect the server in every Agent host that had already tried.

## Consequences

- No terminal shows the web UI's address with its per-run token
  ([ADR 0002](0002-approval-stops-mistakes-not-bypass.md)), so the User opens the web UI with `yaam open`, which
  reads the current run's token and opens the browser.
