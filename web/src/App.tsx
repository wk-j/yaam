import { useEffect, useRef, useState } from 'react'
import { streamEvents } from './api'
import { notifyPending } from './notify'
import { logEvents } from './live'
import type { AppState, Connection } from './types'
import {
  IconAgentLog,
  IconApprovals,
  IconBrand,
  IconButton,
  IconDatabase,
  IconEditor,
  IconExecutionLog,
  IconMonitor,
  IconMoon,
  IconPanelCollapse,
  IconPanelExpand,
  IconSettings,
  IconSun,
} from './icons'
import { useTheme, type ThemePref } from './theme'
import { RENAMED_EVENT, SELECTED_CONNECTION_KEY, useStoredFlag } from './prefs'
import { Editor } from './views/Editor'
import { Approvals, NotifyToggle } from './views/Approvals'
import { AgentLog } from './views/AgentLog'
import { ExecutionLog } from './views/ExecutionLog'
import { Connections } from './views/Connections'
import { SettingsView } from './views/Settings'

const VIEWS = [
  { route: 'editor', title: 'Editor', Icon: IconEditor },
  { route: 'approvals', title: 'Approvals', Icon: IconApprovals },
  { route: 'agent-log', title: 'Agent Log', Icon: IconAgentLog },
  { route: 'execution-log', title: 'Execution Log', Icon: IconExecutionLog },
  { route: 'connections', title: 'Connections', Icon: IconDatabase },
  { route: 'settings', title: 'Settings', Icon: IconSettings },
] as const

type Route = (typeof VIEWS)[number]['route']

