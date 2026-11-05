import { useCallback, useEffect, useState } from 'react'
import { api } from './api'

// logEvents carries `agent-log` and `execution-log` entries from /api/events
// to whichever log view is open.
export const logEvents = new EventTarget()

const PAGE = 200

type Entry = { id: number }

// merge combines entries by id (later lists win) and keeps them newest first.
function merge<T extends Entry>(older: T[], newer: T[]): T[] {
  const byId = new Map<number, T>()
  for (const e of older) byId.set(e.id, e)
  for (const e of newer) byId.set(e.id, e)
  return [...byId.values()].sort((a, b) => b.id - a.id)
}

// useLog loads a log page by page (newest first) and applies live inserts
// and updates for the given event type.
export function useLog<T extends Entry>(path: string, eventType: 'agent-log' | 'execution-log') {
  const [entries, setEntries] = useState<T[]>([])
  const [hasMore, setHasMore] = useState(false)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    const onEvent = (e: Event) => {
      const entry = (e as CustomEvent<T>).detail
      setEntries((prev) => {
        const i = prev.findIndex((p) => p.id === entry.id)
        if (i >= 0) return prev.map((p, j) => (j === i ? entry : p))
        // An entry older than everything loaded arrives with "Load more".
        if (prev.length > 0 && entry.id < prev[prev.length - 1].id) return prev
        return merge(prev, [entry])
      })
    }
    logEvents.addEventListener(eventType, onEvent)
    api<{ entries: T[] }>('GET', `${path}?limit=${PAGE}`)
      .then((r) => {
        if (cancelled) return
        setEntries((prev) => merge(r.entries, prev))
        setHasMore(r.entries.length === PAGE)
      })
      .catch((err: Error) => !cancelled && setError(err.message))
      .finally(() => !cancelled && setLoading(false))
    return () => {
      cancelled = true
      logEvents.removeEventListener(eventType, onEvent)
    }
  }, [path, eventType])

  const loadMore = useCallback(async () => {
    const last = entries[entries.length - 1]
    if (!last) return
    setLoading(true)
    try {
      const r = await api<{ entries: T[] }>('GET', `${path}?before=${last.id}&limit=${PAGE}`)
      setEntries((prev) => merge(r.entries, prev))
      setHasMore(r.entries.length === PAGE)
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setLoading(false)
    }
  }, [entries, path])

  return { entries, hasMore, loading, error, loadMore }
}
