import { useState } from 'react'

// The User picks System, Light or Dark; System follows macOS. The effective
// theme is set as <html data-theme="light|dark">, which styles.css reads.
export type ThemePref = 'system' | 'light' | 'dark'

const KEY = 'yaam.theme'
const osLight = window.matchMedia('(prefers-color-scheme: light)')

function readPref(): ThemePref {
  const v = localStorage.getItem(KEY)
  return v === 'light' || v === 'dark' ? v : 'system'
}

function apply(pref: ThemePref) {
  const light = pref === 'light' || (pref === 'system' && osLight.matches)
  document.documentElement.dataset.theme = light ? 'light' : 'dark'
}

// initTheme applies the saved theme before the first render and keeps
// following the OS while the preference is System.
export function initTheme() {
  apply(readPref())
  osLight.addEventListener('change', () => apply(readPref()))
}

export function useTheme(): [ThemePref, (pref: ThemePref) => void] {
  const [pref, setPref] = useState(readPref)
  const set = (next: ThemePref) => {
    if (next === 'system') localStorage.removeItem(KEY)
    else localStorage.setItem(KEY, next)
    // Colours fade to the new theme instead of flashing (styles.css .theme-anim).
    const root = document.documentElement
    root.classList.add('theme-anim')
    window.setTimeout(() => root.classList.remove('theme-anim'), 300)
    apply(next)
    setPref(next)
  }
  return [pref, set]
}
