import { useState, useRef, useEffect } from "react"
import { ArrowUp, Loader2 } from "lucide-react"
import { motion, useMotionValue, useSpring, useTransform } from "motion/react"
import { BouncingBalls } from "@/components/ui/bouncing-balls"
import { useStoreApps, useCategories } from "@/hooks/use-store"
import { AppCard } from "@/components/store/app-card"
import { CategoryPills } from "@/components/store/category-pills"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

export function StoreHomePage() {
  const [query, setQuery] = useState("")
  const [category, setCategory] = useState("")
  const [sort, setSort] = useState("popular")
  const inputRef = useRef<HTMLTextAreaElement>(null)

  // Typewriter / backspace cycling effect
  const phrases = ["listo", "hey listo", "hola listo"]
  const [displayed, setDisplayed] = useState("")
  const [cursorOn, setCursorOn] = useState(true)

  useEffect(() => {
    let phraseIdx = 0
    let charIdx = 0
    let deleting = false
    let timeout: ReturnType<typeof setTimeout>

    // Cursor blink
    const cursorInterval = setInterval(() => setCursorOn(v => !v), 530)

    const tick = () => {
      const current = phrases[phraseIdx]
      if (!deleting) {
        charIdx++
        setDisplayed(current.slice(0, charIdx))
        if (charIdx === current.length) {
          // Pause at full word before deleting
          timeout = setTimeout(() => { deleting = true; tick() }, 1800)
          return
        }
        timeout = setTimeout(tick, 95 + Math.random() * 55)
      } else {
        charIdx--
        setDisplayed(current.slice(0, charIdx))
        if (charIdx === 0) {
          deleting = false
          phraseIdx = (phraseIdx + 1) % phrases.length
          // Pause before typing next
          timeout = setTimeout(tick, 420)
          return
        }
        timeout = setTimeout(tick, 55 + Math.random() * 35)
      }
    }

    timeout = setTimeout(tick, 600)
    return () => { clearTimeout(timeout); clearInterval(cursorInterval) }
  }, [])

  // Bouncing dots state: true during initial load (3s) or while typing
  const [bouncing, setBouncing] = useState(true)
  const typingTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Stop bouncing after 3s on load
  useEffect(() => {
    const t = setTimeout(() => setBouncing(false), 3000)
    return () => clearTimeout(t)
  }, [])

  const handleQueryChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setQuery(e.target.value)
    setBouncing(true)
    if (typingTimer.current) clearTimeout(typingTimer.current)
    typingTimer.current = setTimeout(() => setBouncing(false), 800)
  }

  // Mouse parallax for glows
  const mouseX = useMotionValue(0)
  const mouseY = useMotionValue(0)
  const springX = useSpring(mouseX, { stiffness: 40, damping: 20 })
  const springY = useSpring(mouseY, { stiffness: 40, damping: 20 })
  const glowX = useTransform(springX, [-0.5, 0.5], ["-30px", "30px"])
  const glowY = useTransform(springY, [-0.5, 0.5], ["-20px", "20px"])

  const handleMouseMove = (e: React.MouseEvent<HTMLElement>) => {
    const { left, top, width, height } = e.currentTarget.getBoundingClientRect()
    mouseX.set((e.clientX - left) / width - 0.5)
    mouseY.set((e.clientY - top) / height - 0.5)
  }

  const { data, isLoading } = useStoreApps({ q: query, category, sort, limit: 30 })
  const { data: categories } = useCategories()

  const handleSearch = () => {
    if (query.trim()) {
      document.getElementById("app-grid")?.scrollIntoView({ behavior: "smooth" })
    }
  }

  return (
    <div className="min-h-full">
      {/* ── Grok-style full-bleed hero ── */}
      <section
        className="relative flex flex-col items-center justify-center min-h-[100vh] overflow-hidden bg-[#0e1015]"
        onMouseMove={handleMouseMove}
      >
        {/* Atmospheric glow — parallax on mouse */}
        <div className="pointer-events-none absolute inset-0">
          <motion.div
            className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[900px] h-[600px]"
            style={{ x: glowX, y: glowY,
              background: "radial-gradient(ellipse at center, rgba(120,140,200,0.14) 0%, rgba(80,100,160,0.06) 30%, rgba(40,60,120,0.02) 60%, transparent 80%)",
              filter: "blur(40px)",
            }}
          />
          {/* Floating orb 1 */}
          <motion.div
            className="absolute w-[300px] h-[300px] rounded-full"
            style={{
              top: "20%", left: "15%",
              background: "radial-gradient(circle, rgba(100,120,255,0.08) 0%, transparent 70%)",
              filter: "blur(30px)",
            }}
            animate={{ y: [0, -30, 0], x: [0, 15, 0], scale: [1, 1.1, 1] }}
            transition={{ duration: 8, repeat: Infinity, ease: "easeInOut" }}
          />
          {/* Floating orb 2 */}
          <motion.div
            className="absolute w-[400px] h-[400px] rounded-full"
            style={{
              top: "30%", right: "10%",
              background: "radial-gradient(circle, rgba(180,200,255,0.07) 0%, transparent 70%)",
              filter: "blur(50px)",
            }}
            animate={{ y: [0, 25, 0], x: [0, -20, 0], scale: [1, 0.9, 1] }}
            transition={{ duration: 11, repeat: Infinity, ease: "easeInOut", delay: 2 }}
          />
          {/* Floating orb 3 — bottom */}
          <motion.div
            className="absolute w-[250px] h-[250px] rounded-full"
            style={{
              bottom: "15%", left: "40%",
              background: "radial-gradient(circle, rgba(140,100,255,0.06) 0%, transparent 70%)",
              filter: "blur(35px)",
            }}
            animate={{ y: [0, -20, 0], scale: [1, 1.15, 1] }}
            transition={{ duration: 9, repeat: Infinity, ease: "easeInOut", delay: 4 }}
          />
          {/* Subtle grain overlay */}
          <div className="absolute inset-0 opacity-[0.03]" style={{
            backgroundImage: "url(\"data:image/svg+xml,%3Csvg viewBox='0 0 256 256' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='noise'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.9' numOctaves='4' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23noise)' opacity='1'/%3E%3C/svg%3E\")",
          }} />
        </div>

        {/* Brand title — typewriter / backspace cycling */}
        <motion.div
          className="relative z-10 flex items-end mb-10 select-none"
          initial={{ opacity: 0, y: 40, filter: "blur(12px)" }}
          animate={{ opacity: 1, y: 0, filter: "blur(0px)" }}
          transition={{ duration: 0.8, delay: 0.1, ease: [0.16, 1, 0.3, 1] }}
        >
          <span
            className="font-mono font-extralight leading-[0.9] tracking-tight"
            style={{
              fontSize: "clamp(3rem,9vw,7rem)",
              background: "linear-gradient(180deg, rgba(255,255,255,0.95) 0%, rgba(255,255,255,0.45) 100%)",
              WebkitBackgroundClip: "text",
              WebkitTextFillColor: "transparent",
              minWidth: "2ch",
              display: "inline-block",
            }}
          >
            {displayed}
          </span>
          {/* Blinking cursor */}
          <span
            className="font-mono font-extralight leading-[0.9] ml-[2px]"
            style={{
              fontSize: "clamp(3rem,9vw,7rem)",
              color: "rgba(255,255,255,0.55)",
              opacity: cursorOn ? 1 : 0,
              transition: "opacity 0.1s",
              display: "inline-block",
              lineHeight: 0.9,
            }}
          >|</span>

          {/* 5 coloured bouncing balls — inline, after cursor, bottom-aligned */}
          <div className="ml-5 pb-[0.15em]">
            <BouncingBalls active={bouncing} size={14} />
          </div>
        </motion.div>

        {/* Search box */}
        <motion.div
          className="relative z-10 w-full max-w-[760px] px-6"
          initial={{ opacity: 0, y: 30, filter: "blur(8px)" }}
          animate={{ opacity: 1, y: 0, filter: "blur(0px)" }}
          transition={{ duration: 0.8, delay: 0.55, ease: [0.16, 1, 0.3, 1] }}
        >
          <div className="relative rounded-2xl border border-white/10 bg-white/[0.04] backdrop-blur-sm overflow-hidden focus-within:border-white/20 transition-colors">
            <input
              type="text"
              value={query}
              onChange={handleQueryChange}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault()
                  handleSearch()
                }
              }}
              placeholder="What do you want to find?"
              className="w-full bg-transparent px-6 py-5 pr-14 text-[15px] text-white placeholder:text-white/30 focus:outline-none font-light"
            />
            <motion.button
              onClick={handleSearch}
              className="absolute right-4 top-1/2 -translate-y-1/2 flex h-9 w-9 items-center justify-center rounded-full border border-white/15 text-white/40 hover:text-white hover:border-white/30 transition-colors"
              whileHover={{ scale: 1.1 }}
              whileTap={{ scale: 0.9 }}
            >
              <ArrowUp size={16} />
            </motion.button>
          </div>
        </motion.div>

        {/* Bottom announcement strip */}
        <motion.div
          className="absolute bottom-0 inset-x-0 z-10 flex items-center justify-center gap-6 py-4 px-6 border-t border-white/[0.06]"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ duration: 1, delay: 0.9 }}
        >
          <span className="text-white/40 text-[13px] font-light">
            AI-powered tools for IoT & building automation
          </span>
          <button
            onClick={() => document.getElementById("app-grid")?.scrollIntoView({ behavior: "smooth" })}
            className="font-mono text-[10px] uppercase tracking-[1.6px] border border-white/15 px-4 py-1.5 text-white/60 hover:text-white hover:border-white/30 transition-colors"
          >
            Browse Apps
          </button>
        </motion.div>

        {/* Scroll hint */}
        <div className="absolute bottom-16 left-8 z-10">
          <svg width="14" height="14" viewBox="0 0 14 14" className="text-white/30 animate-bounce">
            <path d="M7 1v10M3 8l4 4 4-4" stroke="currentColor" strokeWidth="1.2" fill="none" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        </div>
      </section>

      {/* ── App grid section ── */}
      <div id="app-grid" className="max-w-6xl mx-auto px-8 py-20 space-y-10">
        {/* Section heading — Grok style */}
        <h2 className="text-3xl sm:text-4xl font-light tracking-tight leading-tight">
          Apps for every workflow
        </h2>

        {data?.apps?.length ? (
          <div className="flex gap-6 mb-0">
            <span className="text-sm text-muted-foreground font-mono uppercase tracking-wide">
              {data.total} apps
            </span>
            <span className="text-sm text-muted-foreground font-mono uppercase tracking-wide">
              {categories?.length || 0} categories
            </span>
          </div>
        ) : null}

        <div className="flex items-center justify-between gap-4 flex-wrap">
          <CategoryPills
            categories={categories || []}
            selected={category}
            onSelect={setCategory}
          />
          <Select value={sort} onValueChange={setSort}>
            <SelectTrigger className="w-[140px] border-white/[0.1] text-sm font-mono uppercase tracking-wide text-xs rounded-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent className="rounded-lg">
              <SelectItem value="popular">Popular</SelectItem>
              <SelectItem value="recent">Recent</SelectItem>
              <SelectItem value="rating">Top Rated</SelectItem>
              <SelectItem value="name">Name</SelectItem>
            </SelectContent>
          </Select>
        </div>

        {isLoading ? (
          <div className="flex flex-col items-center justify-center py-24 gap-4">
            <Loader2 className="animate-spin text-muted-foreground" size={20} />
            <p className="text-sm text-muted-foreground font-mono uppercase tracking-wide">Loading...</p>
          </div>
        ) : !data?.apps?.length ? (
          <div className="text-center py-24">
            <p className="font-mono text-lg mb-2">No apps found</p>
            <p className="text-sm text-muted-foreground">
              {query || category
                ? "Try adjusting your search or filters"
                : "No apps have been published yet"}
            </p>
          </div>
        ) : (
          <>
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6">
              {data.apps.map((app) => (
                <AppCard key={app.id} app={app} />
              ))}
            </div>
            {data.total > data.apps.length && (
              <p className="text-center text-xs text-muted-foreground font-mono uppercase tracking-wide pt-8">
                Showing {data.apps.length} of {data.total}
              </p>
            )}
          </>
        )}
      </div>
    </div>
  )
}
