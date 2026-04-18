import { useCallback, useRef, useMemo, useEffect, useImperativeHandle, forwardRef } from 'react'
import {
  ReactFlow,
  MiniMap,
  Controls,
  Background,
  BackgroundVariant,
  useNodesState,
  useEdgesState,
  addEdge,
  type Connection,
  type Edge,
  type Node,
  type NodeTypes,
  type OnConnect,
  ReactFlowProvider,
  useReactFlow,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import dagre from '@dagrejs/dagre'
import { BaseNode, type BaseNodeData } from './custom-nodes/base-node'
import type { FlowNodeDef, FlowEdgeDef, NodeTypeDef, NodeState } from '@/lib/types'

const nodeTypes: NodeTypes = {
  flowNode: BaseNode,
}

export interface FlowCanvasHandle {
  doAutoLayout: () => void
  getFlowData: () => { nodes: FlowNodeDef[]; edges: FlowEdgeDef[] }
  updateNodeData: (nodeId: string, data: Record<string, unknown>) => void
}

interface FlowCanvasProps {
  initialNodes: FlowNodeDef[]
  initialEdges: FlowEdgeDef[]
  nodeTypeDefs: Record<string, NodeTypeDef>
  nodeStates?: Record<string, NodeState>
  onNodeSelect?: (node: Node | null) => void
  onDirty?: () => void
}

function toReactFlowNodes(
  flowNodes: FlowNodeDef[],
  nodeTypeDefs: Record<string, NodeTypeDef>,
  nodeStates?: Record<string, NodeState>,
): Node[] {
  return flowNodes.map((n) => {
    const typeDef = nodeTypeDefs[n.type]
    const state = nodeStates?.[n.id]
    return {
      id: n.id,
      type: 'flowNode',
      position: { x: n.position.x, y: n.position.y },
      data: {
        label: n.label || typeDef?.label || n.type,
        nodeType: n.type,
        category: typeDef?.category || 'tool',
        source: typeDef?.source || 'builtin',
        description: typeDef?.description,
        inputPorts: n.ports?.inputs || typeDef?.ports?.inputs || [],
        outputPorts: n.ports?.outputs || typeDef?.ports?.outputs || [],
        config: n.data || {},
        status: state?.status,
        error: state?.error,
        duration_ms: state?.duration_ms,
        inputValue: state?.input,
        outputValue: state?.output,
      } satisfies BaseNodeData,
    }
  })
}

function toReactFlowEdges(flowEdges: FlowEdgeDef[]): Edge[] {
  return flowEdges.map((e) => ({
    id: e.id,
    source: e.source,
    sourceHandle: e.sourceHandle,
    target: e.target,
    targetHandle: e.targetHandle,
    label: e.label || e.condition || undefined,
    animated: false,
    style: { stroke: 'var(--muted-foreground)', strokeWidth: 1.5 },
  }))
}

// Convert React Flow nodes/edges back to FlowDef format
function rfToFlowDef(nodes: Node[], edges: Edge[]): { nodes: FlowNodeDef[]; edges: FlowEdgeDef[] } {
  return {
    nodes: nodes.map((n) => {
      const data = n.data as BaseNodeData
      return {
        id: n.id,
        type: data.nodeType || n.type || 'unknown',
        label: data.label || '',
        position: { x: n.position.x, y: n.position.y },
        data: data.config || {},
      }
    }),
    edges: edges.map((e) => ({
      id: e.id,
      source: e.source,
      sourceHandle: e.sourceHandle || 'output',
      target: e.target,
      targetHandle: e.targetHandle || 'input',
      label: typeof e.label === 'string' ? e.label : '',
    })),
  }
}

// Dagre auto-layout
function autoLayout(nodes: Node[], edges: Edge[]): Node[] {
  const g = new dagre.graphlib.Graph()
  g.setDefaultEdgeLabel(() => ({}))
  g.setGraph({ rankdir: 'LR', nodesep: 50, ranksep: 80 })

  nodes.forEach((n) => {
    g.setNode(n.id, { width: 180, height: 80 })
  })
  edges.forEach((e) => {
    g.setEdge(e.source, e.target)
  })

  dagre.layout(g)

  return nodes.map((n) => {
    const pos = g.node(n.id)
    return {
      ...n,
      position: { x: pos.x - 90, y: pos.y - 40 },
    }
  })
}

const FlowCanvasInner = forwardRef<FlowCanvasHandle, FlowCanvasProps>(function FlowCanvasInner(
  { initialNodes: flowNodes, initialEdges: flowEdges, nodeTypeDefs, nodeStates, onNodeSelect, onDirty },
  ref,
) {
  const { screenToFlowPosition } = useReactFlow()

  const rfNodes = useMemo(() => toReactFlowNodes(flowNodes, nodeTypeDefs, nodeStates), [flowNodes, nodeTypeDefs, nodeStates])
  const rfEdges = useMemo(() => toReactFlowEdges(flowEdges), [flowEdges])

  const [nodes, setNodes, onNodesChange] = useNodesState(rfNodes)
  const [edges, setEdges, onEdgesChange] = useEdgesState(rfEdges)

  // Merge execution state into nodes when polling updates.
  // Compares by JSON to avoid creating new node objects when nothing changed.
  const prevNodeStatesJson = useRef<Record<string, string>>({})
  useEffect(() => {
    if (!nodeStates) return
    setNodes((nds) => {
      let changed = false
      const next = nds.map((n) => {
        const state = nodeStates[n.id]
        const stateJson = state ? JSON.stringify(state) : ''
        if (stateJson === (prevNodeStatesJson.current[n.id] || '')) return n
        prevNodeStatesJson.current[n.id] = stateJson
        changed = true
        return {
          ...n,
          data: {
            ...n.data,
            status: state?.status,
            error: state?.error,
            duration_ms: state?.duration_ms,
            inputValue: state?.input,
            outputValue: state?.output,
          },
        }
      })
      return changed ? next : nds
    })
  }, [nodeStates, setNodes])

  // Expose handle to parent
  useImperativeHandle(ref, () => ({
    doAutoLayout: () => {
      setNodes((nds) => autoLayout(nds, edges))
      onDirty?.()
    },
    getFlowData: () => rfToFlowDef(nodes, edges),
    updateNodeData: (nodeId: string, data: Record<string, unknown>) => {
      setNodes((nds) =>
        nds.map((n) => (n.id === nodeId ? { ...n, data: { ...n.data, ...data } } : n)),
      )
      onDirty?.()
    },
  }), [nodes, edges, setNodes, onDirty])

  const onConnect: OnConnect = useCallback(
    (connection: Connection) => {
      setEdges((eds) => addEdge({ ...connection, style: { stroke: 'var(--muted-foreground)', strokeWidth: 1.5 } }, eds))
      onDirty?.()
    },
    [setEdges, onDirty],
  )

  const handleNodesChange = useCallback(
    (changes: Parameters<typeof onNodesChange>[0]) => {
      onNodesChange(changes)
      if (changes.some((c) => c.type === 'position' || c.type === 'remove')) {
        onDirty?.()
      }
    },
    [onNodesChange, onDirty],
  )

  const handleEdgesChange = useCallback(
    (changes: Parameters<typeof onEdgesChange>[0]) => {
      onEdgesChange(changes)
      if (changes.some((c) => c.type === 'remove')) {
        onDirty?.()
      }
    },
    [onEdgesChange, onDirty],
  )

  const onNodeClick = useCallback(
    (_: React.MouseEvent, node: Node) => {
      onNodeSelect?.(node)
    },
    [onNodeSelect],
  )

  const onPaneClick = useCallback(() => {
    onNodeSelect?.(null)
  }, [onNodeSelect])

  const onDragOver = useCallback((event: React.DragEvent) => {
    event.preventDefault()
    event.dataTransfer.dropEffect = 'move'
  }, [])

  const onDrop = useCallback(
    (event: React.DragEvent) => {
      event.preventDefault()
      const typeData = event.dataTransfer.getData('application/flow-node-type')
      if (!typeData) return

      const nodeDef: NodeTypeDef = JSON.parse(typeData)
      const position = screenToFlowPosition({
        x: event.clientX,
        y: event.clientY,
      })

      const newNode: Node = {
        id: `${nodeDef.type}-${Date.now()}`,
        type: 'flowNode',
        position,
        data: {
          label: nodeDef.label,
          nodeType: nodeDef.type,
          category: nodeDef.category,
          source: nodeDef.source,
          description: nodeDef.description,
          inputPorts: nodeDef.ports?.inputs || [],
          outputPorts: nodeDef.ports?.outputs || [],
          config: {},
        } satisfies BaseNodeData,
      }

      setNodes((nds) => [...nds, newNode])
      onDirty?.()
    },
    [screenToFlowPosition, setNodes, onDirty],
  )

  return (
    <div className="flex-1 h-full">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={handleNodesChange}
        onEdgesChange={handleEdgesChange}
        onConnect={onConnect}
        onNodeClick={onNodeClick}
        onPaneClick={onPaneClick}
        onDragOver={onDragOver}
        onDrop={onDrop}
        nodeTypes={nodeTypes}
        fitView
        deleteKeyCode="Delete"
        className="bg-background"
      >
        <Controls
          position="bottom-left"
          className="!bg-card !border-border !shadow-sm [&>button]:!bg-card [&>button]:!border-border [&>button]:!fill-foreground"
        />
        <MiniMap
          position="bottom-right"
          className="!bg-card !border-border"
          nodeColor="var(--muted-foreground)"
          maskColor="rgba(0,0,0,0.1)"
        />
        <Background variant={BackgroundVariant.Dots} gap={16} size={0.5} color="var(--muted-foreground)" />
      </ReactFlow>
    </div>
  )
})

// Wrapper with ReactFlowProvider — forwards ref through
export const FlowCanvas = forwardRef<FlowCanvasHandle, FlowCanvasProps>(function FlowCanvas(props, ref) {
  return (
    <ReactFlowProvider>
      <FlowCanvasInner ref={ref} {...props} />
    </ReactFlowProvider>
  )
})
