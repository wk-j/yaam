import { useEffect, useRef, useState, type ButtonHTMLAttributes, type ReactNode } from 'react'

// Stroke icons drawn on a 24×24 grid in currentColor. They are decoration:
// the control that holds one carries the accessible name.
export interface IconProps {
  size?: number
  className?: string
}

function Svg({ size = 16, className, children }: IconProps & { children: ReactNode }) {
  return (
    <svg
      className={className ? `ico ${className}` : 'ico'}
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.75}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
    >
      {children}
    </svg>
  )
}

export const IconEditor = (p: IconProps) => (
  <Svg {...p}>
    <rect x="3" y="4" width="18" height="16" rx="2.5" />
    <path d="m7.5 9.5 2.5 2.5-2.5 2.5M13 15h3.5" />
  </Svg>
)

export const IconApprovals = (p: IconProps) => (
  <Svg {...p}>
    <path d="M12 21.5s7.5-3.6 7.5-9.5V5.6L12 2.8 4.5 5.6V12c0 5.9 7.5 9.5 7.5 9.5Z" />
    <path d="m8.8 11.9 2.2 2.2 4.2-4.4" />
  </Svg>
)

export const IconAgentLog = (p: IconProps) => (
  <Svg {...p}>
    <path d="M3 12h3.5l2.5-6.5 4.5 13 2.5-6.5H21" />
  </Svg>
)

export const IconExecutionLog = (p: IconProps) => (
  <Svg {...p}>
    <path d="M3.5 12a8.5 8.5 0 1 0 2.5-6L3.5 8.5" />
    <path d="M3.5 3.5v5h5M12 7.5V12l3 2" />
  </Svg>
)

export const IconDatabase = (p: IconProps) => (
  <Svg {...p}>
    <ellipse cx="12" cy="5.5" rx="8" ry="2.75" />
    <path d="M4 5.5v13c0 1.5 3.6 2.75 8 2.75s8-1.25 8-2.75v-13" />
    <path d="M4 12c0 1.5 3.6 2.75 8 2.75S20 13.5 20 12" />
  </Svg>
)

export const IconSettings = (p: IconProps) => (
  <Svg {...p}>
    <path d="M4 6h9M17 6h3M4 12h3M11 12h9M4 18h11M19 18h1" />
    <circle cx="15" cy="6" r="2" />
    <circle cx="9" cy="12" r="2" />
    <circle cx="17" cy="18" r="2" />
  </Svg>
)

// Brand mark: a "Y" set in a filled accent tile.
export const IconBrand = ({ size = 20, className }: IconProps) => (
  <svg
    className={className ? `ico brand-mark ${className}` : 'ico brand-mark'}
    width={size}
    height={size}
    viewBox="0 0 24 24"
    aria-hidden="true"
    focusable="false"
  >
    <rect x="1.5" y="1.5" width="21" height="21" rx="6" fill="var(--accent)" />
    <path
      d="m7.5 6.5 4.5 5.5 4.5-5.5M12 12v6"
      fill="none"
      stroke="var(--on-accent)"
      strokeWidth={2.4}
      strokeLinecap="round"
      strokeLinejoin="round"
    />
  </svg>
)

export const IconPlus = (p: IconProps) => (
  <Svg {...p}>
    <path d="M12 5v14M5 12h14" />
  </Svg>
)

export const IconPlay = (p: IconProps) => (
  <Svg {...p}>
    <path d="M7 4.5v15l12-7.5Z" fill="currentColor" />
  </Svg>
)

export const IconCheck = (p: IconProps) => (
  <Svg {...p}>
    <path d="m4.5 12.5 5 5 10-11" />
  </Svg>
)

export const IconX = (p: IconProps) => (
  <Svg {...p}>
    <path d="M6 6l12 12M18 6 6 18" />
  </Svg>
)

export const IconPlug = (p: IconProps) => (
  <Svg {...p}>
    <path d="M9 3v4.5M15 3v4.5M6 7.5h12v3a6 6 0 0 1-12 0Z" />
    <path d="M12 16.5V21" />
  </Svg>
)

export const IconEdit = (p: IconProps) => (
  <Svg {...p}>
    <path d="M15.5 4.5a2.1 2.1 0 0 1 3 3L8 18l-4 1 1-4Z" />
    <path d="M13.5 6.5l3 3" />
  </Svg>
)

