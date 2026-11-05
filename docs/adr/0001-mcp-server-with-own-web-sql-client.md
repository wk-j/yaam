---
status: accepted
---

# yaam is an MCP server with its own web SQL client, not DBHub

DBHub already pairs a database MCP server with a web Workbench, so a reader will ask why yaam exists. yaam is an
MCP server that Agents call plus a web UI where Users work with the same databases, and it is built rather than
adopted because it needs three things together: a full SQL client for Users (editor with autocomplete, schema
browser, editable result grid, Execution Log); User control over Agents, where an Agent's Write runs only after the
User's Approval; and an Agent Log, where the User watches every call an Agent makes, including Writes that wait
for Approval or were rejected. Agents work only through their own MCP tools: they never see or change what the User
has open in the web UI.

## Considered Options

- Adopt DBHub: its Workbench is for executing MCP tools and viewing request traces, which it keeps in memory, the
  latest 100 per source, until restart; its only write control is a per-tool `readonly = true` switch that blocks
  every write, with no step where a User approves one Agent Write.
- Let Agents read what the User has open (editor text, selection, result on screen) and place queries in the
  User's editor: rejected; it couples the User's work to the Agent's and adds interaction the User does not want.
  The User only needs to watch what Agents do.
