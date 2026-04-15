import { Link } from "react-router-dom"
import { Download, Wrench } from "lucide-react"
import type { StoreAppSummary } from "@/lib/types"
import { categoryLabel } from "@/lib/utils"
import { AppCardBase, AppIcon, PatternArea, ColorPill } from "./app-card-base"

export function AppCard({ app }: { app: StoreAppSummary }) {
  const color = app.color || '#6366f1'

  return (
    <Link to={`/apps/${app.id}`} className="group block h-full">
      <AppCardBase
        color={color}
        stats={
          <>
            <span className="flex items-center gap-1">
              <Download size={11} /> {app.installCount}
            </span>
            <span className="flex items-center gap-1">
              <Wrench size={11} /> {app.toolCount}
            </span>
          </>
        }
        footer={
          <ColorPill
            label={app.category ? categoryLabel(app.category) : "View"}
            color={color}
            arrow
          />
        }
      >
        {/* Title + meta */}
        <div className="px-5 pt-5 pb-3">
          <div className="flex items-start gap-3 mb-3">
            <AppIcon name={app.displayName} color={color} />
            <div className="min-w-0 flex-1">
              <p className="font-semibold text-[15px] leading-tight truncate">{app.displayName}</p>
              <p className="text-xs text-muted-foreground mt-0.5">by {app.authorName}</p>
            </div>
          </div>
          <p className="text-[13px] text-muted-foreground leading-relaxed line-clamp-2">
            {app.description}
          </p>
        </div>

        {/* Visual pattern area */}
        <PatternArea
          name={app.displayName}
          color={color}
          className="flex-1 mx-4 mb-3 min-h-[100px]"
        />
      </AppCardBase>
    </Link>
  )
}
