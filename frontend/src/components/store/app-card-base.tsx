import type { ReactNode } from "react"
import { ArrowUpRight } from "lucide-react"

/* ── Shared visual helpers ── */

/** Deterministic decorative SVG pattern from app name + color */
export function cardPattern(name: string, color: string) {
  const seed = name.split("").reduce((a, c) => a + c.charCodeAt(0), 0)
  const s1 = seed % 7
  const shapes = [
    `<defs><radialGradient id="g"><stop offset="0%" stop-color="${color}" stop-opacity="0.25"/><stop offset="100%" stop-color="${color}" stop-opacity="0"/></radialGradient></defs>`,
    `<circle cx="${70 + (seed % 60)}" cy="${55 + (seed % 30)}" r="${80}" fill="url(#g)"/>`,
    `<circle cx="${60 + (seed % 40)}" cy="${50 + (seed % 30)}" r="${35 + (seed % 20)}" fill="none" stroke="${color}" stroke-width="1.2" opacity="0.25"/>`,
    `<circle cx="${100 + (seed % 50)}" cy="${30 + (seed % 40)}" r="${12 + s1}" fill="${color}" opacity="0.12"/>`,
    `<line x1="${10 + s1 * 5}" y1="${90}" x2="${140 + s1 * 3}" y2="${10 + s1 * 4}" stroke="${color}" stroke-width="0.7" opacity="0.15"/>`,
    `<rect x="${20 + s1 * 8}" y="${60 + s1 * 3}" width="${30 + s1 * 2}" height="${30 + s1 * 2}" rx="4" fill="none" stroke="${color}" stroke-width="0.8" opacity="0.15" transform="rotate(${s1 * 7} ${50 + s1 * 8} ${75 + s1 * 3})"/>`,
  ]
  return `data:image/svg+xml,${encodeURIComponent(
    `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 120">${shapes.join("")}</svg>`
  )}`
}

/* ── Reusable sub-components ── */

/** Colored icon square with gradient + letter */
export function AppIcon({
  name,
  color,
  size = "md",
}: {
  name: string
  color: string
  size?: "sm" | "md" | "lg"
}) {
  const dims = { sm: "w-8 h-8 text-xs", md: "w-10 h-10 text-sm", lg: "w-12 h-12 text-base" }
  return (
    <div
      className={`${dims[size]} rounded-xl flex items-center justify-center font-semibold text-white shrink-0 shadow-sm`}
      style={{ background: `linear-gradient(135deg, ${color}, ${color}cc)` }}
    >
      {name.charAt(0)}
    </div>
  )
}

/** Color accent gradient bar */
export function AccentBar({ color }: { color: string }) {
  return (
    <div
      className="h-1 w-full"
      style={{ background: `linear-gradient(90deg, ${color}, ${color}88, transparent)` }}
    />
  )
}

/** Decorative pattern visual area */
export function PatternArea({
  name,
  color,
  className = "",
}: {
  name: string
  color: string
  className?: string
}) {
  return (
    <div
      className={`rounded-xl relative overflow-hidden ${className}`}
      style={{ background: `linear-gradient(135deg, ${color}08, ${color}15)` }}
    >
      <div
        className="absolute inset-0 border rounded-xl transition-colors"
        style={{ borderColor: `${color}20` }}
      />
      <img
        src={cardPattern(name, color)}
        alt=""
        className="absolute inset-0 w-full h-full object-cover opacity-70 group-hover:opacity-100 transition-opacity duration-300"
      />
    </div>
  )
}

/** Color-tinted pill badge */
export function ColorPill({
  label,
  color,
  arrow = false,
}: {
  label: string
  color: string
  arrow?: boolean
}) {
  return (
    <span
      className="inline-flex items-center gap-1 text-[10px] font-medium uppercase tracking-[1.2px] rounded-full px-3 py-1 transition-all duration-200 group-hover:gap-1.5"
      style={{
        color,
        background: `${color}12`,
        border: `1px solid ${color}25`,
      }}
    >
      {label}
      {arrow && (
        <ArrowUpRight
          size={10}
          className="opacity-0 group-hover:opacity-80 transition-opacity"
        />
      )}
    </span>
  )
}

/* ── Full card wrapper ── */

interface AppCardBaseProps {
  color: string
  /** Card body — everything between the accent bar and bottom */
  children: ReactNode
  /** Optional bottom-right element (pill badge, action buttons, etc.) */
  footer?: ReactNode
  /** Optional stats on the bottom-left */
  stats?: ReactNode
  /** Optional className override */
  className?: string
}

export function AppCardBase({ color, children, footer, stats, className = "" }: AppCardBaseProps) {
  return (
    <div
      className={`relative h-full flex flex-col rounded-2xl border border-border/60 bg-card overflow-hidden transition-all duration-200 hover:border-border hover:shadow-lg hover:shadow-black/5 dark:hover:shadow-black/20 ${className}`}
    >
      <AccentBar color={color} />
      {children}
      {(stats || footer) && (
        <div className="px-5 pb-4 pt-1 flex items-center justify-between">
          <div className="flex items-center gap-3 text-[11px] text-muted-foreground">
            {stats}
          </div>
          {footer}
        </div>
      )}
    </div>
  )
}