function readRoute(): Route {
  const r = location.hash.replace(/^#\/?/, '')
  return VIEWS.some((v) => v.route === r) ? (r as Route) : 'editor'
}

export function NoToken({ expired = false }: { expired?: boolean }) {
  return (
    <div className="notoken">
      <h1>yaam</h1>
      {expired && <p className="muted">This page's token is no longer valid; yaam may have restarted.</p>}
      <p>
        Open yaam with <code>yaam open</code>
      </p>
    </div>
  )
}

export function App() {
  const [state, setState] = useState<AppState | null>(null)
  const [live, setLive] = useState(false)
  const [route, setRoute] = useState<Route>(readRoute)
  const [conn, setConnRaw] = useState<string>(() => localStorage.getItem(SELECTED_CONNECTION_KEY) ?? '')
  const seenPending = useRef<Set<string> | null>(null)
  const [narrow, toggleNarrow] = useStoredFlag('yaam.sidebar.collapsed')

  useEffect(() => {
    const onHash = () => setRoute(readRoute())
    window.addEventListener('hashchange', onHash)
    return () => window.removeEventListener('hashchange', onHash)
  }, [])

  // A renamed Connection stays selected under its new name.
  useEffect(() => {
    const onRename = (e: Event) => {
      const { from, to } = (e as CustomEvent<{ from: string; to: string }>).detail
      setConnRaw((cur) => (cur === from ? to : cur))
    }
    window.addEventListener(RENAMED_EVENT, onRename)
    return () => window.removeEventListener(RENAMED_EVENT, onRename)
  }, [])

  useEffect(
    () =>
      streamEvents((type, data) => {
        if (type === 'state') {
          const s = data as AppState
          const seen = seenPending.current
          if (seen) for (const p of s.pending) if (!seen.has(p.id)) notifyPending(p)
          seenPending.current = new Set(s.pending.map((p) => p.id))
          setState(s)
        } else if (type === 'agent-log' || type === 'execution-log') {
          logEvents.dispatchEvent(new CustomEvent(type, { detail: data }))
        }
      }, setLive),
    [],
  )

  const setConn = (name: string) => {
    localStorage.setItem(SELECTED_CONNECTION_KEY, name)
    setConnRaw(name)
  }

  // Fall back to the first Connection when the remembered one is gone.
  const connections = state?.connections ?? []
  const current = connections.find((c) => c.name === conn) ?? connections[0]

  const go = (r: Route) => {
    location.hash = `#/${r}`
  }

  if (!state) {
    return <div className="notoken muted">{live ? 'Loading…' : 'Connecting to yaam…'}</div>
  }

  const title = VIEWS.find((v) => v.route === route)!.title
  const agents = state.agents.length

  return (
    <div className={narrow ? 'app side-collapsed' : 'app'}>
      <aside className="side">
        <div className="brand">
          <IconBrand size={22} />
          <div className="brand-text">
            yaam<small>{location.host}</small>
          </div>
          <IconButton label={narrow ? 'Expand sidebar' : 'Collapse sidebar'} className="side-toggle" onClick={toggleNarrow}>
            {narrow ? <IconPanelExpand /> : <IconPanelCollapse />}
          </IconButton>
        </div>
        <nav className="nav">
          {VIEWS.map(({ route: r, title: t, Icon }) => (
            <button key={r} data-testid={`nav-${r}`} aria-current={route === r} title={narrow ? t : undefined} onClick={() => go(r)}>
              <Icon />
              <span className="nav-label">{t}</span>
              {r === 'approvals' && state.pending.length > 0 && (
                <span key={state.pending.length} className="badge">
                  {state.pending.length}
                </span>
              )}
            </button>
          ))}
        </nav>
        <h4>Connections</h4>
        <div className="side-conns">
          {connections.map((c) => (
            <button
              key={c.name}
              data-testid="sidebar-connection"
              className="conn"
              aria-current={route === 'editor' && current?.name === c.name}
              title={c.status === 'error' ? `${c.name}: ${c.statusError}` : `${c.name} · ${c.host}:${c.port}/${c.database}`}
              onClick={() => {
                setConn(c.name)
                go('editor')
              }}
            >
              <StatusDot c={c} />
              <span className="trunc conn-name">{c.name}</span>
            </button>
          ))}
          {connections.length === 0 && <div className="conn muted">none yet</div>}
        </div>
        <div className="foot">
          <div className="foot-row">
            <span className="trunc foot-text" title={state.agents.map((a) => `${a.agentName} · ${a.project} · ${a.connection}`).join('\n')}>
              {agents === 1 ? '1 Agent connected' : `${agents} Agents connected`}
            </span>
            <ThemeToggle />
          </div>
          {!live && <div className="warn">reconnecting to yaam…</div>}
        </div>
      </aside>

      <section className="mainp">
        <div className="bar">
          <h2>{title}</h2>
          {route === 'approvals' && <NotifyToggle />}
          <span className="sp" />
          {route === 'editor' && connections.length > 0 && (
            <label className="muted bar-ctx">
              Connection{' '}
              <select data-testid="editor-connection" className="mono" value={current?.name ?? ''} onChange={(e) => setConn(e.target.value)}>
                {connections.map((c) => (
                  <option key={c.name} value={c.name}>
                    {c.name}
                  </option>
                ))}
              </select>
            </label>
          )}
        </div>
        <div className="view editor-view" hidden={route !== 'editor'}>
          <Editor connection={current} />
        </div>
        {route === 'approvals' && (
          <div className="view">
            <Approvals pending={state.pending} connections={connections} />
          </div>
        )}
        {route === 'agent-log' && (
          <div className="view">
            <AgentLog limit={state.settings.agentLogLimit} />
          </div>
        )}
        {route === 'execution-log' && (
          <div className="view">
            <ExecutionLog limit={state.settings.executionLogLimit} />
          </div>
        )}
        {route === 'connections' && (
          <div className="view">
            <Connections connections={connections} />
          </div>
        )}
        {route === 'settings' && (
          <div className="view">
            <SettingsView settings={state.settings} />
          </div>
        )}
      </section>
    </div>
  )
}

export function StatusDot({ c }: { c: Connection }) {
  return <span className={`dot ${c.status}`} title={c.status === 'error' ? c.statusError : c.status} />
}

const THEMES: { pref: ThemePref; label: string; Icon: typeof IconSun }[] = [
  { pref: 'system', label: 'System', Icon: IconMonitor },
  { pref: 'light', label: 'Light', Icon: IconSun },
  { pref: 'dark', label: 'Dark', Icon: IconMoon },
]

// ThemeToggle steps through System → Light → Dark.
function ThemeToggle() {
  const [pref, setPref] = useTheme()
  const i = THEMES.findIndex((x) => x.pref === pref)
  const { label, Icon } = THEMES[i]
  const next = THEMES[(i + 1) % THEMES.length]
  return (
    <IconButton label={`Theme: ${label} (switch to ${next.label})`} className="theme-toggle" onClick={() => setPref(next.pref)}>
      <Icon />
    </IconButton>
  )
}