export const IconTrash = (p: IconProps) => (
  <Svg {...p}>
    <path d="M4 6.5h16M9.5 6.5V4.5h5v2" />
    <path d="M6 6.5l1 13a1.5 1.5 0 0 0 1.5 1.5h7a1.5 1.5 0 0 0 1.5-1.5l1-13" />
    <path d="M10 11v6M14 11v6" />
  </Svg>
)

export const IconCopy = (p: IconProps) => (
  <Svg {...p}>
    <rect x="8.5" y="8.5" width="12" height="12" rx="2" />
    <path d="M15.5 8.5V5.5a2 2 0 0 0-2-2h-8a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h3" />
  </Svg>
)

export const IconRefresh = (p: IconProps) => (
  <Svg {...p}>
    <path d="M20 12a8 8 0 1 1-2.35-5.65L20 8.5" />
    <path d="M20 3.5v5h-5" />
  </Svg>
)

export const IconChevronsDown = (p: IconProps) => (
  <Svg {...p}>
    <path d="m7 6.5 5 5 5-5M7 12.5l5 5 5-5" />
  </Svg>
)

export const IconUndo = (p: IconProps) => (
  <Svg {...p}>
    <path d="M9 14 4 9l5-5" />
    <path d="M4 9h10.5a5.5 5.5 0 0 1 0 11H11" />
  </Svg>
)

export const IconSave = (p: IconProps) => (
  <Svg {...p}>
    <path d="M5 3.5h11l4.5 4.5v11a1.5 1.5 0 0 1-1.5 1.5H5A1.5 1.5 0 0 1 3.5 19V5A1.5 1.5 0 0 1 5 3.5Z" />
    <path d="M7.5 3.5v4.5h7.5M7.5 20.5v-6.5h9v6.5" />
  </Svg>
)

export const IconBell = (p: IconProps) => (
  <Svg {...p}>
    <path d="M18 8.5a6 6 0 0 0-12 0c0 6.5-2.5 8.5-2.5 8.5h17S18 15 18 8.5Z" />
    <path d="M10.3 20.5a2 2 0 0 0 3.4 0" />
  </Svg>
)

export const IconBellOff = (p: IconProps) => (
  <Svg {...p}>
    <path d="M8.6 3.6A6 6 0 0 1 18 8.5c0 2.6.4 4.4 1 5.6M17 17H3.5S6 15 6 8.5c0-.6.1-1.2.3-1.8" />
    <path d="M10.3 20.5a2 2 0 0 0 3.4 0M3 3l18 18" />
  </Svg>
)

export const IconInfo = (p: IconProps) => (
  <Svg {...p}>
    <circle cx="12" cy="12" r="9" />
    <path d="M12 11v5.5M12 7.6v.1" />
  </Svg>
)

export const IconChevronRight = (p: IconProps) => (
  <Svg {...p}>
    <path d="m9.5 6 6 6-6 6" />
  </Svg>
)

export const IconChevronDown = (p: IconProps) => (
  <Svg {...p}>
    <path d="m6 9.5 6 6 6-6" />
  </Svg>
)

export const IconSearch = (p: IconProps) => (
  <Svg {...p}>
    <circle cx="11" cy="11" r="6.5" />
    <path d="m20 20-4.4-4.4" />
  </Svg>
)

export const IconTable = (p: IconProps) => (
  <Svg {...p}>
    <rect x="3.5" y="4.5" width="17" height="15" rx="2" />
    <path d="M3.5 9.5h17M3.5 14.5h17M9.5 9.5v10" />
  </Svg>
)

export const IconView = (p: IconProps) => (
  <Svg {...p}>
    <path d="M2.5 12S6 5.5 12 5.5 21.5 12 21.5 12 18 18.5 12 18.5 2.5 12 2.5 12Z" />
    <circle cx="12" cy="12" r="2.75" />
  </Svg>
)

export const IconKey = (p: IconProps) => (
  <Svg {...p}>
    <circle cx="8" cy="15.5" r="4.5" />
    <path d="m11.3 12.2 8.7-8.7M16.5 7l2.5 2.5M14.5 9l1.5 1.5" />
  </Svg>
)

