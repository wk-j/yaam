const TOKEN_KEY = 'yaam.token'

// takeToken moves `#token=…` from the address bar into sessionStorage and
// returns the token for this tab, or null when there is none.
export function takeToken(): string | null {
  const m = /^#token=([^&]+)/.exec(location.hash)
  if (m) {
    sessionStorage.setItem(TOKEN_KEY, decodeURIComponent(m[1]))
    history.replaceState(null, '', location.pathname + location.search + '#/editor')
  }
  return sessionStorage.getItem(TOKEN_KEY)
}

let token = ''
let onUnauthorized: () => void = () => {}

export function setToken(t: string, unauthorized: () => void) {
  token = t
  onUnauthorized = unauthorized
}

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

export async function api<T = void>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = { Authorization: `Bearer ${token}` }
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  const res = await fetch(path, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  if (res.status === 401) onUnauthorized()
  if (!res.ok) {
    let message = `${res.status} ${res.statusText}`
    try {
      const j = (await res.json()) as { error?: string }
      if (j.error) message = j.error
    } catch {
      // not JSON; keep the status line
    }
    throw new ApiError(res.status, message)
  }
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

export const enc = encodeURIComponent

export type EventHandler = (type: string, data: unknown) => void

// streamEvents reads /api/events with fetch (so it can send the bearer token)
// and reconnects with backoff until the returned stop function is called.
export function streamEvents(onEvent: EventHandler, onStatus: (connected: boolean) => void): () => void {
  let stopped = false
  let ctrl: AbortController | null = null
  let timer = 0
  let delay = 1000

  const connect = async () => {
    ctrl = new AbortController()
    try {
      const res = await fetch('/api/events', {
        headers: { Authorization: `Bearer ${token}`, Accept: 'text/event-stream' },
        signal: ctrl.signal,
      })
      if (res.status === 401) {
        onUnauthorized()
        return
      }
      if (!res.ok || !res.body) throw new Error(`events: ${res.status}`)
      onStatus(true)
      delay = 1000
      const reader = res.body.pipeThrough(new TextDecoderStream()).getReader()
      let buf = ''
      for (;;) {
        const { value, done } = await reader.read()
        if (done) break
        buf += value.replace(/\r\n?/g, '\n')
        let i: number
        while ((i = buf.indexOf('\n\n')) >= 0) {
          dispatch(buf.slice(0, i), onEvent)
          buf = buf.slice(i + 2)
        }
      }
    } catch {
      // network error or abort; fall through to reconnect
    }
    if (stopped) return
    onStatus(false)
    timer = window.setTimeout(connect, delay)
    delay = Math.min(delay * 2, 30000)
  }

  connect()
  return () => {
    stopped = true
    clearTimeout(timer)
    ctrl?.abort()
  }
}

function dispatch(block: string, onEvent: EventHandler) {
  let type = 'message'
  const data: string[] = []
  for (const line of block.split('\n')) {
    if (line === '' || line.startsWith(':')) continue
    const c = line.indexOf(':')
    const field = c < 0 ? line : line.slice(0, c)
    let value = c < 0 ? '' : line.slice(c + 1)
    if (value.startsWith(' ')) value = value.slice(1)
    if (field === 'event') type = value
    else if (field === 'data') data.push(value)
  }
  if (data.length === 0) return
  try {
    onEvent(type, JSON.parse(data.join('\n')))
  } catch {
    // ignore a malformed event
  }
}
