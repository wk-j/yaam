---
status: accepted
---

# yaam is written in Go

yaam is a single Go binary holding `yaam serve`, `yaam mcp` and the embedded web UI. Go brings `pg_query_go`, which
wraps PostgreSQL's own parser, so the list of Reads in [ADR 0004](0004-statement-type-decides-write.md) is checked
against the same parse tree PostgreSQL builds, and it lets yaam reuse what tantalum already does in Go: the
Keychain access and the loopback-only web UI with a per-run token.

## Considered Options

- Rust, with `pg_query.rs` over the same parser: not chosen.
- TypeScript on Bun, compiled to one binary: not chosen; PostgreSQL's parser would run as WASM.
