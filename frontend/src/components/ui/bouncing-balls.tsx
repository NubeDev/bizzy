import { motion } from "motion/react"

const DOTS = [
  { color: "#6366f1", delay: 0 },
  { color: "#22d3ee", delay: 0.14 },
  { color: "#34d399", delay: 0.28 },
  { color: "#f472b6", delay: 0.42 },
]

interface BouncingBallsProps {
  /** Whether the balls are actively bouncing */
  active: boolean
  /** Size of each ball in px (default 12) */
  size?: number
  /** Gap between balls (tailwind gap value, default "gap-3") */
  className?: string
}

export function BouncingBalls({ active, size = 12, className = "" }: BouncingBallsProps) {
  return (
    <div className={`flex items-end gap-2.5 ${className}`}>
      {DOTS.map((dot, i) => (
        <motion.div
          key={i}
          className="relative"
          style={{ width: size, height: size }}
          initial={{ opacity: 0, scale: 0 }}
          animate={{ opacity: 1, scale: 1 }}
          transition={{ duration: 0.35, delay: i * 0.07, ease: "backOut" }}
        >
          {/* Pulse ring */}
          <motion.div
            className="absolute inset-0 rounded-full"
            style={{ background: dot.color }}
            animate={active
              ? { scale: [1, 2.8, 1], opacity: [0.35, 0, 0.35] }
              : { scale: 1, opacity: 0 }
            }
            transition={active
              ? { duration: 1.6, repeat: Infinity, delay: dot.delay, ease: "easeInOut" }
              : { duration: 0.25 }
            }
          />
          {/* Ball */}
          <motion.div
            className="relative rounded-full"
            style={{
              width: size,
              height: size,
              background: dot.color,
              boxShadow: `0 0 ${size}px ${dot.color}aa`,
            }}
            animate={active ? { y: [0, -(size * 1.3), 0] } : { y: 0 }}
            transition={active
              ? { duration: 0.8, repeat: Infinity, delay: dot.delay, ease: "easeInOut" }
              : { duration: 0.35, ease: "easeOut" }
            }
          />
        </motion.div>
      ))}
    </div>
  )
}
