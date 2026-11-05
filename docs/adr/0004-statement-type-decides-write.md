---
status: accepted
---

# Statement type, not a trial run, decides what is a Write

yaam parses the SQL of each Agent Request and decides from its statement types alone whether it holds a Write and
so needs Approval; it does not first run the Request in a read-only transaction to let PostgreSQL decide. Whether a
Request needs Approval is known from its SQL text before anything runs, and no Request runs twice. The price is
that a statement whose type reads but whose effect writes runs without Approval: a `SELECT` that calls a function
which changes data, such as `SELECT archive_old_orders()` over a function that deletes rows, or
`SELECT nextval('order_seq')`, counts as a Read.

The list yaam keeps is of Reads, not of Writes: a `SELECT` with no locking clause, no `INTO` and no data-changing
`WITH`, `SHOW`, and `EXPLAIN` without `ANALYZE`. Every other statement is a Write, including `CALL`, `DO`,
`EXPLAIN ANALYZE`, `COPY`, `SET`, `LOCK` and any statement type yaam does not recognise, so a misjudged statement
costs the User one Approval rather than data. A Request needs Approval when any one of its statements is a Write.
A Request that starts or ends a transaction, such as `SELECT 1; COMMIT; DELETE ...`, never reaches Approval: it is
refused outright ([ADR 0005](0005-one-agent-request-one-transaction.md)).

## Considered Options

- Run every Agent Request in a read-only transaction first and treat PostgreSQL's refusal (`25006`, or `25001` for
  commands that cannot run in a transaction) as a Write: not chosen. On PostgreSQL 15.18 it refused writes hidden
  in functions, `nextval()`, `SELECT ... FOR UPDATE` and `CREATE TEMP TABLE`, but a Request that writes would run
  twice, and in `SELECT 1; COMMIT; DELETE ...` sent as one Request the `COMMIT` ended the read-only transaction,
  so the `DELETE` ran unchecked.
