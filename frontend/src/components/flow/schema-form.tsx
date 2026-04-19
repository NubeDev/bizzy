import { useCallback, lazy, Suspense } from 'react'

const CodeEditor = lazy(() =>
  import('./code-editor').then((m) => ({ default: m.CodeEditor })),
)

/**
 * JSON Schema type matching the backend's settings.JSONSchema struct.
 * Supports standard JSON Schema fields plus ui:widget and ui:hidden hints.
 */
export interface JSONSchema {
  title?: string
  description?: string
  type: string
  properties?: Record<string, JSONSchema>
  required?: string[]
  default?: unknown
  enum?: unknown[]
  minimum?: number
  maximum?: number
  minLength?: number
  maxLength?: number
  pattern?: string
  format?: string
  items?: JSONSchema
  readOnly?: boolean
  'ui:widget'?: string
  'ui:hidden'?: boolean
}

interface SchemaFormProps {
  schema: JSONSchema
  values: Record<string, unknown>
  onChange: (key: string, value: unknown) => void
}

/**
 * Renders a JSON Schema object as form fields. Each top-level property
 * becomes a labeled form control. The control type is inferred from the
 * schema property's type, enum, format, and ui:widget.
 *
 * This replaces hand-coded per-node-type config panels — any node type
 * with a settings schema gets a config form for free.
 */
export function SchemaForm({ schema, values, onChange }: SchemaFormProps) {
  if (!schema.properties) return null

  const entries = Object.entries(schema.properties)

  return (
    <>
      {entries.map(([key, prop]) => {
        if (prop['ui:hidden']) return null
        return (
          <SchemaField
            key={key}
            name={key}
            schema={prop}
            value={values[key]}
            required={schema.required?.includes(key)}
            onChange={onChange}
          />
        )
      })}
    </>
  )
}

interface SchemaFieldProps {
  name: string
  schema: JSONSchema
  value: unknown
  required?: boolean
  onChange: (key: string, value: unknown) => void
}

