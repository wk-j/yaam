# yaam

A web-based SQL client built MCP-first: Agents work with databases through yaam's MCP server, and Users work with the same databases in yaam's web UI.

## Language

**Agent**:
An AI assistant (such as Claude Code, Cursor or omp) that uses yaam by calling its MCP tools, told apart by the assistant's name and the project directory it runs in.
_Avoid_: client, MCP client (a protocol role, not a yaam term), bot

**User**:
A person who works with databases in yaam's web UI.
_Avoid_: client, operator

**Connection**:
A named database that yaam can reach, together with the role it signs in as, such as `shop-dev`. Users and Agents use the same Connections.
_Avoid_: data source, source, DSN (only one way to write an address), database (one server holds many databases)

**Request**:
The SQL an Agent sends to yaam in one call, with its Purpose; the SQL runs as one whole or not at all.
_Avoid_: query (one Request can hold several statements), transaction (the database's own unit; a Request runs as one), call

**Purpose**:
The Agent's required one-line statement of why it sends a Request, shown with the Request while it waits for Approval and kept in the Agent Log.
_Avoid_: reason (a User's rejection carries none), comment, description

**Agent Log**:
The record of every call an Agent makes to yaam, each Request and each schema lookup, and what became of it: run, waiting for Approval, rejected, cancelled, or failed. It never holds the rows a query returned.
_Avoid_: history, activity, trace, transaction log

**Execution Log**:
The record of all SQL run on a database through yaam for a User or an Agent, including what yaam writes for a User's edits in the result grid: who ran it, when, for how long, how many rows, and any error, but never the rows themselves. The SQL yaam runs on its own to look up schema is not in it.
_Avoid_: history, query log, SQL executable log ("executable" means runnable, not run), transaction log

**Read**:
A statement whose type only looks at data and is on yaam's short list of such types: a SELECT with no locking clause and no data-changing WITH, SHOW, or EXPLAIN without ANALYZE.
_Avoid_: query (a Write is a query too), safe statement

**Write**:
Any statement that is not a Read, including statement types yaam does not recognise.
_Avoid_: mutation, DML (DML leaves out DDL, CALL, DO and more)

**Approval**:
A User's explicit go-ahead for one Request that holds a Write, given while the Agent still waits for it; until it is given, the Request does not run.
_Avoid_: confirmation, permission (the Agent's own tool-permission prompt is a different thing)
