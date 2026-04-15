import { Star } from "lucide-react"
import { cn } from "@/lib/utils"

interface DisplayProps {
  rating: number
  count?: number
  size?: number
}

export function RatingStars({ rating, count, size = 14 }: DisplayProps) {
  return (
    <span className="inline-flex items-center gap-1">
      {[1, 2, 3, 4, 5].map((i) => (
        <Star
          key={i}
          size={size}
          className={cn(
            i <= Math.round(rating) ? "text-amber-400 fill-amber-400" : "text-muted-foreground/30"
          )}
        />
      ))}
      {rating > 0 && <span className="text-sm ml-1">{rating.toFixed(1)}</span>}
      {count !== undefined && (
        <span className="text-xs text-muted-foreground">({count})</span>
      )}
    </span>
  )
}

interface InputProps {
  value: number
  onChange: (value: number) => void
  size?: number
}

export function RatingInput({ value, onChange, size = 20 }: InputProps) {
  return (
    <span className="inline-flex items-center gap-0.5">
      {[1, 2, 3, 4, 5].map((i) => (
        <button
          key={i}
          type="button"
          onClick={() => onChange(i)}
          className="hover:scale-110 transition-transform"
        >
          <Star
            size={size}
            className={cn(
              i <= value ? "text-amber-400 fill-amber-400" : "text-muted-foreground/30 hover:text-amber-300"
            )}
          />
        </button>
      ))}
    </span>
  )
}
