import { useState, type FormEvent } from 'react'
import { api, enc } from '../api'
import { mcpLine } from '../format'
import { Modal } from '../Modal'
import {
  CopyButton,
  IconAlert,
  IconButton,
  IconChevronRight,
  IconEdit,
  IconPlug,
  IconPlus,
  IconStatusError,
  IconStatusOk,
  IconStatusUnknown,
  IconTrash,
} from '../icons'
import type { Connection, SslMode } from '../types'
import { followRename } from '../prefs'

const NAME_RE = /^[a-z0-9][a-z0-9_-]{0,62}$/
const SSLMODES: SslMode[] = ['disable', 'require', 'verify-ca', 'verify-full']

export function Connections({ connections }: { connections: Connection[] }) {
  const [editing, setEditing] = useState<Connection | 'new' | null>(null)

  return (
    <>
      <div className="view-actions">
        <button data-testid="connection-add" className="btn pri sm" onClick={() => setEditing('new')}>
          <IconPlus size={14} />
          Add Connection
        </button>
      </div>
      {connections.length > 0 && (
        <ul className="conns">
          {connections.map((c) => (
            <ConnectionRow key={c.name} c={c} onEdit={() => setEditing(c)} />
          ))}
        </ul>
      )}
      {connections.length === 0 && <p className="muted">No Connection yet. Add one to start.</p>}
      {editing && (
        <ConnectionDialog
          existing={editing === 'new' ? null : editing}
          taken={connections.map((c) => c.name)}
          onClose={() => setEditing(null)}
        />
      )}
    </>
  )
}

export function StatusIcon({ c }: { c: Connection }) {
  const label = c.status === 'ok' ? 'Connected' : c.status === 'error' ? 'Cannot connect' : 'Not tried yet'
  return (
    <span className={`status-ico ${c.status}`} role="img" aria-label={label} title={label}>
      {c.status === 'ok' ? <IconStatusOk /> : c.status === 'error' ? <IconStatusError /> : <IconStatusUnknown />}
    </span>
  )
}

function ConnectionRow({ c, onEdit }: { c: Connection; onEdit: () => void }) {
  const [open, setOpen] = useState(false)
  const [testing, setTesting] = useState(false)
  const [test, setTest] = useState<{ ok: boolean; error: string } | null>(null)
  const [error, setError] = useState('')

  const runTest = async () => {
    setTesting(true)
    setTest(null)
    setError('')
    try {
      setTest(await api<{ ok: boolean; error: string }>('POST', `/api/connections/${enc(c.name)}/test`))
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setTesting(false)
    }
  }

  const summary = `${c.role} @ ${c.host}:${c.port} / ${c.database}`
  const statusError = c.status === 'error' ? c.statusError : ''
  const detailId = `conn-detail-${c.name}`
  // A failed test also sets the Connection's error, shown under the summary.
  const testNote = test && (test.ok ? 'Test passed: connected and ran SELECT 1.' : test.error !== statusError ? `Test failed: ${test.error}` : '')

  return (
    <li data-testid="connection-row" className={`crow${open ? ' open' : ''}`}>
      <div className="crow-main">
        <StatusIcon c={c} />
        <div className="crow-text" onClick={() => setOpen(!open)}>
          <div className="crow-line">
            <strong className="mono crow-name" title={c.name}>
              {c.name}
            </strong>
            <span className="mono muted trunc" title={summary}>
              {summary}
            </span>
          </div>
          {statusError && !open && (
            <div className="crow-err err trunc" title={statusError}>
              {statusError}
            </div>
          )}
          {testNote && <div data-testid="connection-test-result" className={`small trunc ${test?.ok ? 'good' : 'err'}`} title={testNote}>{testNote}</div>}
          {error && <div className="err small wrap">{error}</div>}
        </div>
        <div className="crow-act">
          <IconButton data-testid="connection-test" label={testing ? 'Testing…' : 'Test Connection'} className={testing ? 'busy' : ''} disabled={testing} onClick={runTest}>
            <IconPlug />
          </IconButton>
          <CopyButton text={mcpLine(c.name)} label="Copy .mcp.json entry" onError={setError} />
          <IconButton label={`Edit ${c.name}`} onClick={onEdit}>
            <IconEdit />
          </IconButton>
          <IconButton
            label={open ? 'Hide details' : 'Show details'}
            className="chev"
            aria-expanded={open}
            aria-controls={detailId}
            onClick={() => setOpen(!open)}
          >
            <IconChevronRight />
          </IconButton>
        </div>
      </div>
      {open && (
        <div className="crow-detail" id={detailId}>
          <dl className="kv">
            <dt>Address</dt>
            <dd className="wrap">
              {c.host}:{c.port}/{c.database}
            </dd>
            <dt>Role</dt>
            <dd className="wrap">{c.role}</dd>
            <dt>sslmode</dt>
            <dd>{c.sslmode}</dd>
            <dt>Agent time limit</dt>
            <dd>{c.agentTimeoutSeconds} s</dd>
            <dt>Password</dt>
            <dd className="muted">{c.hasPassword ? 'saved in Keychain' : 'none'}</dd>
            {statusError && (
              <>
                <dt>Error</dt>
                <dd className="err wrap">{statusError}</dd>
              </>
            )}
          </dl>
          <div className="muted small mcp-head">
            <span>
              Put this in a project's <span className="mono">.mcp.json</span>:
            </span>
          </div>
          <div className="snippet">
            <pre>
              <code>{mcpLine(c.name)}</code>
            </pre>
            <CopyButton text={mcpLine(c.name)} label="Copy .mcp.json entry" onError={setError} />
          </div>
        </div>
      )}
    </li>
  )
}

