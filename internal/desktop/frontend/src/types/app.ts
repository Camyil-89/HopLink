export type Server = {
  id: string
  name: string
  host: string
  sshPort: number
  sshUser: string
  sshPassword: string
  sshPrivateKey?: string
  sshKeyPassphrase?: string
  x: number
  y: number
  role?: string
}

export type User = { id: string; name: string; password: string; link: string; active?: boolean }
export type Edge = { id: string; source: string; target: string }
export type ServerServiceStatus = { serverId: string; running: boolean; error?: string }
export type RouteServiceStatus = { routeId: string; running: number; total: number; servers: ServerServiceStatus[] }

export type Route = {
  id: string
  index: number
  name: string
  mapServers: Server[]
  deployedServers?: Server[]
  users: User[]
  edges: Edge[]
  sni: string
  port: number
  entryDomain?: string
  chainKey: string
  certificate: string
  certificateKey: string
}

export type AppState = {
  installSource?: string
  servers: Server[]
  mapServers: Server[]
  deployedServers?: Server[]
  users: User[]
  edges: Edge[]
  routes: Route[]
  activeRouteId: string
  sni: string
  port: number
  entryDomain: string
  chainKey: string
  certificate: string
  certificateKey: string
  knownHosts: Record<string, string>
}

export type VaultStatus = {
  locked: boolean
  needsSetup: boolean
  hasData: boolean
  error: string
}

export const emptyState: AppState = {
  servers: [],
  mapServers: [],
  users: [],
  edges: [],
  routes: [],
  activeRouteId: '',
  sni: 'www.cloudflare.com',
  port: 22033,
  entryDomain: '',
  chainKey: '',
  certificate: '',
  certificateKey: '',
  knownHosts: {},
}
