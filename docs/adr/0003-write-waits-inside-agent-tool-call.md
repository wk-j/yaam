---
status: accepted
---

# An Agent's Write waits for Approval inside the Agent's own tool call

When an Agent sends a Write, yaam holds the tool call open until the User approves or rejects it, then answers that
same call with the result or the rejection. A Write runs only while its call is still open: if the Agent cancels
the call or disconnects first, yaam cancels the Write, records it as cancelled in the Agent Log, and never runs it,
because an Agent that stopped waiting may already have changed its plan. While waiting, yaam sends progress
notifications so that the Agent's idle timeout does not cut the call. Claude Code, for one, cuts a silent call
after five minutes over HTTP and thirty over stdio, and moves a call that lasts over two minutes to the background
so the Agent keeps working meanwhile.

## Considered Options

- Answer at once with an id the Agent asks about later: rejected; the Agent may never ask, or ask too often, and
  an approved Write could run long after the Agent moved on.
- The MCP tasks extension (protocol revision 2026-07-28): rejected for now; Claude Code supports it only in its
  CLI, not in its VS Code extension or desktop app, and other Agents may not support it at all.

## Consequences

- yaam can send progress notifications only when the Agent's tool call carries a progress token; without one, the
  Agent's idle timeout bounds how long a Write can wait for Approval.
