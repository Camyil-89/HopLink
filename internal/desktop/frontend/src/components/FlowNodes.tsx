import { useRef, useState } from 'react'
import {
  BaseEdge,
  EdgeLabelRenderer,
  getSmoothStepPath,
  Handle,
  Position,
  type EdgeProps,
  type NodeProps,
} from '@xyflow/react'
import { Server as ServerIcon, X } from 'lucide-react'
import type { Server, ServerServiceStatus } from '../types/app'

export function ServerNode({ data, selected }: NodeProps) {
  const server = (data as { server: Server }).server
  const role = (data as { role: string }).role
  const hasIn = (data as { hasIn: boolean }).hasIn
  const hasOut = (data as { hasOut: boolean }).hasOut
  const serviceStatus = (data as { serviceStatus?: ServerServiceStatus }).serviceStatus

  return (
    <div className={`server-node ${selected ? 'selected' : ''} ${serviceStatus ? (serviceStatus.running ? 'service-running' : 'service-stopped') : ''}`} title={serviceStatus?.error || (serviceStatus?.running ? 'Сервис работает' : undefined)}>
      <Handle type="target" position={Position.Left} isConnectable={!hasIn} />
      <span className="node-icon"><ServerIcon size={16} /></span>
      <span className="node-text">
        <b>{server.name || server.host}</b>
        <small>{server.host}:{server.sshPort}</small>
        {serviceStatus && <small className={`node-version ${serviceStatus.versionCurrent === undefined ? 'unknown' : serviceStatus.versionCurrent ? 'current' : 'outdated'}`}>
          {serviceStatus.version
            ? `v${serviceStatus.version} · ${serviceStatus.version === 'dev' ? 'сборка из исходников' : serviceStatus.versionCurrent === undefined ? 'актуальность неизвестна' : serviceStatus.versionCurrent ? 'актуальна' : 'устарела'}`
            : 'версия неизвестна'}
        </small>}
      </span>
      <span className={`node-role ${role}`}>
        {role === 'entry' ? 'ВХОДНАЯ ТОЧКА' : role === 'exit' ? 'ВЫХОДНАЯ ТОЧКА' : 'УЗЕЛ ЦЕПОЧКИ'}
      </span>
      <Handle type="source" position={Position.Right} isConnectable={!hasOut} />
    </div>
  )
}

export function DeletableEdge(props: EdgeProps) {
  const [hovered, setHovered] = useState(false)
  const hideTimer = useRef<number>()
  const enter = () => {
    window.clearTimeout(hideTimer.current)
    setHovered(true)
  }
  const leave = () => {
    window.clearTimeout(hideTimer.current)
    hideTimer.current = window.setTimeout(() => setHovered(false), 180)
  }
  const [path, labelX, labelY] = getSmoothStepPath({
    sourceX: props.sourceX,
    sourceY: props.sourceY,
    targetX: props.targetX,
    targetY: props.targetY,
    sourcePosition: props.sourcePosition,
    targetPosition: props.targetPosition,
  })
  const onDelete = (props.data as { onDelete?: () => void } | undefined)?.onDelete

  return (
    <>
      <g onMouseEnter={enter} onMouseLeave={leave}>
        <BaseEdge
          id={props.id}
          path={path}
          style={props.style}
          markerEnd={props.markerEnd}
          interactionWidth={48}
        />
      </g>
      <EdgeLabelRenderer>
        <button
          className="edge-delete-button nodrag nopan"
          aria-label="Удалить связь"
          title="Удалить связь"
          style={{
            transform: `translate(-50%,-50%) translate(${labelX}px,${labelY}px)`,
            opacity: hovered || props.selected ? 1 : 0,
            pointerEvents: hovered || props.selected ? 'all' : 'none',
          }}
          onMouseEnter={enter}
          onMouseLeave={leave}
          onClick={event => {
            event.stopPropagation()
            onDelete?.()
          }}
        >
          <X size={13} />
        </button>
      </EdgeLabelRenderer>
    </>
  )
}

export const nodeTypes = { server: ServerNode }
export const edgeTypes = { deletable: DeletableEdge }
