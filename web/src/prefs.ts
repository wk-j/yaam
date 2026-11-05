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
