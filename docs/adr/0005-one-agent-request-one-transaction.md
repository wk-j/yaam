---
status: accepted
---

# One Agent Request runs as one transaction

yaam opens a transaction for every Agent Request, commits it when all its statements succeed and rolls it back at
the first error, so several Writes in one Request take effect together or not at all. No transaction, setting or
temporary table outlives the Request, so nothing an Agent left open can hold locks while another Request waits for
Approval, or after the Agent has moved on. A Request containing a statement that starts or ends a transaction
(`BEGIN`, `START TRANSACTION`, `COMMIT`, `END`, `ROLLBACK`, `ABORT`, `PREPARE TRANSACTION`) is refused at once with
the reason, without waiting for the User; the Agent sends it again without them.

## Considered Options

- A database session of its own for each Agent, kept across calls, so an Agent can open a transaction in one call
  and commit it in another: rejected; an Agent that never commits leaves rows locked, and the User then cannot
  change them in the web UI.
