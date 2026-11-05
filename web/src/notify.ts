import type { Pending } from './types'

const PREF_KEY = 'yaam.notifications'

export const notificationsSupported = typeof Notification !== 'undefined'

// notificationsWanted is the User's on/off preference, kept in localStorage.
export function notificationsWanted(): boolean {
  return localStorage.getItem(PREF_KEY) === 'on'
}

export function setNotificationsWanted(on: boolean) {
  localStorage.setItem(PREF_KEY, on ? 'on' : 'off')
  window.dispatchEvent(new Event('yaam-notify-pref'))
}

// turnOnNotifications asks the browser for permission and stores the
// preference; it returns the resulting permission.
export async function turnOnNotifications(): Promise<NotificationPermission> {
  if (!notificationsSupported) return 'denied'
  const p = Notification.permission === 'granted' ? 'granted' : await Notification.requestPermission()
  setNotificationsWanted(p === 'granted')
  return p
}

export function notifyPending(item: Pending) {
  if (!notificationsSupported || !notificationsWanted() || Notification.permission !== 'granted') return
  if (!document.hidden) return
  const n = new Notification('yaam: Approval needed', {
    body: `${item.agentName} · ${item.connection}: ${item.purpose}`,
    tag: item.id,
  })
  n.onclick = () => {
    window.focus()
    location.hash = '#/approvals'
    n.close()
  }
}
