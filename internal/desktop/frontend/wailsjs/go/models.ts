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
	    knownHosts: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
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
	
	
	export class ServerServiceStatus {
	    serverId: string;
	    running: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ServerServiceStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.serverId = source["serverId"];
	        this.running = source["running"];
	        this.error = source["error"];
	    }
	}
	export class RouteServiceStatus {
	    routeId: string;
	    running: number;
	    total: number;
	    servers: ServerServiceStatus[];
	
	    static createFrom(source: any = {}) {
	        return new RouteServiceStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.routeId = source["routeId"];
	        this.running = source["running"];
	        this.total = source["total"];
	        this.servers = this.convertValues(source["servers"], ServerServiceStatus);
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

