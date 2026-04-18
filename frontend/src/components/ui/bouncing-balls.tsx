import { motion, useAnimate } from "motion/react"
import { useEffect, useRef } from "react"

const DOTS = [
  { color: "#6366f1", delay: 0 },
  { color: "#22d3ee", delay: 0.14 },
  { color: "#34d399", delay: 0.28 },
  { color: "#f472b6", delay: 0.42 },
]

interface BouncingBallsProps {
  active: boolean
  size?: number
  className?: string
}

function Ball({ color, delay, active, size }: { color: string; delay: number; active: boolean; size: number }) {
  const [scope, animate] = useAnimate()
  const running = useRef(false)
  const stopRequested = useRef(false)

  useEffect(() => {
    if (active) {
      stopRequested.current = false
      if (running.current) return
      running.current = true

      const sleep = (ms: number) => new Promise(res => setTimeout(res, ms))

      const bounce = async () => {
        // Initial stagger offset so each ball stays in its own wave phase
        await sleep(delay * 1000)

        while (!stopRequested.current) {
          await animate(scope.current, { y: -(size * 1.3) }, { duration: 0.32, ease: "easeOut" })
          if (stopRequested.current) break
          await animate(scope.current, { y: 0 }, { duration: 0.32, ease: "easeIn" })
          // Small pause at the bottom keeps the wave rhythm natural
          await sleep(60)
        }

        // Smooth wind-down: progressively smaller bounces
        const steps = [0.65, 0.38, 0.18, 0.07]
        for (const factor of steps) {
          const dur = 0.28 + (1 - factor) * 0.22
          await animate(scope.current, { y: -(size * factor) }, { duration: dur, ease: "easeOut" })
          await animate(scope.current, { y: 0 }, { duration: dur, ease: "easeIn" })
          await sleep(40)
        }
        await animate(scope.current, { y: 0 }, { duration: 0.3, ease: "easeOut" })
        running.current = false
      }

      bounce()
    } else {
      stopRequested.current = true
    }
  }, [active])

  return (
    <div className="relative" style={{ width: size, height: size }}>
      {/* Pulse ring */}
      <motion.div
        className="absolute inset-0"
        style={{ background: color }}
        animate={active ? { scale: [1, 2.8, 1], opacity: [0.3, 0, 0.3] } : { scale: 1, opacity: 0 }}
        transition={active
          ? { duration: 1.6, repeat: Infinity, delay, ease: "easeInOut" }
          : { duration: 0.5 }
        }
      />
      {/* Square */}
      <div
        ref={scope}
        style={{
          width: size,
          height: size,
          background: color,
          boxShadow: `0 0 ${size}px ${color}aa`,
          position: "relative",
        }}
      />
    </div>
  )
}

export function BouncingBalls({ active, size = 12, className = "" }: BouncingBallsProps) {
  return (
    <div className={`flex items-end gap-2.5 ${className}`}>
      {DOTS.map((dot, i) => (
        <motion.div
          key={i}
          initial={{ opacity: 0, scale: 0 }}
          animate={{ opacity: 1, scale: 1 }}
          transition={{ duration: 0.35, delay: i * 0.07, ease: "backOut" }}
        >
          <Ball color={dot.color} delay={dot.delay} active={active} size={size} />
        </motion.div>
      ))}
    </div>
  )
}
