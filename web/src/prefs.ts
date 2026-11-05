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