function SchemaField({ name, schema, value, required, onChange }: SchemaFieldProps) {
  const handleChange = useCallback(
    (v: unknown) => onChange(name, v),
    [name, onChange],
  )

  // Resolve the value, falling back to default.
  const resolvedValue = value ?? schema.default

  // Determine which control to render.
  const widget = schema['ui:widget']

  // Enum → select dropdown.
  if (schema.enum && schema.enum.length > 0) {
    return (
      <Field label={schema.title || name} description={schema.description} required={required}>
        <select
          value={String(resolvedValue ?? '')}
          onChange={(e) => handleChange(e.target.value)}
          disabled={schema.readOnly}
          className="w-full px-2 py-1 text-xs bg-background border border-border rounded"
        >
          {resolvedValue == null && <option value="">—</option>}
          {schema.enum.map((opt) => (
            <option key={String(opt)} value={String(opt)}>
              {String(opt)}
            </option>
          ))}
        </select>
      </Field>
    )
  }

  // Boolean → checkbox.
  if (schema.type === 'boolean') {
    return (
      <Field label={schema.title || name} description={schema.description} required={required} inline>
        <input
          type="checkbox"
          checked={Boolean(resolvedValue)}
          onChange={(e) => handleChange(e.target.checked)}
          disabled={schema.readOnly}
          className="rounded border-border"
        />
      </Field>
    )
  }

  // Integer / number → number input.
  if (schema.type === 'integer' || schema.type === 'number') {
    return (
      <Field label={schema.title || name} description={schema.description} required={required}>
        <input
          type="number"
          value={resolvedValue != null ? Number(resolvedValue) : ''}
          onChange={(e) => {
            const v = e.target.value
            if (v === '') {
              handleChange(undefined)
            } else {
              handleChange(schema.type === 'integer' ? parseInt(v, 10) : parseFloat(v))
            }
          }}
          min={schema.minimum}
          max={schema.maximum}
          disabled={schema.readOnly}
          className="w-full px-2 py-1 text-xs bg-background border border-border rounded"
        />
      </Field>
    )
  }

  // String with widget or format overrides.
  if (schema.type === 'string') {
    // Code widget → CodeMirror editor.
    if (widget === 'code') {
      return (
        <Field label={schema.title || name} description={schema.description} required={required}>
          <Suspense fallback={
            <textarea
              value={String(resolvedValue ?? '')}
              onChange={(e) => handleChange(e.target.value)}
              className="w-full px-2 py-1 text-xs bg-background border border-border rounded font-mono resize-y min-h-[120px]"
              placeholder={schema.description}
            />
          }>
            <CodeEditor
              value={String(resolvedValue ?? '')}
              onChange={(v) => handleChange(v)}
              placeholder={schema.description}
            />
          </Suspense>
        </Field>
      )
    }

    // Textarea / json widgets → textarea.
    if (widget === 'textarea' || widget === 'json') {
      return (
        <Field label={schema.title || name} description={schema.description} required={required}>
          <textarea
            value={
              widget === 'json' && resolvedValue != null && typeof resolvedValue !== 'string'
                ? JSON.stringify(resolvedValue, null, 2)
                : String(resolvedValue ?? '')
            }
            onChange={(e) => {
              if (widget === 'json') {
                try {
                  handleChange(JSON.parse(e.target.value))
                } catch {
                  handleChange(e.target.value)
                }
              } else {
                handleChange(e.target.value)
              }
            }}
            disabled={schema.readOnly}
            className={`w-full px-2 py-1 text-xs bg-background border border-border rounded resize-y min-h-[60px] ${
              widget === 'json' ? 'font-mono' : ''
            }`}
            placeholder={schema.description}
          />
        </Field>
      )
    }

    // Default string → text input.
    return (
      <Field label={schema.title || name} description={schema.description} required={required}>
        <input
          type="text"
          value={String(resolvedValue ?? '')}
          onChange={(e) => handleChange(e.target.value)}
          disabled={schema.readOnly}
          maxLength={schema.maxLength}
          className={`w-full px-2 py-1 text-xs bg-background border border-border rounded ${
            schema.format === 'uri' || schema.pattern ? 'font-mono' : ''
          }`}
          placeholder={schema.description}
        />
      </Field>
    )
  }

  // Object type without properties (free-form JSON).
  if (schema.type === 'object' && !schema.properties) {
    return (
      <Field label={schema.title || name} description={schema.description} required={required}>
        <textarea
          value={resolvedValue != null ? JSON.stringify(resolvedValue, null, 2) : ''}
          onChange={(e) => {
            try {
              handleChange(JSON.parse(e.target.value))
            } catch {
              handleChange(e.target.value)
            }
          }}
          disabled={schema.readOnly}
          className="w-full px-2 py-1 text-xs bg-background border border-border rounded font-mono resize-y min-h-[40px]"
          placeholder="{}"
        />
      </Field>
    )
  }

  // Fallback for unrecognised types: render as text.
  return (
    <Field label={schema.title || name} description={schema.description} required={required}>
      <input
        type="text"
        value={String(resolvedValue ?? '')}
        onChange={(e) => handleChange(e.target.value)}
        className="w-full px-2 py-1 text-xs bg-background border border-border rounded"
      />
    </Field>
  )
}

function Field({
  label,
  description,
  required,
  inline,
  children,
}: {
  label: string
  description?: string
  required?: boolean
  inline?: boolean
  children: React.ReactNode
}) {
  return (
    <div className={inline ? 'flex items-center gap-2' : ''}>
      <label className="text-[10px] font-mono uppercase tracking-wider text-muted-foreground mb-0.5 block">
        {label}
        {required && <span className="text-destructive ml-0.5">*</span>}
      </label>
      {children}
      {description && !inline && (
        <p className="text-[9px] text-muted-foreground mt-0.5 leading-tight">{description}</p>
      )}
    </div>
  )
}
