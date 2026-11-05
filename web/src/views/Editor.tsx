import { useEffect, useMemo, useRef, useState } from 'react'
import { api, enc } from '../api'
import { fmtDuration, plural } from '../format'
import { Modal } from '../Modal'
import { RUN_KEYS, SqlEditor } from '../editor/SqlEditor'
import { ResultGrid, buildChanges, type Edits } from '../editor/ResultGrid'
import type { Connection, EditsResponse, RunResult, Schema } from '../types'
import { SqlBlock } from '../sql'
import { useStoredFlag } from '../prefs'
import {
  IconButton,
  IconChevronRight,
  IconKey,
  IconPanelCollapse,
  IconPanelExpand,
  IconPlay,
  IconRefresh,
  IconSave,
  IconSearch,
  IconTable,
  IconUndo,
  IconView,
  InfoTip,
} from '../icons'

const docKey = (conn: string) => `yaam.sql.${conn}`

export function Editor({ connection }: { connection?: Connection }) {
  const name = connection?.name ?? ''
  const nameRef = useRef(name)
  nameRef.current = name

  const [schema, setSchema] = useState<Schema | null>(null)
  const [schemaError, setSchemaError] = useState('')
  const [schemaGen, setSchemaGen] = useState(0)
  const [schemaHidden, toggleSchema] = useStoredFlag('yaam.schema.hidden')
  const doc = useRef('')
  const [result, setResult] = useState<RunResult | null>(null)
  const [lastSql, setLastSql] = useState('')
  const [runError, setRunError] = useState('')
  const [running, setRunning] = useState(false)
  const [edits, setEdits] = useState<Edits>({})
  const [review, setReview] = useState<{ sql: string[]; error: string; busy: boolean } | null>(null)

  // A new Connection starts clean: its own SQL text, schema and no result.
  useEffect(() => {
    doc.current = name ? (localStorage.getItem(docKey(name)) ?? '') : ''
    setResult(null)
    setEdits({})
    setRunError('')
  }, [name])

  useEffect(() => {
    if (!name) return
    let cancelled = false
    setSchema(null)
    setSchemaError('')
    api<Schema>('GET', `/api/connections/${enc(name)}/schema`)
      .then((s) => !cancelled && setSchema(s))
      .catch((err: Error) => !cancelled && setSchemaError(err.message))
    return () => {
      cancelled = true
    }
  }, [name, schemaGen])

  const run = async (sql: string) => {
    if (!name || running || !sql.trim()) return
    const conn = name
    setRunning(true)
    setRunError('')
    try {
      const r = await api<RunResult>('POST', `/api/connections/${enc(conn)}/run`, { sql })
      if (nameRef.current !== conn) return
      setResult(r)
      setLastSql(sql)
      setEdits({})
    } catch (err) {
      if (nameRef.current === conn) setRunError((err as Error).message)
    } finally {
      setRunning(false)
    }
  }

  // The grid edits the one statement that returned rows (backend rule).
  const target = result?.statements.findIndex((s) => s.columns.length > 0) ?? -1
  const editable = result?.editable ?? null
  const changes = useMemo(
    () => (result && editable && target >= 0 ? buildChanges(result.statements[target], editable, edits) : []),
    [result, editable, target, edits],
  )
  const withRows = result?.statements.filter((s) => s.columns.length > 0).length ?? 0

  const openReview = async () => {
    if (!editable) return
    setReview({ sql: [], error: '', busy: true })
    try {
      const r = await api<EditsResponse>('POST', `/api/connections/${enc(name)}/edits`, {
        schema: editable.schema,
        table: editable.table,
        changes,
        apply: false,
      })
      setReview({ sql: r.sql, error: '', busy: false })
    } catch (err) {
      setReview({ sql: [], error: (err as Error).message, busy: false })
    }
  }

  const save = async () => {
    if (!editable || !review) return
    setReview({ ...review, busy: true, error: '' })
    try {
      const r = await api<EditsResponse>('POST', `/api/connections/${enc(name)}/edits`, {
        schema: editable.schema,
        table: editable.table,
        changes,
        apply: true,
      })
      if (r.error) {
        setReview({ sql: r.sql, error: r.error, busy: false })
        return
      }
      setReview(null)
      setEdits({})
      await run(lastSql)
    } catch (err) {
      setReview({ ...review, busy: false, error: (err as Error).message })
    }
  }

  if (!connection) {
    return <p className="muted">No Connection yet. Add one on the Connections page.</p>
  }

  const rowsTotal = result?.statements.reduce((n, s) => n + (s.columns.length > 0 ? s.rowCount : 0), 0) ?? 0

  return (
    <div className={schemaHidden ? 'ed-grid schema-hidden' : 'ed-grid'}>
      {schemaHidden ? (
        <div className="schema-rail">
          <IconButton label="Show schema" className="sm" onClick={toggleSchema}>
            <IconPanelExpand size={14} />
          </IconButton>
        </div>
      ) : (
        <SchemaBrowser
          conn={name}
          schema={schema}
          error={schemaError}
          onRefresh={() => setSchemaGen((g) => g + 1)}
          onHide={toggleSchema}
        />
      )}
      <div className="work">
        <SqlEditor
          key={name}
          initialDoc={localStorage.getItem(docKey(name)) ?? ''}
          schema={schema}
          onChange={(d) => {
            doc.current = d
            localStorage.setItem(docKey(name), d)
          }}
          onRun={() => run(doc.current)}
        />
        <div className="toolbar">
          <button className="btn pri" disabled={running} onClick={() => run(doc.current)}>
            <IconPlay size={13} />
            {running ? 'Running…' : `Run ${RUN_KEYS}`}
          </button>
          {result && (
            <span className="muted trunc">
              {result.statements.length > 1 && `${result.statements.length} statements · `}
              {withRows > 0 && `${plural(rowsTotal, 'row', 'rows')} · `}
              {fmtDuration(result.durationMs)}
            </span>
          )}
          {editable && <InfoTip text="Double-click a cell to edit · Enter keeps · Esc cancels · × marks a row for deletion" />}
          <span className="sp" />
        </div>
        {changes.length > 0 && editable && (
          <div className="pending-bar">
            <span className="trunc">
              {plural(changes.length, 'unsaved change', 'unsaved changes')} to{' '}
              <span className="mono">{editable.table}</span>
            </span>
            <span className="sp" />
            <button className="btn" onClick={() => setEdits({})}>
              <IconUndo size={14} />
              Discard
            </button>
            <button className="btn pri" onClick={openReview}>
              <IconSave size={14} />
              Review &amp; Save
            </button>
          </div>
        )}
        <div className="results">
          {runError && <div className="errbox">{runError}</div>}
          {result?.statements.map((s, i) => (
            <section key={i} className={`stmt-res${withRows === 1 && s.columns.length > 0 ? ' fill' : ''}`}>
              <div className="stmt-head">
                <span className="mono">{s.command || '(no command)'}</span>
                {s.columns.length > 0 && <span className="muted"> · {plural(s.rowCount, 'row', 'rows')}</span>}
              </div>
              {s.columns.length > 0 && (
                <ResultGrid
                  stmt={s}
                  editable={i === target ? editable : null}
                  edits={i === target ? edits : {}}
                  setEdits={setEdits}
                  fill={withRows === 1}
                />
              )}
            </section>
          ))}
          {result?.error && <div className="errbox">{result.error}</div>}
          {result && result.notices.length > 0 && (
            <ul className="notices">
              {result.notices.map((n, i) => (
                <li key={i}>{n}</li>
              ))}
            </ul>
          )}
          {result && !editable && withRows > 0 && (
            <div
              className="cap trunc"
              title="The grid edits only a result from one table that has a primary key, with every key column selected; tables without a primary key show their rows but cannot be edited here."
            >
              Read-only: the grid edits one table's rows with its primary key selected.
            </div>
          )}
        </div>
      </div>
      {review && editable && (
        <Modal wide onClose={() => setReview(null)}>
          <h3>
            Save {plural(changes.length, 'change', 'changes')} to {editable.table}
          </h3>
          <p className="muted dlg-text">yaam runs these together in one transaction. If one fails, none of them apply.</p>
          {review.busy && review.sql.length === 0 ? (
            <pre className="review-sql">
              <code>Building SQL…</code>
            </pre>
          ) : (
            <SqlBlock sql={review.sql.join('\n')} className="review-sql" />
          )}
          {review.error && <div className="errbox">{review.error}</div>}
          <div className="dlg-act">
            <button className="btn" onClick={() => setReview(null)}>
              Cancel
            </button>
            <button className="btn pri" disabled={review.busy || review.sql.length === 0} onClick={save}>
              <IconSave size={14} />
              Run and save
            </button>
          </div>
        </Modal>
      )}
    </div>
  )
}

