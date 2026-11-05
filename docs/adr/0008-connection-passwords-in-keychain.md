---
status: accepted
---

# Connection passwords live in the macOS Keychain, set through the web UI

The User adds a Connection in the web UI with its address, role and password. yaam keeps the address and role in
its own configuration and the password in the macOS Keychain, so no file holds a password and a new Connection is
usable without restarting yaam. The web UI accepts a password but never sends a saved one back: no HTTP response
contains a password, following tantalum's
[ADR 0003](https://github.com/wk-j/tantalum/blob/main/docs/adr/0003-web-ui-loopback-write-only.md) for its own web UI.

## Considered Options

- Secrets in tantalum's default vault, injected with `ta run -s ... -- yaam serve` started outside any git
  repository: rejected; tantalum keeps one vault per git repository and hands values only to a child's
  environment, so every secret must be listed when yaam starts, and adding a Connection means restarting yaam,
  which cuts every Agent waiting for Approval.
- PostgreSQL's `~/.pg_service.conf` and `~/.pgpass`: rejected; the password sits in a plain-text file.
