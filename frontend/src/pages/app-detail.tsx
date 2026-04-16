import { useState } from "react"
import { useParams, Link } from "react-router-dom"
import { ArrowLeft, Download, Wrench, MessageSquare, Check, Loader2, Play } from "lucide-react"
import { ToolRunner } from "@/components/store/tool-runner"
import { PromptRunner } from "@/components/store/prompt-runner"
import { QaWizard } from "@/components/store/qa-wizard"
import { useStoreApp, useAppReviews, useInstallApp, useSubmitReview } from "@/hooks/use-store"
import { RatingStars, RatingInput } from "@/components/store/rating-stars"
import { categoryLabel, formatDate } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import type { SettingDef, StoreApp } from "@/lib/types"

export function AppDetailPage() {
  const { id } = useParams<{ id: string }>()
  const { data, isLoading } = useStoreApp(id!)
  const { data: reviews } = useAppReviews(id!)
  const installMutation = useInstallApp()
  const reviewMutation = useSubmitReview()

  const [showInstall, setShowInstall] = useState(false)
  const [installSettings, setInstallSettings] = useState<Record<string, string>>({})
  const [reviewRating, setReviewRating] = useState(0)
  const [reviewComment, setReviewComment] = useState("")

  if (isLoading || !data) {
    return (
      <div className="flex justify-center py-20">
        <Loader2 className="animate-spin text-muted-foreground" size={24} />
      </div>
    )
  }

  const { app, installed } = data

  const handleInstall = async () => {
    if (app.settings?.length && !showInstall) {
      setShowInstall(true)
      return
    }
    await installMutation.mutateAsync({ id: app.id, settings: installSettings })
    setShowInstall(false)
  }

  const handleReview = async () => {
    if (reviewRating === 0) return
    await reviewMutation.mutateAsync({ appId: app.id, rating: reviewRating, comment: reviewComment })
    setReviewRating(0)
    setReviewComment("")
  }

  return (
    <div className="p-6 max-w-4xl mx-auto space-y-6">
      <Button variant="ghost" size="sm" asChild className="rounded-none font-mono text-xs uppercase tracking-wider">
        <Link to="/">
          <ArrowLeft size={14} className="mr-1" /> Back
        </Link>
      </Button>

      {/* Header */}
      <div className="flex items-start gap-4">
        <div
          className="w-14 h-14 rounded-none flex items-center justify-center text-xl font-mono font-light text-white shrink-0"
          style={{ background: app.color || '#1f2228' }}
        >
          {app.displayName.charAt(0)}
        </div>
        <div className="flex-1 min-w-0">
          <h1 className="text-xl font-bold">{app.displayName}</h1>
          <p className="text-sm text-muted-foreground mt-0.5">
            by {app.authorName} &middot; <span className="font-mono text-[10px] border border-border px-1.5 py-0.5">v{app.version}</span>
          </p>
          <div className="flex items-center gap-3 mt-2">
            <RatingStars rating={app.avgRating} count={app.reviewCount} size={14} />
            <span className="flex items-center gap-1 text-xs text-muted-foreground">
              <Download size={12} /> {app.installCount} installs
            </span>
          </div>
        </div>
        <Button
          onClick={handleInstall}
          disabled={installed || installMutation.isPending}
          variant={installed ? "outline" : "default"}
          className={`rounded-none font-mono text-xs uppercase tracking-wider transition-opacity ${
            installed ? '' : 'hover:opacity-50'
          }`}
        >
          {installed ? (
            <><Check size={14} className="mr-1" /> Installed</>
          ) : installMutation.isPending ? (
            <Loader2 size={14} className="animate-spin" />
          ) : (
            "Install"
          )}
        </Button>
      </div>

      {/* Install settings dialog */}
      <Dialog open={showInstall} onOpenChange={setShowInstall}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Install {app.displayName}</DialogTitle>
            <DialogDescription>Configure the settings for this app.</DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2">
            {app.settings?.map((s: SettingDef) => (
              <div key={s.key} className="space-y-2">
                <Label>
                  {s.label} {s.required && <span className="text-destructive">*</span>}
                </Label>
                <Input
                  type={s.type === "secret" ? "password" : "text"}
                  value={installSettings[s.key] || ""}
                  onChange={(e: React.ChangeEvent<HTMLInputElement>) => setInstallSettings({ ...installSettings, [s.key]: e.target.value })}
                  placeholder={s.default || ""}
                />
              </div>
            ))}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowInstall(false)}>Cancel</Button>
            <Button onClick={handleInstall} disabled={installMutation.isPending}>
              {installMutation.isPending ? "Installing..." : "Install"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Tabs defaultValue="overview">
        <TabsList className="bg-transparent border-b border-border/30 rounded-none w-full justify-start gap-0 p-0 h-auto">
          <TabsTrigger value="overview" className="rounded-none border-b-2 border-transparent data-[state=active]:border-primary data-[state=active]:text-primary data-[state=active]:bg-transparent data-[state=active]:shadow-none px-4 py-2.5 text-[13px] text-muted-foreground transition-colors">Overview</TabsTrigger>
          <TabsTrigger value="tools" className="rounded-none border-b-2 border-transparent data-[state=active]:border-primary data-[state=active]:text-primary data-[state=active]:bg-transparent data-[state=active]:shadow-none px-4 py-2.5 text-[13px] text-muted-foreground transition-colors">Tools & Prompts ({(app.tools?.length || 0) + (app.prompts?.length || 0)})</TabsTrigger>
          <TabsTrigger value="reviews" className="rounded-none border-b-2 border-transparent data-[state=active]:border-primary data-[state=active]:text-primary data-[state=active]:bg-transparent data-[state=active]:shadow-none px-4 py-2.5 text-[13px] text-muted-foreground transition-colors">Reviews ({app.reviewCount})</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" className="space-y-4 mt-6">
          <p className="text-sm leading-relaxed whitespace-pre-wrap">{app.longDescription || app.description}</p>
          {app.category && (
            <div><span className="font-mono text-[10px] uppercase tracking-[1px] border border-border px-2 py-1 text-muted-foreground">{categoryLabel(app.category)}</span></div>
          )}
          {app.tags?.length > 0 && (
            <div className="flex gap-1.5 flex-wrap">
              {app.tags.map((tag) => (
                <Badge key={tag} variant="outline" className="text-xs rounded-none font-mono">{tag}</Badge>
              ))}
            </div>
          )}
        </TabsContent>

        <TabsContent value="tools" className="space-y-3 mt-6">
          <QaToolsSection app={app} installed={installed} />
          <RegularToolsSection app={app} installed={installed} />
          {app.prompts?.length > 0 && (
            <>
              <h3 className="text-sm font-medium flex items-center gap-1.5 mt-4"><MessageSquare size={14} /> Prompts</h3>
              {app.prompts.map((prompt) => (
                <PromptRunner key={prompt.name} appName={app.name} prompt={prompt} installed={installed} />
              ))}
            </>
          )}
          {!app.tools?.length && !app.prompts?.length && (
            <p className="text-sm text-muted-foreground">No tools or prompts defined yet.</p>
          )}
        </TabsContent>

        <TabsContent value="reviews" className="space-y-4 mt-6">
          {installed && (
            <div className="rounded-none border border-border bg-card p-5 space-y-3">
              <h3 className="text-sm font-mono font-medium uppercase tracking-wider">Write a review</h3>
                <RatingInput value={reviewRating} onChange={setReviewRating} />
                <Textarea
                  value={reviewComment}
                  onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => setReviewComment(e.target.value)}
                  placeholder="Optional comment..."
                  rows={2}
                  className="rounded-none bg-transparent border-border"
                />
                <Button onClick={handleReview} disabled={reviewRating === 0 || reviewMutation.isPending} size="sm" className="rounded-none font-mono text-xs uppercase tracking-wider">
                  Submit Review
                </Button>
            </div>
          )}
          {reviews?.map((review) => (
            <div key={review.id} className="rounded-none border border-border bg-card p-4">
                <div className="flex items-center gap-2 mb-1">
                  <span className="text-sm font-medium">{review.userName}</span>
                  <RatingStars rating={review.rating} size={12} />
                  <span className="text-xs text-muted-foreground">{formatDate(review.createdAt)}</span>
                </div>
                {review.comment && <p className="text-sm text-muted-foreground mt-1">{review.comment}</p>}
            </div>
          ))}
          {!reviews?.length && <p className="text-sm text-muted-foreground">No reviews yet.</p>}
        </TabsContent>
      </Tabs>
    </div>
  )
}