function SchemaBrowser({
  conn,
  schema,
  error,
  onRefresh,
  onHide,
}: {
  conn: string
  schema: Schema | null
  error: string
  onRefresh: () => void
  onHide: () => void
}) {
  const [filter, setFilter] = useState('')
  const f = filter.trim().toLowerCase()
  const tables = (schema?.tables ?? []).filter(
    (t) => !f || t.name.toLowerCase().includes(f) || t.columns.some((c) => c.name.toLowerCase().includes(f)),
  )
  const schemas = [...new Set(tables.map((t) => t.schema))]

  return (
    <div className="schema">
      <div className="schema-top">
        <label className="search">
          <IconSearch size={14} />
          <input placeholder="Filter tables" aria-label="Filter tables" value={filter} onChange={(e) => setFilter(e.target.value)} />
        </label>
        <IconButton label="Reload schema" className="sm" onClick={onRefresh}>
          <IconRefresh size={14} />
        </IconButton>
        <IconButton label="Hide schema" className="sm" onClick={onHide}>
          <IconPanelCollapse size={14} />
        </IconButton>
      </div>
      {error && <div className="err small wrap">{error}</div>}
      {!schema && !error && <div className="muted small">Loading schema…</div>}
      {schema && tables.length === 0 && <div className="muted small">No tables.</div>}
      {schemas.map((s) => (
        <div key={s}>
          <div className="muted schema-name trunc" title={`${conn} · ${s}`}>
            {conn} · {s}
          </div>
          {tables
            .filter((t) => t.schema === s)
            .map((t) => (
              <details key={t.name} open={!!f}>
                <summary title={`${t.schema}.${t.name}${t.kind !== 'table' ? ` (${t.kind})` : ''}`}>
                  <IconChevronRight size={12} className="chev-ico" />
                  {t.kind === 'table' ? <IconTable size={13} /> : <IconView size={13} />}
                  <span className="trunc">{t.name}</span>
                  {t.kind !== 'table' && <span className="muted kind">{t.kind}</span>}
                </summary>
                <ul>
                  {t.columns.map((c) => (
                    <li key={c.name} className={c.pk ? 'pk' : ''} title={`${c.name} ${c.type}${c.pk ? ' · primary key' : ''}`}>
                      <span className="pk-mark">{c.pk && <IconKey size={11} />}</span>
                      <span className="trunc">
                        {c.name} <span className="ty">{c.type}</span>
                      </span>
                    </li>
                  ))}
                  {t.kind === 'table' && !t.hasPrimaryKey && <li className="muted">no primary key</li>}
                </ul>
              </details>
            ))}
        </div>
      ))}
    </div>
  )
}
