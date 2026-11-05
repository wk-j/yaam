import { useState, type KeyboardEvent, type ReactNode } from 'react'
import { baseName, fmtCount, fmtDateTime, fmtDuration, fmtTime } from '../format'
import { IconChevronRight, IconChevronsDown } from '../icons'
import { useLog } from '../live'
import { CELL_SQL_MAX, SqlBlock, SqlText } from '../sql'
import type { AgentLogEntry, AgentLogStatus } from '../types'

const STATUS_LABEL: Record<AgentLogStatus, string> = {
  run: 'run',
  waiting: 'waiting for Approval',
  rejected: 'rejected',
  cancelled: 'cancelled',
  failed: 'failed',
}

const STATUS_CLASS: Record<AgentLogStatus, string> = {
  run: 'run',
  waiting: 'wait',
  rejected: 'rej',
  cancelled: 'can',
  failed: 'fail',
}

const STATUSES: AgentLogStatus[] = ['run', 'waiting', 'rejected', 'cancelled', 'failed']

const agentKey = (e: AgentLogEntry) => `${e.agentName} · ${baseName(e.project)}`

export function AgentLog({ limit }: { limit: number }) {
  const log = useLog<AgentLogEntry>('/api/agent-log', 'agent-log')
  const [agent, setAgent] = useState('')
  const [status, setStatus] = useState<AgentLogStatus | ''>('')

  const agents = [...new Set(log.entries.map(agentKey))].sort()
  const shown = log.entries.filter((e) => (!agent || agentKey(e) === agent) && (!status || e.status === status))

  return (
    <>
      <div className="filters">
        <button className="chip" aria-pressed={agent === ''} onClick={() => setAgent('')}>
          All Agents
        </button>
        {agents.map((a) => (
          <button key={a} className="chip" aria-pressed={agent === a} onClick={() => setAgent(a)}>
            {a}
          </button>
        ))}
        <span className="chip-gap" />
        <button className="chip" aria-pressed={status === ''} onClick={() => setStatus('')}>
          All statuses
        </button>
        {STATUSES.map((s) => (
          <button key={s} className="chip" aria-pressed={status === s} onClick={() => setStatus(s)}>
            {s}
          </button>
        ))}
      </div>
      <table className="log agent">
        <thead>
          <tr>
            <th className="chev-cell">
              <span className="sr-only">Details</span>
            </th>
            <th>Time</th>
            <th>Agent</th>
            <th>Connection</th>
            <th>Purpose</th>
            <th>SQL / lookup</th>
            <th>Status</th>
            <th>Time taken</th>
            <th>Rows</th>
          </tr>
        </thead>
        <tbody>
          {shown.map((e) => {
            const schema = e.kind === 'schema'
            const sql = schema ? JSON.stringify(e.sql) : e.sql
            const note = e.error || (e.status === 'cancelled' ? 'Agent stopped waiting' : '')
            return (
              <LogRow
                key={e.id}
                cols={8}
                detail={
                  <>
                    <dl className="kv">
                      <dt>When</dt>
                      <dd>{fmtDateTime(e.time)}</dd>
                      <dt>Agent</dt>
                      <dd className="wrap">{e.agentName}</dd>
                      <dt>Project</dt>
                      <dd className="wrap">{e.project}</dd>
                      <dt>Call</dt>
                      <dd className="prose">{schema ? 'schema lookup' : 'Request'}</dd>
                      {!schema && (
                        <>
                          <dt>Purpose</dt>
                          <dd className="wrap prose">{e.purpose}</dd>
                        </>
                      )}
                      {note && (
                        <>
                          <dt>{e.error ? 'Error' : 'Note'}</dt>
                          <dd className={`wrap ${e.status === 'cancelled' ? 'muted' : 'err'}`}>{note}</dd>
                        </>
                      )}
                    </dl>
                    {schema ? (
                      <pre>
                        <code>{sql}</code>
                      </pre>
                    ) : (
                      <SqlBlock sql={sql} />
                    )}
                  </>
                }
              >
                <td className="nowrap" title={fmtDateTime(e.time)}>
                  {fmtTime(e.time)}
                </td>
                <td>
                  <div className="trunc cell-agent" title={`${e.agentName} · ${e.project}`}>
                    {e.agentName} <span className="muted mono">· {baseName(e.project)}</span>
                  </div>
                </td>
                <td>
                  <div className="trunc mono cell-conn" title={e.connection}>
                    {e.connection}
                  </div>
                </td>
                <td className={`fill${schema ? ' muted' : ''}`}>
                  <div className="trunc" title={schema ? 'schema lookup' : e.purpose}>
                    {schema ? 'schema lookup' : e.purpose}
                  </div>
                </td>
                <td className="fill mono">
                  <div className="trunc" title={sql}>
                    {schema ? sql : <SqlText sql={sql.slice(0, CELL_SQL_MAX)} />}
                  </div>
                </td>
                <td className="nowrap">
                  <span key={e.status} className={`pill ${STATUS_CLASS[e.status]}`} title={note || undefined}>
                    {STATUS_LABEL[e.status]}
                  </span>
                </td>
                <td className="nowrap">{fmtDuration(e.durationMs)}</td>
                <td className="nowrap">
                  {e.rows == null ? '—' : schema ? `${fmtCount(e.rows)} ${e.rows === 1 ? 'match' : 'matches'}` : fmtCount(e.rows)}
                </td>
              </LogRow>
            )
          })}
        </tbody>
      </table>
      <LogFooter {...log} empty={shown.length === 0} />
      <div className="cap">
        Every call an Agent makes, each Request and each schema lookup. No result rows are stored. Keeps the latest{' '}
        {fmtCount(limit)} entries. Click a row for its details.
      </div>
    </>
  )
}

// LogRow is one log entry on one line; clicking it, or Enter on it, shows
// `detail` in a row underneath. `cols` counts the cells in `children`.
export function LogRow({ cols, children, detail }: { cols: number; children: ReactNode; detail: ReactNode }) {
  const [open, setOpen] = useState(false)
  const onKey = (ev: KeyboardEvent) => {
    if (ev.target !== ev.currentTarget || (ev.key !== 'Enter' && ev.key !== ' ')) return
    ev.preventDefault()
    setOpen(!open)
  }
  return (
    <>
      <tr
        className={`log-row${open ? ' open' : ''}`}
        tabIndex={0}
        aria-expanded={open}
        // A drag that selects text is not a click on the row.
        onClick={() => window.getSelection()?.type !== 'Range' && setOpen(!open)}
        onKeyDown={onKey}
      >
        <td className="chev-cell">
          <IconChevronRight size={14} className="chev-ico" />
        </td>
        {children}
      </tr>
      {open && (
        <tr className="log-detail">
          <td colSpan={cols + 1}>{detail}</td>
        </tr>
      )}
    </>
  )
}

export function LogFooter({
  loading,
  error,
  hasMore,
  loadMore,
  empty,
}: {
  loading: boolean
  error: string
  hasMore: boolean
  loadMore: () => void
  empty: boolean
}) {
  return (
    <div className="log-foot">
      {error && <span className="err wrap">{error}</span>}
      {!loading && empty && <span className="muted">Nothing here yet.</span>}
      {hasMore && (
        <button className="btn" disabled={loading} onClick={loadMore}>
          <IconChevronsDown size={14} />
          {loading ? 'Loading…' : 'Load more'}
        </button>
      )}
      {loading && !hasMore && <span className="muted">Loading…</span>}
    </div>
  )
}
