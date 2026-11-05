import { useEffect, useState } from 'react'
import { ApiError, api, enc } from '../api'
import { baseName, fmtWaiting } from '../format'
import { CopyButton, IconBell, IconBellOff, IconButton, IconCheck, IconChevronDown, IconClock, IconX, InfoTip } from '../icons'
import { notificationsSupported, notificationsWanted, setNotificationsWanted, turnOnNotifications } from '../notify'
import { SqlText } from '../sql'
import type { Connection, Pending } from '../types'

// A statement longer than this shows its first lines until the User asks for all.
const STMT_LINES = 6

export function Approvals({ pending, connections }: { pending: Pending[]; connections: Connection[] }) {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const t = window.setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(t)
  }, [])

  return (
    <>
      {pending.map((p) => (
        <PendingCard
          key={p.id}
          item={p}
          now={now}
          timeLimit={connections.find((c) => c.name === p.connection)?.agentTimeoutSeconds}
        />
      ))}
      {pending.length === 0 && <p className="muted">No Request is waiting.</p>}
    </>
  )
}

// NotifyToggle turns browser notifications for waiting Requests on and off
// from the page header.
export function NotifyToggle() {
  const [, rerender] = useState(0)
  const [note, setNote] = useState('')
  useEffect(() => {
    const on = () => rerender((n) => n + 1)
    window.addEventListener('yaam-notify-pref', on)
    return () => window.removeEventListener('yaam-notify-pref', on)
  }, [])

  const on = notificationsSupported && Notification.permission === 'granted' && notificationsWanted()
  const label = !notificationsSupported
    ? 'This browser does not support notifications'
    : on
      ? 'Browser notifications are on: click to turn them off'
      : 'Browser notifications are off: click to be told when a Request waits'

  const toggle = async () => {
    setNote('')
    if (on) {
      setNotificationsWanted(false)
      return
    }
    const p = await turnOnNotifications()
    if (p !== 'granted') setNote('The browser blocks notifications for this page; allow them in its site settings.')
    rerender((n) => n + 1)
  }

  return (
    <span className="notify">
      <IconButton label={label} className={on ? 'on' : ''} aria-pressed={on} disabled={!notificationsSupported} onClick={toggle}>
        {on ? <IconBell /> : <IconBellOff />}
      </IconButton>
      {note && <span className="small muted trunc" title={note}>{note}</span>}
    </span>
  )
}

function PendingCard({ item, now, timeLimit }: { item: Pending; now: number; timeLimit?: number }) {
  const [busy, setBusy] = useState<'' | 'approve' | 'reject'>('')
  const [error, setError] = useState('')

  const decide = async (action: 'approve' | 'reject') => {
    setBusy(action)
    setError('')
    try {
      await api('POST', `/api/pending/${enc(item.id)}/${action}`)
      // The card disappears with the next `state` event.
    } catch (err) {
      setError(
        err instanceof ApiError && err.status === 404
          ? 'This Request is no longer waiting; the Agent may have stopped waiting.'
          : (err as Error).message,
      )
      setBusy('')
    }
  }

  return (
    <div className="card waiting">
      <div className="who">
        <strong className="trunc">{item.agentName}</strong>
        <span className="sep">·</span>
        <span className="mono muted trunc" title={item.project}>
          {baseName(item.project)}
        </span>
        <span className="sep">·</span>
        <span className="pill mono trunc" title="Connection">
          {item.connection}
        </span>
        <span className="sp" />
        <span className="pill wait with-ico" title={`Waiting since ${new Date(item.time).toLocaleString()}`}>
          <IconClock size={12} />
          {fmtWaiting(item.time, now)}
        </span>
      </div>
      <div className="purpose">
        <b>Purpose</b>
        {item.purpose}
      </div>
      <div className="stmts">
        {item.statements.map((s, i) => (
          <Statement key={i} text={s.text} read={s.kind === 'read'} />
        ))}
        <CopyButton text={item.sql} label="Copy SQL" onError={setError} className="sql-copy" />
      </div>
      <div className="act">
        <button className="btn good" disabled={busy !== ''} onClick={() => decide('approve')}>
          <IconCheck size={14} />
          {busy === 'approve' ? 'Running…' : 'Approve'}
        </button>
        <button className="btn bad" disabled={busy !== ''} onClick={() => decide('reject')}>
          <IconX size={14} />
          Reject
        </button>
        <InfoTip text={`Runs as one transaction${timeLimit ? ` · time limit ${timeLimit} s` : ''}`} />
        {error && <span className="err small wrap">{error}</span>}
      </div>
    </div>
  )
}

function Statement({ text, read }: { text: string; read: boolean }) {
  const [all, setAll] = useState(false)
  const lines = text.split('\n')
  const long = lines.length > STMT_LINES
  return (
    <div className="stmt">
      <span className={`tag ${read ? 'r' : 'w'}`}>{read ? 'READ' : 'WRITE'}</span>
      <div className="stmt-body">
        <code>
          <SqlText sql={long && !all ? lines.slice(0, STMT_LINES).join('\n') : text} />
        </code>
        {long && (
          <button className="linkbtn with-ico" aria-expanded={all} onClick={() => setAll(!all)}>
            <IconChevronDown size={13} className={all ? 'flip' : ''} />
            {all ? 'Show first lines only' : `Show all ${lines.length} lines`}
          </button>
        )}
      </div>
    </div>
  )
}
