import { useMemo, useRef, useState } from 'react'
import { useVirtualizer } from '@tanstack/react-virtual'
import { IconUndo, IconX } from '../icons'
import type { EditChange, Editable, StatementResult } from '../types'

// RowEdit is the staged change of one result row: new values by column
// index, or the whole row marked for deletion.
export interface RowEdit {
  set: Record<number, string | null>
  deleted: boolean
}
export type Edits = Record<number, RowEdit>

const ROW_H = 24
const HEAD_H = 28
const DEL_W = 28
const CHAR_W = 7.6
const SCROLLBAR = 14

// columnWidths sizes each column from its header and up to ~400 rows spread
// over the whole result, so long values deep in a large result still fit.
function columnWidths(stmt: StatementResult): number[] {
  const n = stmt.rows.length
  const step = Math.max(1, Math.floor(n / 200))
  const sample = stmt.rows.slice(0, 200)
  for (let r = 200; r < n; r += step) sample.push(stmt.rows[r])
  return stmt.columns.map((c, i) => {
    // Header shows "name type" with the type in a smaller font (~90% width).
    let chars = c.name.length + 2 + Math.ceil(c.type.length * 0.9)
    for (const r of sample) {
      const v = r[i]
      chars = Math.max(chars, v == null ? 4 : v.length)
    }
    return Math.round(Math.min(48, Math.max(5, chars)) * CHAR_W + 18)
  })
}

// buildChanges turns staged edits into the /edits request changes.
export function buildChanges(stmt: StatementResult, editable: Editable, edits: Edits): EditChange[] {
  const out: EditChange[] = []
  const rowIdx = Object.keys(edits)
    .map(Number)
    .sort((a, b) => a - b)
  for (const ri of rowIdx) {
    const e = edits[ri]
    const row = stmt.rows[ri]
    const key: Record<string, string | null> = {}
    for (const k of editable.keyColumns) key[k] = row[editable.columnSources.indexOf(k)]
    if (e.deleted) {
      out.push({ op: 'delete', key })
      continue
    }
    const set: Record<string, string | null> = {}
    for (const [ci, v] of Object.entries(e.set)) set[editable.columnSources[Number(ci)]!] = v
    if (Object.keys(set).length > 0) out.push({ op: 'update', key, set })
  }
  return out
}

export function ResultGrid({
  stmt,
  editable,
  edits,
  setEdits,
  fill,
}: {
  stmt: StatementResult
  editable: Editable | null
  edits: Edits
  setEdits: (fn: (prev: Edits) => Edits) => void
  fill: boolean
}) {
  const scrollRef = useRef<HTMLDivElement>(null)
  const widths = useMemo(() => columnWidths(stmt), [stmt])
  const canEdit = useMemo(
    () =>
      stmt.columns.map((_, i) => {
        const src = editable?.columnSources[i]
        return !!editable && src != null && !editable.keyColumns.includes(src)
      }),
    [stmt, editable],
  )
  const [editing, setEditing] = useState<{ row: number; col: number; value: string } | null>(null)
  const cancelled = useRef(false)

  const virt = useVirtualizer({
    count: stmt.rows.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => ROW_H,
    overscan: 20,
  })

  const lead = editable ? DEL_W : 0
  const totalW = lead + widths.reduce((a, b) => a + b, 0)
  const contentH = HEAD_H + stmt.rows.length * ROW_H + SCROLLBAR + 2

  const commit = (row: number, col: number, value: string | null) => {
    const orig = stmt.rows[row][col]
    setEdits((prev) => {
      const cur = prev[row] ?? { set: {}, deleted: false }
      const set = { ...cur.set }
      if (value === orig) delete set[col]
      else set[col] = value
      const next = { ...prev }
      if (Object.keys(set).length === 0 && !cur.deleted) delete next[row]
      else next[row] = { ...cur, set }
      return next
    })
    setEditing(null)
  }

  const toggleDelete = (row: number) =>
    setEdits((prev) => {
      const cur = prev[row] ?? { set: {}, deleted: false }
      const next = { ...prev, [row]: { ...cur, deleted: !cur.deleted } }
      if (!next[row].deleted && Object.keys(cur.set).length === 0) delete next[row]
      return next
    })

  return (
    <div
      ref={scrollRef}
      className={`grid-wrap${fill ? ' fill' : ''}`}
      style={fill ? { maxHeight: contentH } : { height: Math.min(contentH, 320) }}
    >
      <div className="grid-inner" style={{ width: totalW, height: HEAD_H + virt.getTotalSize() }}>
        <div className="grid-head" style={{ width: totalW, height: HEAD_H }}>
          {editable && <div className="gc lead" style={{ width: DEL_W }} />}
          {stmt.columns.map((c, i) => (
            <div key={i} className="gc" style={{ width: widths[i] }} title={`${c.name} ${c.type}`}>
              {c.name} <span className="ty">{c.type}</span>
            </div>
          ))}
        </div>
        {virt.getVirtualItems().map((vr) => {
          const ri = vr.index
          const row = stmt.rows[ri]
          const e = edits[ri]
          return (
            <div
              key={ri}
              className={`grid-row${e?.deleted ? ' del' : ''}${ri % 2 ? ' odd' : ''}`}
              style={{ width: totalW, height: ROW_H, transform: `translateY(${HEAD_H + vr.start}px)` }}
            >
              {editable && (
                <div className="gc lead" style={{ width: DEL_W }}>
                  <button
                    className="x"
                    aria-label={e?.deleted ? 'Keep this row' : 'Delete row'}
                    title={e?.deleted ? 'Keep this row' : 'Delete row'}
                    onClick={() => toggleDelete(ri)}
                  >
                    {e?.deleted ? <IconUndo size={13} /> : <IconX size={13} />}
                  </button>
                </div>
              )}
              {row.map((orig, ci) => {
                const changed = !!e && ci in e.set
                const v = changed ? e.set[ci] : orig
                const isEditing = editing?.row === ri && editing.col === ci
                const editableCell = canEdit[ci] && !e?.deleted
                return (
                  <div
                    key={ci}
                    className={`gc${v == null ? ' null' : ''}${changed ? ' chg' : ''}${editableCell ? ' ed' : ''}`}
                    style={{ width: widths[ci] }}
                    title={v ?? 'NULL'}
                    onDoubleClick={
                      editableCell
                        ? () => {
                            cancelled.current = false
                            setEditing({ row: ri, col: ci, value: v ?? '' })
                          }
                        : undefined
                    }
                  >
                    {isEditing ? (
                      <span className="cell-edit">
                        <input
                          autoFocus
                          value={editing.value}
                          onChange={(ev) => setEditing({ ...editing, value: ev.target.value })}
                          onKeyDown={(ev) => {
                            if (ev.key === 'Enter') {
                              ev.preventDefault()
                              commit(ri, ci, editing.value)
                            } else if (ev.key === 'Escape') {
                              ev.preventDefault()
                              cancelled.current = true
                              setEditing(null)
                            }
                          }}
                          onBlur={() => {
                            if (!cancelled.current) commit(ri, ci, editing.value)
                          }}
                        />
                        <button
                          className="setnull"
                          title="Set to NULL"
                          onMouseDown={(ev) => {
                            ev.preventDefault()
                            cancelled.current = true
                            commit(ri, ci, null)
                          }}
                        >
                          NULL
                        </button>
                      </span>
                    ) : v == null ? (
                      'NULL'
                    ) : (
                      v
                    )}
                  </div>
                )
              })}
            </div>
          )
        })}
      </div>
    </div>
  )
}
