import type { DragEvent } from 'react'
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  type Connection,
  type Edge as FlowEdge,
  type Node as FlowNode,
  type OnEdgesChange,
  type OnNodesChange,
} from '@xyflow/react'
import { ArrowDownToLine, ArrowRight, Cloud, Plus, Server as ServerIcon } from 'lucide-react'
import { edgeTypes, nodeTypes } from '../components/FlowNodes'
import { MapViewport } from '../components/MapViewport'
import type { AppState, Server } from '../types/app'

type Props = {
  state: AppState
  nodes: FlowNode[]
  flowNodes: FlowNode[]
  edges: FlowEdge[]
  servers: Server[]
  search: string
  dirty: boolean
  busy: boolean
  fitRevision: number
  onSearch: (value: string) => void
  onNavigateServers: () => void
  onAddServer: (server: Server) => void
  onDrop: (event: DragEvent<HTMLDivElement>) => void
  onClear: () => void
  onNodeDragStop: (node: FlowNode) => void
  onNodesChange: OnNodesChange
  onEdgesChange: OnEdgesChange
  onConnect: (connection: Connection) => void
  canConnect: (connection: Connection | FlowEdge) => boolean
  onEdgeClick: (edge: FlowEdge) => void
  onNodeClick: (event: unknown, node: FlowNode) => void
  onPaneClick: () => void
  onSave: () => void
  onApply: () => void
}

export function MapTab(props: Props) {
  return (
    <div className="map-layout">
      <section className="server-library">
        <div className="library-head">
          <div><h3>Мои серверы</h3><small>{props.servers.length} доступны для добавления</small></div>
          <button className="icon-btn add-icon" onClick={props.onNavigateServers} title="Список серверов"><Plus size={17} /></button>
        </div>
        <div className="search-box"><span>⌕</span><input placeholder="Найти сервер…" value={props.search} onChange={event => props.onSearch(event.target.value)} /></div>
        <div className="library-list">
          {props.servers.map(server => (
            <div key={server.id} className="server-card" draggable onDragStart={event => event.dataTransfer.setData('application/hoplink-server', server.id)} onClick={() => props.onAddServer(server)}>
              <div className="server-symbol"><ServerIcon size={16} /></div>
              <div className="server-info"><b>{server.name}</b><small>{server.host}:{server.sshPort}</small></div>
              <span className="drag-dots">⠿</span>
            </div>
          ))}
          {props.servers.length === 0 && <div className="empty-small">Все серверы на карте.<br />Добавьте сервер через +</div>}
        </div>
        <div className="library-foot"><span className="online-dot" /> {props.state.servers.length} серверов в библиотеке</div>
      </section>

      <section className="canvas-panel">
        <div className="canvas-toolbar"><div className="canvas-tools">
          <button className="map-tool-button danger" onClick={props.onClear} disabled={!props.nodes.length && !props.edges.length}>Очистить карту</button>
          <span className="tool-sep" />
          <span className="hint">Перетащите сервер или соедините точки</span>
          <div className="head-actions">
            <button className="btn secondary" onClick={props.onSave} disabled={!props.dirty}>{props.dirty ? 'Сохранить изменения' : 'Маршрут сохранён'}</button>
            <button className="btn primary" onClick={props.onApply} disabled={props.busy}>{props.busy ? 'Синхронизация…' : 'Применить маршрут'}</button>
          </div>
        </div></div>

        <div className="flow-wrap" onDrop={props.onDrop} onDragOver={event => event.preventDefault()}>
          <ReactFlow
            nodes={props.flowNodes}
            edges={props.edges}
            snapToGrid
            snapGrid={[20, 20]}
            onNodeDragStop={(_, node) => props.onNodeDragStop(node)}
            nodeTypes={nodeTypes}
            edgeTypes={edgeTypes}
            nodesConnectable
            onNodesChange={props.onNodesChange}
            onEdgesChange={props.onEdgesChange}
            onConnect={props.onConnect}
            isValidConnection={props.canConnect}
            connectionRadius={72}
            edgesFocusable
            elementsSelectable
            onEdgeClick={(_, edge) => props.onEdgeClick(edge)}
            onNodeClick={props.onNodeClick}
            onPaneClick={props.onPaneClick}
            fitView
            deleteKeyCode={['Backspace', 'Delete']}
            proOptions={{ hideAttribution: true }}
          >
            <Background color="#dfe3ed" gap={20} size={1} />
            <MapViewport nodes={props.flowNodes} edges={props.edges} revision={props.fitRevision} />
            <Controls />
            <MiniMap nodeColor={node => (node.data as any)?.role === 'entry' ? '#50b486' : (node.data as any)?.role === 'exit' ? '#e6a367' : '#7380e6'} maskColor="rgba(244,246,250,.55)" />
          </ReactFlow>
          {props.nodes.length === 0 && <div className="canvas-empty">
            <div className="empty-graphic"><span><ServerIcon /></span><ArrowRight /><span><Cloud /></span></div>
            <b>Начните строить цепочку</b>
            <p>Перетащите сервер из библиотеки слева<br />или нажмите на него, чтобы добавить.</p>
            <button className="text-btn" onClick={props.onNavigateServers}><Plus size={14} /> Добавить сервер</button>
          </div>}
        </div>
        <div className="canvas-footer">
          <span><i className="legend-dot entry-dot" /> Входная точка</span>
          <span><i className="legend-dot relay-dot" /> Промежуточный узел</span>
          <span><i className="legend-dot exit-dot" /> Выходная точка</span>
          <span className="footer-tip"><ArrowDownToLine size={13} /> Перетаскивайте из списка</span>
        </div>
      </section>
    </div>
  )
}
