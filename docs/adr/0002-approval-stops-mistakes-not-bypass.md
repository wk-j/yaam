---
status: accepted
---

# Approval stops Agent mistakes, not an Agent that sets out to bypass it

The first version runs on the User's own machine for one User, so every Agent it serves runs on that machine too,
with a shell. Such an Agent could call the web UI's approval API itself or, if it can read a database's
credentials, connect with `psql` and skip yaam altogether. Approval therefore guards against an Agent's honest
mistakes only: the User approves with a button in the web UI, which listens on loopback only, requires a random
token created anew on every run, and rejects requests whose `Host` or `Origin` is not yaam itself, so other
websites and DNS rebinding cannot reach it. Containing an Agent that deliberately bypasses Approval needs a
separate OS user or a sandbox and is outside yaam. Serving several Users is left for a later version.

## Considered Options

- Confirm every Approval through the Mac's own prompt (Touch ID or login password): rejected; an Agent cannot
  answer that prompt, but it can still connect to the database directly, so the prompt adds a step to every Write
  without closing that path.
- The Mac's prompt only for risky Writes (DDL, or UPDATE and DELETE without WHERE): rejected for the same reason.
