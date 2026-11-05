---
status: accepted
---

# The web UI is React built with Vite, embedded in the Go binary

The web UI is written in React and built with Vite and npm, the same stack as Krypton; `go build` embeds the built
files, so building yaam takes an npm build first. The UI carries a SQL editor with autocomplete, a result grid
that edits rows and has to hold large results (yaam puts no limit on the rows a query returns), the Agent Log and
the Requests waiting for Approval, which is far more than tantalum's page; React offers ready-made editable grids
for large results, such as Glide Data Grid, which draws on a canvas.

## Considered Options

- Plain JavaScript and CSS with no build step, as tantalum's web UI is, with CodeMirror 6 built once into a single
  committed file: not chosen.
- Svelte with Vite: not chosen; fewer ready-made editable grids.
