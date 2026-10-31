export namespace desktop {
	
	export class Route {
	    id: string;
	    name: string;
	    index: number;
	    legacyLayout?: boolean;
	    legacyIdLayout?: boolean;
	    entryDomain?: string;
	    mapServers: Server[];
	    deployedServers?: Server[];
	    users: User[];
	    edges: Edge[];
	    sni: string;
	    port: number;
	    chainKey: string;
	    certificate: string;
	    certificateKey: string;
	    logLevel?: string;
	    obfsType?: string;
	    obfsPassword?: string;
	    relayTransport?: string;
	    relayTransportClientKey?: string;
	    relayTransportClientPub?: string;
	    relayTransportHostKey?: string;
	    relayTransportHostPub?: string;
	
	    static createFrom(source: any = {}) {
	        return new Route(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.index = source["index"];
	        this.legacyLayout = source["legacyLayout"];
	        this.legacyIdLayout = source["legacyIdLayout"];
	        this.entryDomain = source["entryDomain"];
	        this.mapServers = this.convertValues(source["mapServers"], Server);
	        this.deployedServers = this.convertValues(source["deployedServers"], Server);
	        this.users = this.convertValues(source["users"], User);
	        this.edges = this.convertValues(source["edges"], Edge);
	        this.sni = source["sni"];
	        this.port = source["port"];
	        this.chainKey = source["chainKey"];
	        this.certificate = source["certificate"];
	        this.certificateKey = source["certificateKey"];
	        this.logLevel = source["logLevel"];
	        this.obfsType = source["obfsType"];
	        this.obfsPassword = source["obfsPassword"];
	        this.relayTransport = source["relayTransport"];
	        this.relayTransportClientKey = source["relayTransportClientKey"];
	        this.relayTransportClientPub = source["relayTransportClientPub"];
	        this.relayTransportHostKey = source["relayTransportHostKey"];
	        this.relayTransportHostPub = source["relayTransportHostPub"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Edge {
	    id: string;
	    source: string;
	    target: string;
	
	    static createFrom(source: any = {}) {
	        return new Edge(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.source = source["source"];
	        this.target = source["target"];
	    }
	}
	export class User {
	    id: string;
	    name: string;
	    password: string;
	    link: string;
	    active?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new User(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.password = source["password"];
	        this.link = source["link"];
	        this.active = source["active"];
	    }
	}
	export class Server {
	    id: string;
	    name: string;
	    host: string;
	    sshPort: number;
	    sshUser: string;
	    sshPassword: string;
	    sshPrivateKey?: string;
	    sshKeyPassphrase?: string;
	    x: number;
	    y: number;
	    role?: string;
	
	    static createFrom(source: any = {}) {
	        return new Server(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.host = source["host"];
	        this.sshPort = source["sshPort"];
	        this.sshUser = source["sshUser"];
	        this.sshPassword = source["sshPassword"];
	        this.sshPrivateKey = source["sshPrivateKey"];
	        this.sshKeyPassphrase = source["sshKeyPassphrase"];
	        this.x = source["x"];
	        this.y = source["y"];
	        this.role = source["role"];
	    }
	}
	export class AppConfig {
	    installSource?: string;
	    servers: Server[];
	    mapServers: Server[];
	    deployedServers?: Server[];
	    users: User[];
	    edges: Edge[];
	    routes: Route[];
	    activeRouteId: string;
	    entryDomain: string;
	    sni: string;
	    port: number;
	    chainKey: string;
	    certificate: string;
	    certificateKey: string;
	    certificateLifetime?: string;
	    logLevel?: string;
	    obfsType?: string;
	    obfsPassword?: string;
	    relayTransport?: string;
	    relayTransportClientKey?: string;
	    relayTransportClientPub?: string;
	    relayTransportHostKey?: string;
	    relayTransportHostPub?: string;
	    knownHosts: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.installSource = source["installSource"];
	        this.servers = this.convertValues(source["servers"], Server);
	        this.mapServers = this.convertValues(source["mapServers"], Server);
	        this.deployedServers = this.convertValues(source["deployedServers"], Server);
	        this.users = this.convertValues(source["users"], User);
	        this.edges = this.convertValues(source["edges"], Edge);
	        this.routes = this.convertValues(source["routes"], Route);
	        this.activeRouteId = source["activeRouteId"];
	        this.entryDomain = source["entryDomain"];
	        this.sni = source["sni"];
	        this.port = source["port"];
	        this.chainKey = source["chainKey"];
	        this.certificate = source["certificate"];
	        this.certificateKey = source["certificateKey"];
	        this.certificateLifetime = source["certificateLifetime"];
	        this.logLevel = source["logLevel"];
	        this.obfsType = source["obfsType"];
	        this.obfsPassword = source["obfsPassword"];
	        this.relayTransport = source["relayTransport"];
	        this.relayTransportClientKey = source["relayTransportClientKey"];
	        this.relayTransportClientPub = source["relayTransportClientPub"];
	        this.relayTransportHostKey = source["relayTransportHostKey"];
	        this.relayTransportHostPub = source["relayTransportHostPub"];
	        this.knownHosts = source["knownHosts"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DesktopUpdate {
	    installed: string;
	    latest: string;
	    available: boolean;
	    url: string;
	    digest: string;
	
	    static createFrom(source: any = {}) {
	        return new DesktopUpdate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.installed = source["installed"];
	        this.latest = source["latest"];
	        this.available = source["available"];
	        this.url = source["url"];
	        this.digest = source["digest"];
	    }
	}
	
	
	export class ServerServiceStatus {
	    serverId: string;
	    running: boolean;
	    version?: string;
	    latestVersion?: string;
	    versionCurrent?: boolean;
	    error?: string;
	    nextServerId?: string;
	    nextLatencyMs?: number;
	    nextPingError?: string;
	
	    static createFrom(source: any = {}) {
	        return new ServerServiceStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.serverId = source["serverId"];
	        this.running = source["running"];
	        this.version = source["version"];
	        this.latestVersion = source["latestVersion"];
	        this.versionCurrent = source["versionCurrent"];
	        this.error = source["error"];
	        this.nextServerId = source["nextServerId"];
	        this.nextLatencyMs = source["nextLatencyMs"];
	        this.nextPingError = source["nextPingError"];
	    }
	}
	export class RouteServiceStatus {
	    routeId: string;
	    running: number;
	    total: number;
	    servers: ServerServiceStatus[];
	    entryLatencyMs?: number;
	    entryPingError?: string;
	
	    static createFrom(source: any = {}) {
	        return new RouteServiceStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.routeId = source["routeId"];
	        this.running = source["running"];
	        this.total = source["total"];
	        this.servers = this.convertValues(source["servers"], ServerServiceStatus);
	        this.entryLatencyMs = source["entryLatencyMs"];
	        this.entryPingError = source["entryPingError"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	export class VaultStatus {
	    locked: boolean;
	    needsSetup: boolean;
	    hasData: boolean;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new VaultStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.locked = source["locked"];
	        this.needsSetup = source["needsSetup"];
	        this.hasData = source["hasData"];
	        this.error = source["error"];
	    }
	}

}

