import { useState, useRef } from "react"
import { ArrowUp, Loader2 } from "lucide-react"
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
      <section className="relative flex flex-col items-center justify-center min-h-[100vh] overflow-hidden bg-[#0e1015]">
        {/* Atmospheric glow */}
        <div className="pointer-events-none absolute inset-0">
          {/* Central radial glow */}
          <div
            className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[900px] h-[600px]"
            style={{
              background:
                "radial-gradient(ellipse at center, rgba(120,140,200,0.12) 0%, rgba(80,100,160,0.06) 30%, rgba(40,60,120,0.02) 60%, transparent 80%)",
              filter: "blur(40px)",
            }}
          />
          {/* Right-side highlight (like the Grok screenshot) */}
          <div
            className="absolute top-1/4 right-0 w-[600px] h-[600px]"
            style={{
              background:
                "radial-gradient(ellipse at 80% 40%, rgba(180,200,255,0.08) 0%, rgba(120,160,220,0.03) 40%, transparent 70%)",
              filter: "blur(60px)",
            }}
          />
          {/* Subtle grain overlay */}
          <div className="absolute inset-0 opacity-[0.03]" style={{
            backgroundImage: "url(\"data:image/svg+xml,%3Csvg viewBox='0 0 256 256' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='noise'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.9' numOctaves='4' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23noise)' opacity='1'/%3E%3C/svg%3E\")",
          }} />
        </div>

        {/* Brand title — massive, centered, like "Grok" */}
        <h1
          className="relative z-10 font-mono font-extralight text-[clamp(5rem,15vw,12rem)] leading-[0.9] tracking-tight text-white select-none mb-10"
          style={{
            background: "linear-gradient(180deg, rgba(255,255,255,0.95) 0%, rgba(255,255,255,0.55) 100%)",
            WebkitBackgroundClip: "text",
            WebkitTextFillColor: "transparent",
          }}
        >
          nube
        </h1>

        {/* Search box — centered, matches Grok's input style */}
        <div className="relative z-10 w-full max-w-[640px] px-6">
          <div className="relative rounded-2xl border border-white/10 bg-white/[0.04] backdrop-blur-sm overflow-hidden focus-within:border-white/20 transition-colors">
            <input
              type="text"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault()
                  handleSearch()
                }
              }}
              placeholder="What do you want to find?"
              className="w-full bg-transparent px-6 py-5 pr-14 text-[15px] text-white placeholder:text-white/30 focus:outline-none font-light"
            />
            <button
              onClick={handleSearch}
              className="absolute right-4 top-1/2 -translate-y-1/2 flex h-9 w-9 items-center justify-center rounded-full border border-white/15 text-white/40 hover:text-white hover:border-white/30 transition-colors"
            >
              <ArrowUp size={16} />
            </button>
          </div>
        </div>

        {/* Bottom announcement strip */}
        <div className="absolute bottom-0 inset-x-0 z-10 flex items-center justify-center gap-6 py-4 px-6 border-t border-white/[0.06]">
          <span className="text-white/40 text-[13px] font-light">
            AI-powered tools for IoT & building automation
          </span>
          <button
            onClick={() => document.getElementById("app-grid")?.scrollIntoView({ behavior: "smooth" })}
            className="font-mono text-[10px] uppercase tracking-[1.6px] border border-white/15 px-4 py-1.5 text-white/60 hover:text-white hover:border-white/30 transition-colors"
          >
            Browse Apps
          </button>
        </div>

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
