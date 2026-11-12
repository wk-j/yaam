import { useState } from 'react'

// useStoredFlag is an on/off UI preference kept in localStorage under key.
export function useStoredFlag(key: string): [boolean, () => void] {
  const [on, setOn] = useState(() => localStorage.getItem(key) === '1')
  const toggle = () => {
    const next = !on
    if (next) localStorage.setItem(key, '1')
    else localStorage.removeItem(key)
    setOn(next)
  }
  return [on, toggle]
}

// useStoredSet is a set of strings kept in localStorage under key as a JSON
// array; toggle adds an item that is missing and removes one that is there.
export function useStoredSet(key: string): [string[], (item: string) => void] {
  const [items, setItems] = useState<string[]>(() => {
    try {
      const v: unknown = JSON.parse(localStorage.getItem(key) ?? '[]')
      return Array.isArray(v) ? v.filter((x): x is string => typeof x === 'string') : []
    } catch {
      return []
    }
  })
  const toggle = (item: string) => {
    const next = items.includes(item) ? items.filter((x) => x !== item) : [...items, item]
    if (next.length > 0) localStorage.setItem(key, JSON.stringify(next))
    else localStorage.removeItem(key)
    setItems(next)
  }
  return [items, toggle]
}

// Browser state keyed by Connection name: the selected Connection and each
// Connection's editor text.
export const SELECTED_CONNECTION_KEY = 'yaam.connection'
export const sqlDocKey = (conn: string) => `yaam.sql.${conn}`

// RENAMED_EVENT is dispatched on window with detail {from, to} after a
// Connection is renamed, so views holding the old name can follow it.
export const RENAMED_EVENT = 'yaam:connection-renamed'

// followRename moves the browser state of Connection `from` to `to` and
// tells open views.
export function followRename(from: string, to: string) {
  const doc = localStorage.getItem(sqlDocKey(from))
  if (doc !== null) {
    localStorage.setItem(sqlDocKey(to), doc)
    localStorage.removeItem(sqlDocKey(from))
  }
  if (localStorage.getItem(SELECTED_CONNECTION_KEY) === from) localStorage.setItem(SELECTED_CONNECTION_KEY, to)
  window.dispatchEvent(new CustomEvent(RENAMED_EVENT, { detail: { from, to } }))
}