// --- QA Tools: show wizard launch buttons ---

function QaToolsSection({ app, installed }: { app: StoreApp; installed: boolean }) {
  const [activeFlow, setActiveFlow] = useState<string | null>(null)
  const qaTools = app.tools?.filter(t => t.mode === "qa") || []
  if (qaTools.length === 0) return null

  return (
    <>
      <h3 className="text-sm font-medium flex items-center gap-1.5"><Play size={14} /> Interactive Tools</h3>
      {qaTools.map(tool => (
        <div key={tool.name} className="border border-border bg-card">
          {activeFlow === `${app.name}.${tool.name}` ? (
            <div className="p-4">
              <QaWizard
                flow={`${app.name}.${tool.name}`}
                title={tool.name.replace(/_/g, " ")}
                onClose={() => setActiveFlow(null)}
              />
            </div>
          ) : (
            <div className="flex items-center gap-3 p-4">
              <div className="flex-1 min-w-0">
                <code className="text-sm font-medium font-mono">{tool.name}</code>
                <p className="text-xs text-muted-foreground mt-0.5">{tool.description}</p>
              </div>
              <Button
                size="sm"
                className="rounded-none font-mono text-xs uppercase tracking-wider shrink-0"
                disabled={!installed}
                onClick={() => setActiveFlow(`${app.name}.${tool.name}`)}
              >
                <Play size={12} className="mr-1" /> Start
              </Button>
            </div>
          )}
        </div>
      ))}
    </>
  )
}

// --- Regular Tools: keep the expandable ToolRunner ---

function RegularToolsSection({ app, installed }: { app: StoreApp; installed: boolean }) {
  const regularTools = app.tools?.filter(t => t.mode !== "qa") || []
  if (regularTools.length === 0) return null

  return (
    <>
      <h3 className="text-sm font-medium flex items-center gap-1.5"><Wrench size={14} /> Tools</h3>
      {regularTools.map(tool => (
        <ToolRunner key={tool.name} appName={app.name} tool={tool} installed={installed} />
      ))}
    </>
  )
}
