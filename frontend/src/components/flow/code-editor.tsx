import { useRef, useEffect, useCallback } from 'react'
import { EditorState } from '@codemirror/state'
import { EditorView, keymap, lineNumbers, highlightActiveLine, highlightActiveLineGutter, drawSelection, placeholder as cmPlaceholder } from '@codemirror/view'
import { defaultKeymap, indentWithTab, history, historyKeymap } from '@codemirror/commands'
import { javascript } from '@codemirror/lang-javascript'
import { syntaxHighlighting, indentOnInput, bracketMatching, foldGutter, foldKeymap, defaultHighlightStyle } from '@codemirror/language'
import { oneDark } from '@codemirror/theme-one-dark'
import { autocompletion, closeBrackets, closeBracketsKeymap, completionKeymap } from '@codemirror/autocomplete'
import { lintKeymap } from '@codemirror/lint'
import { searchKeymap, highlightSelectionMatches } from '@codemirror/search'
import type { CompletionContext, CompletionResult } from '@codemirror/autocomplete'

interface CodeEditorProps {
  value: string
  onChange: (value: string) => void
  placeholder?: string
  minHeight?: string
  maxHeight?: string
  language?: 'javascript'
}

// Flow function node API completions
function flowCompletions(context: CompletionContext): CompletionResult | null {
  const word = context.matchBefore(/[\w.]*/)
  if (!word || (word.from === word.to && !context.explicit)) return null

  const completions = [
    // msg
    { label: 'msg', type: 'variable', detail: 'message object', info: 'The incoming message' },
    { label: 'msg.payload', type: 'property', detail: 'any', info: 'Main data payload' },
    { label: 'msg.topic', type: 'property', detail: 'string', info: 'Message topic/category' },
    { label: 'msg._msgid', type: 'property', detail: 'string', info: 'Unique message ID' },
    // node
    { label: 'node.log', type: 'function', detail: '(msg)', info: 'Log a message' },
    { label: 'node.warn', type: 'function', detail: '(msg)', info: 'Log a warning' },
    { label: 'node.error', type: 'function', detail: '(msg)', info: 'Log an error' },
    { label: 'node.id', type: 'property', detail: 'string', info: 'Current node ID' },
    { label: 'node.name', type: 'property', detail: 'string', info: 'Current node label' },
    // flow
    { label: 'flow.get', type: 'function', detail: '(key)', info: 'Read flow-level state' },
    { label: 'flow.set', type: 'function', detail: '(key, value)', info: 'Write flow-level state' },
    // tools
    { label: 'tools.call', type: 'function', detail: '(name, params)', info: 'Call an app tool' },
  ]

  return {
    from: word.from,
    options: completions,
  }
}

const flowTheme = EditorView.theme({
  '&': {
    fontSize: '12px',
    backgroundColor: 'var(--background)',
  },
  '.cm-content': {
    fontFamily: 'ui-monospace, SFMono-Regular, "SF Mono", Menlo, monospace',
    padding: '8px 0',
  },
  '.cm-gutters': {
    backgroundColor: 'var(--background)',
    borderRight: '1px solid var(--border)',
    color: 'var(--muted-foreground)',
  },
  '.cm-activeLineGutter': {
    backgroundColor: 'transparent',
    color: 'var(--foreground)',
  },
  '.cm-activeLine': {
    backgroundColor: 'hsl(var(--accent) / 0.3)',
  },
  '.cm-cursor': {
    borderColor: 'var(--foreground)',
  },
  '.cm-selectionBackground': {
    backgroundColor: 'hsl(var(--accent) / 0.5) !important',
  },
  '.cm-tooltip': {
    backgroundColor: 'var(--popover)',
    border: '1px solid var(--border)',
    borderRadius: '4px',
  },
  '.cm-tooltip-autocomplete ul li': {
    fontSize: '11px',
    padding: '2px 8px',
  },
  '.cm-tooltip-autocomplete ul li[aria-selected]': {
    backgroundColor: 'hsl(var(--accent) / 0.5)',
  },
  '.cm-placeholder': {
    color: 'var(--muted-foreground)',
    fontStyle: 'italic',
  },
  '.cm-scroller': {
    overflow: 'auto',
  },
})

export function CodeEditor({
  value,
  onChange,
  placeholder = '',
  minHeight = '120px',
  maxHeight = '400px',
}: CodeEditorProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const viewRef = useRef<EditorView | null>(null)
  const onChangeRef = useRef(onChange)
  onChangeRef.current = onChange

  // Stable update listener
  const updateListener = useCallback(
    () =>
      EditorView.updateListener.of((update) => {
        if (update.docChanged) {
          onChangeRef.current(update.state.doc.toString())
        }
      }),
    [],
  )

  useEffect(() => {
    if (!containerRef.current) return

    const state = EditorState.create({
      doc: value,
      extensions: [
        lineNumbers(),
        highlightActiveLineGutter(),
        highlightActiveLine(),
        drawSelection(),
        indentOnInput(),
        bracketMatching(),
        closeBrackets(),
        foldGutter(),
        highlightSelectionMatches(),
        history(),
        autocompletion({
          override: [flowCompletions],
        }),
        javascript(),
        oneDark,
        flowTheme,
        EditorView.lineWrapping,
        keymap.of([
          ...defaultKeymap,
          ...historyKeymap,
          ...closeBracketsKeymap,
          ...foldKeymap,
          ...completionKeymap,
          ...lintKeymap,
          ...searchKeymap,
          indentWithTab,
        ]),
        syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
        cmPlaceholder(placeholder),
        updateListener(),
      ],
    })

    const view = new EditorView({
      state,
      parent: containerRef.current,
    })

    viewRef.current = view

    return () => {
      view.destroy()
      viewRef.current = null
    }
    // Only create editor on mount
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // Sync external value changes (e.g. undo, reset) without losing cursor
  useEffect(() => {
    const view = viewRef.current
    if (!view) return
    const current = view.state.doc.toString()
    if (current !== value) {
      view.dispatch({
        changes: { from: 0, to: current.length, insert: value },
      })
    }
  }, [value])

  return (
    <div
      ref={containerRef}
      className="border border-border rounded overflow-hidden"
      style={{ minHeight, maxHeight }}
    />
  )
}
