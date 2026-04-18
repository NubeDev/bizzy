import { useState, useCallback, useMemo, useRef } from 'react'
import { useParams } from 'react-router-dom'
import { useFlow, useUpdateFlow, useValidateFlow, useNodeTypes, useLatestFlowRun } from '@/hooks/use-flows'
import { FlowCanvas, type FlowCanvasHandle } from '@/components/flow/canvas'
import { NodePalette } from '@/components/flow/node-palette'
import { NodeConfig } from '@/components/flow/node-config'
import { FlowToolbar, type PollInterval } from '@/components/flow/flow-toolbar'
import { ExecutionOverlay, useNodeStatesForCanvas } from '@/components/flow/execution-overlay'
import type { Node } from '@xyflow/react'
import type { NodeTypeDef } from '@/lib/types'

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
  const [pollInterval, setPollInterval] = useState<PollInterval>(5000)

  // Poll latest run for this flow.
  const { run: latestRun, runId: latestRunId, totalRuns } = useLatestFlowRun(id || '', pollInterval)
  const nodeStates = useNodeStatesForCanvas(latestRun)

  const nodeTypeDefs = useMemo(() => {
    const map: Record<string, NodeTypeDef> = {}
    if (nodeTypeCatalog?.types) {
      for (const t of nodeTypeCatalog.types) {
        map[t.type] = t
      }
    }
    return map
  }, [nodeTypeCatalog])

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
        pollInterval={pollInterval}
        onPollIntervalChange={setPollInterval}
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
          onNodeSelect={setSelectedNode}
          onDirty={() => setDirty(true)}
        />

        {selectedNode && (
          <NodeConfig
            node={selectedNode}
            onChange={handleNodeConfigChange}
            onClose={() => setSelectedNode(null)}
          />
        )}

        <ExecutionOverlay
          runId={latestRunId}
          onClose={() => {}}
        />
      </div>
    </div>
  )
}
