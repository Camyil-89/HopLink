import type { Edge, Node } from '@xyflow/react'
import type { Edge as RouteEdge, Server } from '../types/app'

export const initialNodes = (servers: Server[]): Node[] => servers.map(server => ({
  id: server.id,
  position: { x: Math.round((server.x ?? 140) / 20) * 20, y: Math.round((server.y ?? 140) / 20) * 20 },
  data: { server },
  type: 'server',
}))

export function routeSnapshot(
  servers: Server[],
  connections: RouteEdge[],
  settings: { sni: string; port: number; entryDomain: string; chainKey: string; certificate: string; certificateKey: string },
) {
  return JSON.stringify({
    nodes: servers.map(server => ({ id: server.id, x: Math.round((server.x ?? 0) / 20) * 20, y: Math.round((server.y ?? 0) / 20) * 20 })).sort((a, b) => a.id.localeCompare(b.id)),
    edges: connections.map(edge => ({ source: edge.source, target: edge.target })).sort((a, b) => `${a.source}:${a.target}`.localeCompare(`${b.source}:${b.target}`)),
    settings: {
      sni: settings.sni,
      port: settings.port,
      entryDomain: settings.entryDomain,
      chainKey: settings.chainKey,
      certificate: settings.certificate,
      certificateKey: settings.certificateKey,
    },
  })
}

export function toRouteEdges(edges: Edge[]): RouteEdge[] {
  return edges.map(edge => ({ id: edge.id, source: edge.source, target: edge.target }))
}
