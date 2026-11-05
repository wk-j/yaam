import { baseName, fmtCount, fmtDateTime, fmtDuration, fmtTime } from '../format'
import { useLog } from '../live'
import { CELL_SQL_MAX, SqlBlock, SqlText } from '../sql'
import type { ExecutionLogEntry } from '../types'
import { LogFooter, LogRow } from './AgentLog'

export function ExecutionLog({ limit }: { limit: number }) {
  const log = useLog<ExecutionLogEntry>('/api/execution-log', 'execution-log')

  return (
    <>
      <table className="log">
        <thead>
          <tr>
            <th className="chev-cell">
              <span className="sr-only">Details</span>
            </th>
            <th>Time</th>
            <th>Ran by</th>
            <th>Connection</th>
            <th>SQL</th>
            <th>Time taken</th>
            <th>Rows</th>
            <th>Status</th>
          </tr>
        </thead>
        <tbody>
          {log.entries.map((e) => (
            <LogRow
              key={e.id}
              cols={7}
              detail={
                <>
                  <dl className="kv">
                    <dt>When</dt>
                    <dd>{fmtDateTime(e.time)}</dd>
                    <dt>Ran by</dt>
                    <dd className="wrap">
                      {e.by === 'user' ? `User${e.source === 'grid' ? ' · grid edit' : ''}` : `Agent ${e.agentName}`}
                    </dd>
                    {e.by === 'agent' && (
                      <>
                        <dt>Project</dt>
                        <dd className="wrap">{e.project}</dd>
                      </>
                    )}
                    {e.error && (
                      <>
                        <dt>Error</dt>
                        <dd className="err wrap">{e.error}</dd>
                      </>
                    )}
                  </dl>
                  <SqlBlock sql={e.sql} />
                </>
              }
            >
              <td className="nowrap" title={fmtDateTime(e.time)}>
                {fmtTime(e.time)}
              </td>
              <td>
                {e.by === 'user' ? (
                  <div className="trunc cell-agent">
                    User{e.source === 'grid' && <span className="muted"> · grid edit</span>}
                  </div>
                ) : (
                  <div className="trunc cell-agent" title={`${e.agentName} · ${e.project}`}>
                    {e.agentName} <span className="muted mono">· {baseName(e.project)}</span>
                  </div>
                )}
              </td>
              <td>
                <div className="trunc mono cell-conn" title={e.connection}>
                  {e.connection}
                </div>
              </td>
              <td className="fill mono">
                <div className="trunc" title={e.sql}>
                  <SqlText sql={e.sql.slice(0, CELL_SQL_MAX)} />
                </div>
              </td>
              <td className="nowrap">{fmtDuration(e.durationMs)}</td>
              <td className="nowrap">{e.rows == null ? '—' : fmtCount(e.rows)}</td>
              <td className="nowrap">
                {e.error ? (
                  <span className="pill fail" title={e.error}>
                    error
                  </span>
                ) : (
                  <span className="pill run">ok</span>
                )}
              </td>
            </LogRow>
          ))}
        </tbody>
      </table>
      <LogFooter {...log} empty={log.entries.length === 0} />
      <div className="cap">
        All SQL that ran for a User or an Agent, including grid edits. Not the SQL yaam runs on its own to look up
        schema. No result rows are stored. Keeps the latest {fmtCount(limit)} entries. Click a row for its details.
      </div>
    </>
  )
}
