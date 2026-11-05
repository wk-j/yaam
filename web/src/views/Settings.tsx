import { useEffect, useState, type FormEvent } from 'react'
import { api } from '../api'
import { notificationsSupported, notificationsWanted, setNotificationsWanted, turnOnNotifications } from '../notify'
import type { Settings } from '../types'
import { IconAlert, IconBell, IconBellOff, IconCheck, IconSave } from '../icons'

const MIN = 1000
const MAX = 10000000

export function SettingsView({ settings }: { settings: Settings }) {
  const [agentLog, setAgentLog] = useState(String(settings.agentLogLimit))
  const [execLog, setExecLog] = useState(String(settings.executionLogLimit))
  const [busy, setBusy] = useState(false)
  const [message, setMessage] = useState<{ ok: boolean; text: string } | null>(null)
  const [notify, setNotify] = useState(notificationsWanted)
  const [notifyNote, setNotifyNote] = useState('')

  // Follow changes saved elsewhere (another tab), which arrive with `state`.
  useEffect(() => {
    setAgentLog(String(settings.agentLogLimit))
    setExecLog(String(settings.executionLogLimit))
  }, [settings.agentLogLimit, settings.executionLogLimit])

  const parse = (s: string) => Number(s.replace(/[,_\s]/g, ''))

  const save = async (e: FormEvent) => {
    e.preventDefault()
    const a = parse(agentLog)
    const x = parse(execLog)
    for (const n of [a, x]) {
      if (!Number.isInteger(n) || n < MIN || n > MAX) {
        setMessage({ ok: false, text: `Each limit must be a whole number from ${MIN.toLocaleString('en-US')} to ${MAX.toLocaleString('en-US')}.` })
        return
      }
    }
    setBusy(true)
    setMessage(null)
    try {
      await api('PUT', '/api/settings', { agentLogLimit: a, executionLogLimit: x })
      setMessage({ ok: true, text: 'Saved. Older entries beyond the limits were removed.' })
    } catch (err) {
      setMessage({ ok: false, text: (err as Error).message })
    } finally {
      setBusy(false)
    }
  }

  const changeNotify = async (on: boolean) => {
    setNotifyNote('')
    if (!on) {
      setNotificationsWanted(false)
      setNotify(false)
      return
    }
    const p = await turnOnNotifications()
    setNotify(p === 'granted')
    if (p !== 'granted') setNotifyNote('The browser did not allow notifications for this page.')
  }

  return (
    <form className="form settings" onSubmit={save}>
      <label htmlFor="s-agent">Agent Log keeps</label>
      <input id="s-agent" value={agentLog} inputMode="numeric" onChange={(e) => setAgentLog(e.target.value)} />
      <span className="hint">latest entries; the oldest go first</span>
      <label htmlFor="s-exec">Execution Log keeps</label>
      <input id="s-exec" value={execLog} inputMode="numeric" onChange={(e) => setExecLog(e.target.value)} />
      <span className="hint">latest entries; the oldest go first</span>
      <span />
      <div className="row">
        <button type="submit" className="btn pri" disabled={busy}>
          <IconSave size={14} />
          Save limits
        </button>
        {message && (
          <span className={`small with-ico wrap ${message.ok ? 'good' : 'err'}`}>
            {message.ok ? <IconCheck size={14} /> : <IconAlert size={14} />}
            <span>{message.text}</span>
          </span>
        )}
      </div>
      <label htmlFor="s-notify" className="with-ico">
        {notify ? <IconBell size={14} /> : <IconBellOff size={14} />}
        Notifications
      </label>
      <select
        id="s-notify"
        value={notify ? 'on' : 'off'}
        disabled={!notificationsSupported}
        onChange={(e) => changeNotify(e.target.value === 'on')}
      >
        <option value="on">Browser notification when a Request waits</option>
        <option value="off">Off</option>
      </select>
      {notifyNote && (
        <span className="hint err with-ico">
          <IconAlert size={13} />
          <span>{notifyNote}</span>
        </span>
      )}
    </form>
  )
}
