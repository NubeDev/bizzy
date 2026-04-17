import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import type { ToolParam } from "@/lib/types"

interface Props {
  params: Record<string, ToolParam>
  values: Record<string, unknown>
  onChange: (values: Record<string, unknown>) => void
}

export function SchemaForm({ params, values, onChange }: Props) {
  const entries = Object.entries(params)

  if (!entries.length) {
    return <p className="text-xs text-muted-foreground">No parameters defined.</p>
  }

  const setValue = (name: string, value: unknown) => {
    onChange({ ...values, [name]: value })
  }

  return (
    <div className="space-y-3">
      {entries.map(([name, def]) => (
        <div key={name} className="space-y-1">
          <Label className="text-xs font-mono">
            {name}
            {def.required && <span className="text-destructive ml-0.5">*</span>}
            <span className="text-muted-foreground font-normal ml-2">{def.type}</span>
          </Label>
          {def.description && (
            <p className="text-[11px] text-muted-foreground">{def.description}</p>
          )}
          {renderField(name, def, values[name], setValue)}
        </div>
      ))}
    </div>
  )
}

function renderField(
  name: string,
  def: ToolParam,
  value: unknown,
  setValue: (name: string, value: unknown) => void,
) {
  // String with options -> select dropdown
  if (def.type === "string" && "options" in def && Array.isArray((def as Record<string, unknown>).options)) {
    const options = (def as Record<string, unknown>).options as string[]
    return (
      <Select
        value={String(value ?? "")}
        onValueChange={(v) => setValue(name, v)}
      >
        <SelectTrigger className="rounded-none bg-background/60 border-border font-mono text-sm h-8">
          <SelectValue placeholder={`Select ${name}...`} />
        </SelectTrigger>
        <SelectContent>
          {options.map((opt) => (
            <SelectItem key={opt} value={opt}>{opt}</SelectItem>
          ))}
        </SelectContent>
      </Select>
    )
  }

  // Boolean -> select true/false (simple approach without a Switch component dependency)
  if (def.type === "boolean") {
    return (
      <Select
        value={value === true ? "true" : value === false ? "false" : ""}
        onValueChange={(v) => setValue(name, v === "true")}
      >
        <SelectTrigger className="rounded-none bg-background/60 border-border font-mono text-sm h-8">
          <SelectValue placeholder="Select..." />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="true">true</SelectItem>
          <SelectItem value="false">false</SelectItem>
        </SelectContent>
      </Select>
    )
  }

  // Number
  if (def.type === "number") {
    return (
      <Input
        type="number"
        className="rounded-none bg-background/60 border-border font-mono text-sm h-8"
        placeholder={def.description || name}
        value={value !== undefined && value !== null ? String(value) : ""}
        onChange={(e) => setValue(name, e.target.value === "" ? undefined : Number(e.target.value))}
      />
    )
  }

  // Multiline string
  if ("multiline" in def && (def as Record<string, unknown>).multiline) {
    return (
      <Textarea
        className="rounded-none bg-background/60 border-border font-mono text-sm"
        placeholder={def.description || name}
        rows={4}
        value={String(value ?? "")}
        onChange={(e) => setValue(name, e.target.value)}
      />
    )
  }

  // Default: text input
  return (
    <Input
      className="rounded-none bg-background/60 border-border font-mono text-sm h-8"
      placeholder={def.description || name}
      value={String(value ?? "")}
      onChange={(e) => setValue(name, e.target.value)}
    />
  )
}