function ConnectionDialog({
  existing,
  taken,
  onClose,
}: {
  existing: Connection | null
  taken: string[]
  onClose: () => void
}) {
  const [name, setName] = useState(existing?.name ?? '')
  const [host, setHost] = useState(existing?.host ?? 'localhost')
  const [port, setPort] = useState(String(existing?.port ?? 5432))
  const [database, setDatabase] = useState(existing?.database ?? '')
  const [role, setRole] = useState(existing?.role ?? '')
  const [password, setPassword] = useState('')
  const [removePassword, setRemovePassword] = useState(false)
  const [sslmode, setSslmode] = useState<SslMode>(existing?.sslmode ?? 'disable')
  const [timeout, setTimeoutSecs] = useState(String(existing?.agentTimeoutSeconds ?? 30))
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  // The name the Connection has in yaam now; it changes once a rename succeeds,
  // so a retry after a failed save does not rename again.
  const [savedName, setSavedName] = useState<string | null>(existing?.name ?? null)

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    const portN = Number(port)
    const timeoutN = Number(timeout)
    if (!NAME_RE.test(name)) return setError('Name: lowercase letters, digits, "-" and "_", starting with a letter or digit.')
    const renameFrom = savedName !== null && name !== savedName ? savedName : null
    if ((savedName === null || renameFrom !== null) && taken.includes(name)) return setError(`A Connection named ${name} already exists.`)
    if (!host.trim()) return setError('Host is required.')
    if (!Number.isInteger(portN) || portN < 1 || portN > 65535) return setError('Port must be 1–65535.')
    if (!database.trim()) return setError('Database is required.')
    if (!role.trim()) return setError('Role is required.')
    if (!Number.isInteger(timeoutN) || timeoutN < 1) return setError('Agent time limit must be a whole number of seconds.')

    const body: Record<string, unknown> = {
      host: host.trim(),
      port: portN,
      database: database.trim(),
      role: role.trim(),
      sslmode,
      agentTimeoutSeconds: timeoutN,
    }
    // Absent password keeps the saved one; "" removes it.
    if (removePassword) body.password = ''
    else if (password !== '') body.password = password

    setBusy(true)
    setError('')
    try {
      // Rename first: yaam moves the config and the Keychain password to the
      // new name in one step and refuses a name that is already taken.
      if (renameFrom !== null) {
        await api('POST', `/api/connections/${enc(renameFrom)}/rename`, { name })
        setSavedName(name)
        followRename(renameFrom, name)
      }
      await api('PUT', `/api/connections/${enc(name)}`, body)
      api('POST', `/api/connections/${enc(name)}/test`).catch(() => {})
      onClose()
    } catch (err) {
      setError((err as Error).message)
      setBusy(false)
    }
  }

  const remove = async () => {
    if (savedName === null) return
    if (!window.confirm(`Delete Connection ${savedName}? Its saved password is removed from the Keychain too.`)) return
    setBusy(true)
    setError('')
    try {
      await api('DELETE', `/api/connections/${enc(savedName)}`)
      onClose()
    } catch (err) {
      setError((err as Error).message)
      setBusy(false)
    }
  }

  return (
    <Modal onClose={onClose}>
      <form onSubmit={submit}>
        <h3 className="trunc">{existing ? `Edit ${existing.name}` : 'Add Connection'}</h3>
        <div className="form">
          <label htmlFor="c-name">Name</label>
          <input
            id="c-name"
            value={name}
            spellCheck={false}
            placeholder="shop-dev"
            autoFocus={!existing}
            onChange={(e) => setName(e.target.value)}
          />
          <span className="hint">Projects name it in .mcp.json: yaam mcp &lt;name&gt;</span>
          <label htmlFor="c-host">Host</label>
          <input id="c-host" value={host} placeholder="localhost" onChange={(e) => setHost(e.target.value)} />
          <label htmlFor="c-port">Port</label>
          <input id="c-port" value={port} inputMode="numeric" placeholder="5432" onChange={(e) => setPort(e.target.value)} />
          <label htmlFor="c-db">Database</label>
          <input id="c-db" value={database} placeholder="shop" onChange={(e) => setDatabase(e.target.value)} />
          <label htmlFor="c-role">Role</label>
          <input id="c-role" value={role} placeholder="shop_agent" onChange={(e) => setRole(e.target.value)} />
          <label htmlFor="c-pw">Password</label>
          <input
            id="c-pw"
            type="password"
            autoComplete="new-password"
            value={password}
            disabled={removePassword}
            placeholder={existing?.hasPassword ? 'unchanged' : 'none'}
            onChange={(e) => setPassword(e.target.value)}
          />
          <span className="hint">
            Stored in the macOS Keychain. yaam never shows it again.
            {existing?.hasPassword && ' Leave empty to keep the saved password.'}
          </span>
          {existing?.hasPassword && (
            <label className="check">
              <input
                type="checkbox"
                checked={removePassword}
                onChange={(e) => {
                  setRemovePassword(e.target.checked)
                  if (e.target.checked) setPassword('')
                }}
              />
              Remove password
            </label>
          )}
          <label htmlFor="c-ssl">sslmode</label>
          <select id="c-ssl" value={sslmode} onChange={(e) => setSslmode(e.target.value as SslMode)}>
            {SSLMODES.map((m) => (
              <option key={m}>{m}</option>
            ))}
          </select>
          <label htmlFor="c-timeout">Agent time limit</label>
          <input
            id="c-timeout"
            value={timeout}
            inputMode="numeric"
            placeholder="30"
            onChange={(e) => setTimeoutSecs(e.target.value)}
          />
          <span className="hint">seconds an Agent Request may run</span>
        </div>
        {error && (
          <p className="err small wrap with-ico">
            <IconAlert size={14} />
            <span>{error}</span>
          </p>
        )}
        <div className="dlg-act">
          {existing && (
            <button type="button" className="btn bad" disabled={busy} onClick={remove}>
              <IconTrash size={14} />
              Delete
            </button>
          )}
          <span className="sp" />
          <button type="button" className="btn" onClick={onClose}>
            Cancel
          </button>
          <button data-testid="connection-save" type="submit" className="btn pri" disabled={busy}>
            {existing ? 'Save and test' : 'Add and test'}
          </button>
        </div>
      </form>
    </Modal>
  )
}
