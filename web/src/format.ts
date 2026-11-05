const pad = (n: number) => String(n).padStart(2, '0')

// fmtDateTime shows a local date and time, such as 2026-11-05 14:03:09.
export function fmtDateTime(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

// fmtTime leaves out the date for a time today.
export function fmtTime(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  const full = fmtDateTime(iso)
  return d.toDateString() === new Date().toDateString() ? full.slice(11) : full
}

export function fmtDuration(ms: number | null | undefined): string {
  if (ms == null) return '—'
  if (ms < 10) return `${Math.round(ms * 10) / 10} ms`
  if (ms < 1000) return `${Math.round(ms)} ms`
  return `${(ms / 1000).toFixed(1)} s`
}

export function fmtCount(n: number): string {
  return n.toLocaleString('en-US')
}

export function plural(n: number, one: string, many: string): string {
  return `${fmtCount(n)} ${n === 1 ? one : many}`
}

export function fmtWaiting(sinceIso: string, now: number): string {
  const s = Math.max(0, Math.floor((now - new Date(sinceIso).getTime()) / 1000))
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const rest = pad(s % 60)
  return h > 0 ? `${h}:${pad(m)}:${rest}` : `${m}:${rest}`
}

export function baseName(p: string): string {
  const parts = p.split('/').filter(Boolean)
  return parts[parts.length - 1] ?? p
}

export function mcpLine(name: string): string {
  return `"${name}": { "command": "yaam", "args": ["mcp", "${name}"] }`
}
