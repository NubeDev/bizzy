import { Lock, Users, Link, Globe } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import type { Visibility } from "@/lib/types"

const config: Record<Visibility, { icon: typeof Lock; label: string }> = {
  private: { icon: Lock, label: "Private" },
  shared: { icon: Users, label: "Shared" },
  unlisted: { icon: Link, label: "Unlisted" },
  public: { icon: Globe, label: "Public" },
}

export function VisibilityBadge({ visibility }: { visibility: Visibility }) {
  const { icon: Icon, label } = config[visibility]
  return (
    <Badge variant="outline" className="gap-1">
      <Icon size={12} />
      {label}
    </Badge>
  )
}
