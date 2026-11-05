import { useMemo, useState, type ReactNode } from 'react'
import { PostgreSQL } from '@codemirror/lang-sql'
import { highlightCode, tagHighlighter, tags as t } from '@lezer/highlight'
import { CopyButton } from './icons'

// One token → class mapping for the editor and every read-only SQL block, so
// both colour SQL alike; the colours are the .sql-* rules in styles.css.
export const sqlHighlighter = tagHighlighter([
  { tag: t.keyword, class: 'sql-kw' },
  { tag: [t.string, t.special(t.string)], class: 'sql-str' },
  { tag: [t.number, t.bool, t.null], class: 'sql-num' },
  { tag: t.comment, class: 'sql-cmt' },
  { tag: [t.typeName, t.standard(t.name), t.special(t.name)], class: 'sql-type' },
  { tag: t.invalid, class: 'sql-bad' },
])

const parser = PostgreSQL.language.parser

// Longer SQL, such as a bulk INSERT, stays plain: parsing it on every render
// would stall the page.
const HIGHLIGHT_MAX = 20_000

// One-line table cells show only the start of the SQL.
export const CELL_SQL_MAX = 400

// SqlText renders sql with syntax highlighting, as inline spans.
export function SqlText({ sql }: { sql: string }) {
  const nodes = useMemo(() => {
    if (sql.length > HIGHLIGHT_MAX) return [sql]
    const out: ReactNode[] = []
    highlightCode(
      sql,
      parser.parse(sql),
      sqlHighlighter,
      (text, cls) =>
        out.push(
          cls ? (
            <span key={out.length} className={cls}>
              {text}
            </span>
          ) : (
            text
          ),
        ),
      () => out.push('\n'),
    )
    return out
  }, [sql])
  return <>{nodes}</>
}

// SqlBlock shows highlighted SQL in a code block with a copy button.
export function SqlBlock({ sql, className }: { sql: string; className?: string }) {
  const [error, setError] = useState('')
  return (
    <div className={className ? `sql-block ${className}` : 'sql-block'}>
      <pre>
        <code>
          <SqlText sql={sql} />
        </code>
      </pre>
      <CopyButton text={sql} label="Copy SQL" onError={setError} className="sql-copy" />
      {error && <div className="err small">{error}</div>}
    </div>
  )
}
