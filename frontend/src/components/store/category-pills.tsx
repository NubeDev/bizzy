import { categoryLabel } from "@/lib/utils"

interface Props {
  categories: string[]
  selected: string
  onSelect: (category: string) => void
}

export function CategoryPills({ categories, selected, onSelect }: Props) {
  const activeClass = "bg-foreground text-background border-foreground"
  const inactiveClass = "bg-transparent text-muted-foreground border-white/[0.1] hover:text-foreground hover:border-white/[0.2]"

  return (
    <div className="flex gap-2 flex-wrap">
      <button
        onClick={() => onSelect("")}
        className={`font-mono text-[11px] uppercase tracking-[1.4px] border rounded-full px-4 py-1.5 transition-colors ${!selected ? activeClass : inactiveClass}`}
      >
        All
      </button>
      {categories.map((cat) => (
        <button
          key={cat}
          onClick={() => onSelect(cat === selected ? "" : cat)}
          className={`font-mono text-[11px] uppercase tracking-[1.4px] border rounded-full px-4 py-1.5 transition-colors ${cat === selected ? activeClass : inactiveClass}`}
        >
          {categoryLabel(cat)}
        </button>
      ))}
    </div>
  )
}