export const IconAlert = (p: IconProps) => (
  <Svg {...p}>
    <path d="M10.3 4.1 2.6 17.5a2 2 0 0 0 1.7 3h15.4a2 2 0 0 0 1.7-3L13.7 4.1a2 2 0 0 0-3.4 0Z" />
    <path d="M12 9.5v4M12 17v.1" />
  </Svg>
)

export const IconClock = (p: IconProps) => (
  <Svg {...p}>
    <circle cx="12" cy="12" r="9" />
    <path d="M12 7v5l3.5 2" />
  </Svg>
)

export const IconStatusOk = (p: IconProps) => (
  <Svg {...p}>
    <circle cx="12" cy="12" r="9" />
    <path d="m8 12.3 2.8 2.8L16 9.5" />
  </Svg>
)

export const IconStatusError = (p: IconProps) => (
  <Svg {...p}>
    <circle cx="12" cy="12" r="9" />
    <path d="m9 9 6 6M15 9l-6 6" />
  </Svg>
)

export const IconStatusUnknown = (p: IconProps) => (
  <Svg {...p}>
    <circle cx="12" cy="12" r="9" strokeDasharray="3.2 3.2" />
  </Svg>
)

export const IconSun = (p: IconProps) => (
  <Svg {...p}>
    <circle cx="12" cy="12" r="4" />
    <path d="M12 2.5v2M12 19.5v2M2.5 12h2M19.5 12h2M5.3 5.3l1.4 1.4M17.3 17.3l1.4 1.4M5.3 18.7l1.4-1.4M17.3 6.7l1.4-1.4" />
  </Svg>
)

export const IconMoon = (p: IconProps) => (
  <Svg {...p}>
    <path d="M20 14.5A8 8 0 0 1 9.5 4a8 8 0 1 0 10.5 10.5Z" />
  </Svg>
)

export const IconMonitor = (p: IconProps) => (
  <Svg {...p}>
    <rect x="3" y="4" width="18" height="12.5" rx="2" />
    <path d="M8.5 20.5h7M12 16.5v4" />
  </Svg>
)

// A panel with its left part, and an arrow showing what the click does.
export const IconPanelCollapse = (p: IconProps) => (
  <Svg {...p}>
    <rect x="3" y="4" width="18" height="16" rx="2.5" />
    <path d="M9 4v16M15.5 9.5 13 12l2.5 2.5" />
  </Svg>
)

export const IconPanelExpand = (p: IconProps) => (
  <Svg {...p}>
    <rect x="3" y="4" width="18" height="16" rx="2.5" />
    <path d="M9 4v16M13 9.5l2.5 2.5-2.5 2.5" />
  </Svg>
)

// IconButton is a button that shows only an icon; `label` names it for
// assistive technology and as its tooltip.
export function IconButton({
  label,
  className,
  children,
  ...rest
}: { label: string; children: ReactNode } & Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'aria-label' | 'title'>) {
  return (
    <button type="button" className={className ? `btn icon ${className}` : 'btn icon'} aria-label={label} title={label} {...rest}>
      {children}
    </button>
  )
}

// InfoTip is an info mark whose explanation shows as a tooltip and is
// read by assistive technology.
export function InfoTip({ text }: { text: string }) {
  return (
    <span className="info-tip" tabIndex={0} role="img" aria-label={text} title={text}>
      <IconInfo size={15} />
    </span>
  )
}

// CopyButton copies `text` and shows a check for 1.5 s afterwards.
export function CopyButton({
  text,
  label = 'Copy',
  onError,
  className,
}: {
  text: string
  label?: string
  onError: (message: string) => void
  className?: string
}) {
  const [copied, setCopied] = useState(false)
  const timer = useRef(0)
  useEffect(() => () => window.clearTimeout(timer.current), [])

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(text)
      setCopied(true)
      window.clearTimeout(timer.current)
      timer.current = window.setTimeout(() => setCopied(false), 1500)
    } catch (err) {
      onError(`Copy failed: ${(err as Error).message}`)
    }
  }

  return (
    <IconButton label={copied ? 'Copied' : label} className={`${className ?? ''}${copied ? ' done' : ''}`.trim()} onClick={copy}>
      {copied ? <IconCheck /> : <IconCopy />}
    </IconButton>
  )
}
