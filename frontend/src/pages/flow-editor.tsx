import { useState, useCallback, useMemo, useRef, useEffect } from 'react'
import { useParams } from 'react-router-dom'
import { useFlow, useUpdateFlow, useValidateFlow, useNodeTypes } from '@/hooks/use-flows'
import { useFlowRunWS } from '@/hooks/use-flow-run-ws'
import { FlowCanvas, type FlowCanvasHandle } from '@/components/flow/canvas'
import { NodePalette } from '@/components/flow/node-palette'
import { NodeConfig } from '@/components/flow/node-config'
import { FlowToolbar } from '@/components/flow/flow-toolbar'
import { ExecutionOverlay, useNodeStatesForCanvas } from '@/components/flow/execution-overlay'
import { DebugPanel } from '@/components/flow/debug-panel'
import { Bug, Settings2 } from 'lucide-react'
import { cn } from '@/lib/utils'
import type { Node } from '@xyflow/react'
import type { NodeTypeDef } from '@/lib/types'

type RightTab = 'config' | 'debug'

export function FlowEditorPage() {
  const { id } = useParams<{ id: string }>()
  const { data: flow, isLoading } = useFlow(id || '')
  const { data: nodeTypeCatalog } = useNodeTypes()
  const updateFlow = useUpdateFlow()
  const validateFlow = useValidateFlow()

  const canvasRef = useRef<FlowCanvasHandle>(null)
  const [selectedNode, setSelectedNode] = useState<Node | null>(null)
  const [dirty, setDirty] = useState(false)
  const [validationErrors, setValidationErrors] = useState<string[]>([])
  const [rightTab, setRightTab] = useState<RightTab | null>(null)

  // Live run state via WebSocket (replaces polling).
  const { run: latestRun, runId: latestRunId, totalRuns, wsStatus } = useFlowRunWS(id || '')
  const nodeStates = useNodeStatesForCanvas(latestRun)

  // Accumulate debug entries across runs (persists until user clears).
  const [debugEntries, setDebugEntries] = useState<import('@/lib/types').DebugEntry[]>([])
  const prevDebugLenRef = useRef(0)
  useEffect(() => {
    const runLog = latestRun?.debug_log || []
    if (runLog.length > prevDebugLenRef.current) {
      const newEntries = runLog.slice(prevDebugLenRef.current)
      setDebugEntries((prev) => [...prev, ...newEntries])
    }
    prevDebugLenRef.current = runLog.length
  }, [latestRun?.debug_log])
  // Reset tracking when run changes.
  const prevRunIdRef = useRef<string | null>(null)
  useEffect(() => {
    if (latestRunId && latestRunId !== prevRunIdRef.current) {
      prevDebugLenRef.current = 0
      // Load initial debug log from the new run.
      const runLog = latestRun?.debug_log || []
      if (runLog.length > 0) {
        setDebugEntries((prev) => [...prev, ...runLog])
        prevDebugLenRef.current = runLog.length
      }
    }
    prevRunIdRef.current = latestRunId
  }, [latestRunId, latestRun?.debug_log])
  const clearDebug = useCallback(() => setDebugEntries([]), [])

  const nodeTypeDefs = useMemo(() => {
    const map: Record<string, NodeTypeDef> = {}
    if (nodeTypeCatalog?.types) {
      for (const t of nodeTypeCatalog.types) {
        map[t.type] = t
      }
    }
    return map
  }, [nodeTypeCatalog])

  // When a node is selected, auto-open the config tab.
  const handleNodeSelect = useCallback((node: Node | null) => {
    setSelectedNode(node)
    if (node) setRightTab('config')
  }, [])

  const handleSave = useCallback(async () => {
    if (!flow || !id || !canvasRef.current) return
    const { nodes, edges } = canvasRef.current.getFlowData()
    try {
      await updateFlow.mutateAsync({
        id,
        data: { name: flow.name, description: flow.description, nodes, edges, settings: flow.settings, trigger: flow.trigger, inputs: flow.inputs },
      })
      setDirty(false)
      setValidationErrors([])
    } catch (err: any) {
      if (err.message) setValidationErrors([err.message])
    }
  }, [flow, id, updateFlow])

  const handleNameChange = useCallback(async (name: string) => {
    if (!flow || !id) return
    try {
      await updateFlow.mutateAsync({
        id,
        data: { ...flow, name },
      })
    } catch (err: any) {
      if (err.message) setValidationErrors([err.message])
    }
  }, [flow, id, updateFlow])

  const handleValidate = useCallback(async () => {
    if (!canvasRef.current) return
    const { nodes, edges } = canvasRef.current.getFlowData()
    try {
      const result = await validateFlow.mutateAsync({ nodes, edges })
      setValidationErrors(result.errors || [])
    } catch (err: any) {
      setValidationErrors([err.message])
    }
  }, [validateFlow])

  const handleNodeConfigChange = useCallback((nodeId: string, data: Record<string, unknown>) => {
    canvasRef.current?.updateNodeData(nodeId, data)
    setSelectedNode((prev) => prev && prev.id === nodeId ? { ...prev, data: { ...prev.data, ...data } } : prev)
  }, [])

  const handleAutoLayout = useCallback(() => {
    canvasRef.current?.doAutoLayout()
  }, [])

  if (isLoading) {
    return (
      <div className="flex items-center justify-center" style={{ height: 'calc(100vh - 4rem)' }}>
        <div className="text-xs text-muted-foreground">Loading flow...</div>
      </div>
    )
  }

  if (!flow) {
    return (
      <div className="flex items-center justify-center" style={{ height: 'calc(100vh - 4rem)' }}>
        <div className="text-xs text-muted-foreground">Flow not found</div>
      </div>
    )
  }

  const showRightPanel = rightTab !== null

  return (
    <div className="flex flex-col" style={{ height: 'calc(100vh - 4rem)' }}>
      <FlowToolbar
        flowName={flow.name}
        onNameChange={handleNameChange}
        onSave={handleSave}
        onValidate={handleValidate}
        onAutoLayout={handleAutoLayout}
        saving={updateFlow.isPending}
        validationErrors={validationErrors}
        dirty={dirty}
        wsStatus={wsStatus}
        totalRuns={totalRuns}
        latestRunStatus={latestRun?.status}
      />

      <div className="flex flex-1 overflow-hidden relative">
        <NodePalette />

        <FlowCanvas
          ref={canvasRef}
          initialNodes={flow.nodes || []}
          initialEdges={flow.edges || []}
          nodeTypeDefs={nodeTypeDefs}
          nodeStates={nodeStates}
          onNodeSelect={handleNodeSelect}
          onDirty={() => setDirty(true)}
        />

        {/* Right sidebar: tab bar + content */}
        <div className="flex shrink-0">
          {/* Tab bar (always visible) */}
          <div className="flex flex-col items-center gap-1 py-2 px-1 border-l border-border bg-card">
            <TabButton
              active={rightTab === 'config'}
              onClick={() => setRightTab(rightTab === 'config' ? null : 'config')}
              title="Node Config"
              badge={selectedNode ? 1 : 0}
            >
              <Settings2 className="w-4 h-4" />
            </TabButton>
            <TabButton
              active={rightTab === 'debug'}
              onClick={() => setRightTab(rightTab === 'debug' ? null : 'debug')}
              title="Debug"
              badge={debugEntries.length}
            >
              <Bug className="w-4 h-4" />
            </TabButton>
          </div>

          {/* Panel content */}
          {showRightPanel && (
            <>
              {rightTab === 'config' && (
                <NodeConfig
                  node={selectedNode}
                  nodeTypeDefs={nodeTypeDefs}
                  onChange={handleNodeConfigChange}
                  onClose={() => setRightTab(null)}
                />
              )}
              {rightTab === 'debug' && (
                <DebugPanel entries={debugEntries} onClear={clearDebug} />
              )}
            </>
          )}
        </div>

        <ExecutionOverlay
          run={latestRun}
          onClose={() => {}}
        />
      </div>
    </div>
  )
}

function TabButton({
  active,
  onClick,
  title,
  badge,
  children,
}: {
  active: boolean
  onClick: () => void
  title: string
  badge?: number
  children: React.ReactNode
}) {
  return (
    <button
      onClick={onClick}
      title={title}
      className={cn(
        'relative p-1.5 rounded transition-colors',
        active ? 'bg-accent text-foreground' : 'text-muted-foreground hover:text-foreground hover:bg-accent/50',
      )}
    >
      {children}
      {badge != null && badge > 0 && (
        <span className="absolute -top-0.5 -right-0.5 min-w-[14px] h-[14px] flex items-center justify-center text-[8px] font-mono bg-amber-500 text-white rounded-full px-0.5">
          {badge > 99 ? '99+' : badge}
        </span>
      )}
    </button>
  )
}
