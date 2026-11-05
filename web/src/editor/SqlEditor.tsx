import { useEffect, useRef } from 'react'
import { EditorView, basicSetup } from 'codemirror'
import { closeCompletion } from '@codemirror/autocomplete'
import { keymap, placeholder } from '@codemirror/view'
import { Compartment, Prec } from '@codemirror/state'
import { syntaxHighlighting } from '@codemirror/language'
import { PostgreSQL, sql, type SQLNamespace } from '@codemirror/lang-sql'
import { sqlHighlighter } from '../sql'
import type { Schema } from '../types'

export const RUN_KEYS = /Mac|iPhone|iPad/.test(navigator.platform) ? '⌘↵' : 'Ctrl+↵'

const theme = EditorView.theme({
  '&': { height: '100%', backgroundColor: 'var(--code-bg)', color: 'var(--fg)', fontSize: '13px' },
  '&.cm-focused': { outline: 'none' },
  '.cm-scroller': { fontFamily: 'var(--mono)', lineHeight: '1.5' },
  '.cm-content': { caretColor: 'var(--accent)' },
  '.cm-cursor, .cm-dropCursor': { borderLeftColor: 'var(--accent)' },
  '.cm-gutters': { backgroundColor: 'var(--code-bg)', color: 'var(--muted)', border: 'none' },
  '.cm-activeLine, .cm-activeLineGutter': { backgroundColor: 'color-mix(in srgb, var(--fg) 5%, transparent)' },
  '&.cm-focused > .cm-scroller > .cm-selectionLayer .cm-selectionBackground, .cm-selectionBackground, ::selection': {
    backgroundColor: 'color-mix(in srgb, var(--accent) 28%, transparent)',
  },
  '.cm-matchingBracket': { backgroundColor: 'color-mix(in srgb, var(--accent) 25%, transparent)', outline: 'none' },
  '.cm-tooltip': { backgroundColor: 'var(--card)', color: 'var(--fg)', border: '1px solid var(--border)' },
  '.cm-tooltip-autocomplete > ul': { fontFamily: 'var(--mono)', fontSize: '12.5px' },
  '.cm-tooltip-autocomplete > ul > li[aria-selected]': { backgroundColor: 'var(--accent)', color: '#181a20' },
  '.cm-completionDetail': { color: 'var(--muted)', fontStyle: 'normal', marginLeft: '1em' },
  '.cm-panels': { backgroundColor: 'var(--card)', color: 'var(--fg)' },
})

// namespace turns the schema into lang-sql's completion namespace:
// schema → table → columns, with `public` completing unqualified.
function namespace(schema: Schema | null): SQLNamespace {
  const ns: Record<string, Record<string, SQLNamespace>> = {}
  for (const tb of schema?.tables ?? []) {
    ns[tb.schema] ??= {}
    ns[tb.schema][tb.name] = {
      self: { label: tb.name, type: 'type', detail: tb.kind },
      children: tb.columns.map((c) => ({ label: c.name, type: 'property', detail: c.pk ? `${c.type} pk` : c.type })),
    }
  }
  return ns
}

const sqlLang = (schema: Schema | null) => sql({ dialect: PostgreSQL, schema: namespace(schema), defaultSchema: 'public' })

export function SqlEditor({
  initialDoc,
  schema,
  onChange,
  onRun,
}: {
  initialDoc: string
  schema: Schema | null
  onChange: (doc: string) => void
  onRun: () => void
}) {
  const host = useRef<HTMLDivElement>(null)
  const view = useRef<EditorView | null>(null)
  const lang = useRef(new Compartment())
  const handlers = useRef({ onChange, onRun })
  handlers.current = { onChange, onRun }

  useEffect(() => {
    const v = new EditorView({
      parent: host.current!,
      doc: initialDoc,
      extensions: [
        Prec.highest(keymap.of([{ key: 'Mod-Enter', run: (v) => (closeCompletion(v), handlers.current.onRun(), true) }])),
        basicSetup,
        theme,
        placeholder(`Write SQL, then press ${RUN_KEYS} to run`),
        syntaxHighlighting(sqlHighlighter),
        lang.current.of(sqlLang(null)),
        EditorView.updateListener.of((u) => {
          if (u.docChanged) handlers.current.onChange(u.state.doc.toString())
        }),
      ],
    })
    view.current = v
    return () => {
      v.destroy()
      view.current = null
    }
    // The document is owned by CodeMirror after mount; the parent remounts
    // this component (key) to load another document.
  }, [])

  useEffect(() => {
    view.current?.dispatch({ effects: lang.current.reconfigure(sqlLang(schema)) })
  }, [schema])

  return <div className="sql" ref={host} />
}
