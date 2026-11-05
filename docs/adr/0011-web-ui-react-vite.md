---
status: accepted
---

# The web UI is React built with Vite, embedded in the Go binary

The web UI is written in React and built with Vite and npm, the same stack as Krypton; `go build` embeds the built
files, so building yaam takes an npm build first. The UI carries a SQL editor with autocomplete, a result grid
that edits rows and has to hold large results (yaam puts no limit on the rows a query returns), the Agent Log and
the Requests waiting for Approval, which is far more than tantalum's page. The result grid is a plain React table
virtualized with TanStack Virtual, so only the rows on screen are in the DOM however many rows a query returns, and
editing stays fully under yaam's control. React has the larger choice of such libraries.

## Considered Options

- Plain JavaScript and CSS with no build step, as tantalum's web UI is, with CodeMirror 6 built once into a single
  committed file: not chosen.
- Svelte with Vite: not chosen; fewer ready-made libraries for editors and large grids.
- Glide Data Grid, a canvas grid with built-in editing: not chosen; its stable release supports React only up to
  18, and only an alpha supports React 19.
