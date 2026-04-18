import { Link, Outlet, useLocation, useNavigate } from "react-router-dom"
import { Sun, Moon } from "lucide-react"
import { useTheme } from "@/hooks/use-theme"
import { motion } from "motion/react"

const navItems = [
  { label: "Store", url: "/" },
  { label: "Chat", url: "/chat" },
  { label: "My Apps", url: "/my-apps" },
  { label: "Create", url: "/my-apps/create" },
  { label: "Workshop", url: "/workshop" },
  { label: "Plugins", url: "/plugins" },
]

export function AppShell() {
  const location = useLocation()
  const navigate = useNavigate()
  const { theme, toggleTheme } = useTheme()

  const isHome = location.pathname === "/"

  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      {/* ── Grok-style header — no border, logo+nav left, CTA right ── */}
      <header
        className={`z-50 flex h-16 shrink-0 items-center justify-between px-8 ${
          isHome
            ? "absolute inset-x-0 top-0 bg-transparent"
            : "sticky top-0 bg-background"
        }`}
      >
        {/* Left: logo + nav together */}
        <div className="flex items-center gap-8">
          <Link to="/" className="flex items-center shrink-0">
            <motion.svg
              width="30" height="30" viewBox="0 0 28 28" fill="none"
              initial="hidden"
              animate="visible"
              variants={{ visible: { transition: { staggerChildren: 0.08 } } }}
            >
              {/* top-left: indigo */}
              <motion.rect x="4" y="4" width="8" height="8" fill="#6366f1"
                variants={{ hidden: { opacity: 0, scale: 0.4 }, visible: { opacity: 1, scale: 1, transition: { duration: 0.4, ease: "backOut" } } }}
                style={{ transformOrigin: "8px 8px" }}
              />
              {/* top-right: cyan */}
              <motion.rect x="16" y="4" width="8" height="8" fill="#22d3ee"
                variants={{ hidden: { opacity: 0, scale: 0.4 }, visible: { opacity: 1, scale: 1, transition: { duration: 0.4, ease: "backOut" } } }}
                style={{ transformOrigin: "20px 8px" }}
              />
              {/* bottom-left: emerald */}
              <motion.rect x="4" y="16" width="8" height="8" fill="#34d399"
                variants={{ hidden: { opacity: 0, scale: 0.4 }, visible: { opacity: 1, scale: 1, transition: { duration: 0.4, ease: "backOut" } } }}
                style={{ transformOrigin: "8px 20px" }}
              />
              {/* bottom-right: pink */}
              <motion.rect x="16" y="16" width="8" height="8" fill="#f472b6"
                variants={{ hidden: { opacity: 0, scale: 0.4 }, visible: { opacity: 1, scale: 1, transition: { duration: 0.4, ease: "backOut" } } }}
                style={{ transformOrigin: "20px 20px" }}
              />
            </motion.svg>
          </Link>

          <nav className="hidden sm:flex items-center gap-7">
            {navItems.map((item) => {
              const isActive =
                item.url === "/"
                  ? location.pathname === "/"
                  : location.pathname.startsWith(item.url) && item.url !== "/"
              return (
                <Link
                  key={item.url}
                  to={item.url}
                  className={`font-mono text-[12px] uppercase tracking-[1.8px] transition-colors ${
                    isHome
                      ? isActive ? "text-white" : "text-white/50 hover:text-white"
                      : isActive ? "text-foreground" : "text-foreground/40 hover:text-foreground"
                  }`}
                >
                  {item.label}
                </Link>
              )
            })}
          </nav>
        </div>

        {/* Right: theme toggle + CTA */}
        <div className="flex items-center gap-4">
          <button
            onClick={toggleTheme}
            className={`h-8 w-8 flex items-center justify-center transition-colors ${
              isHome
                ? "text-white/40 hover:text-white"
                : "text-foreground/40 hover:text-foreground"
            }`}
            title={theme === "dark" ? "Light mode" : "Dark mode"}
          >
            {theme === "dark" ? <Sun size={15} /> : <Moon size={15} />}
          </button>
          <button
            onClick={() => navigate("/my-apps/create")}
            className={`hidden sm:inline-flex font-mono text-[11px] uppercase tracking-[1.6px] rounded-full px-5 py-2 transition-colors ${
              isHome
                ? "border border-white/25 text-white hover:bg-white hover:text-black"
                : "border border-foreground/20 text-foreground hover:bg-foreground hover:text-background"
            }`}
          >
            Get Started
          </button>
        </div>
      </header>

      {/* Page content */}
      <main className={`flex-1 ${isHome ? "" : "overflow-y-auto"}`}>
        <Outlet />
      </main>
    </div>
  )
}
