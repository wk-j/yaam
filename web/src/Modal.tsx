import { useEffect, useRef, type ReactNode } from 'react'

// Modal shows a native modal <dialog> for as long as it is rendered.
export function Modal({ onClose, children, wide = false }: { onClose: () => void; children: ReactNode; wide?: boolean }) {
  const ref = useRef<HTMLDialogElement>(null)
  // Removing the element ends the modal state; closing it in a cleanup would
  // fire `close` (and onClose) on a StrictMode remount.
  useEffect(() => {
    const d = ref.current
    if (d && !d.open) d.showModal()
  }, [])
  return (
    <dialog ref={ref} className={wide ? 'wide' : ''} onClose={onClose}>
      {children}
    </dialog>
  )
}
