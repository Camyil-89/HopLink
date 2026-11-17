package desktop

// applyServerLibrary copies connection fields from the library entry onto a map/deployed copy.
// Map position and role are preserved.
func applyServerLibrary(library Server, onto Server) Server {
	onto.Name = library.Name
	onto.Host = library.Host
	onto.SSHPort = library.SSHPort
	onto.SSHUser = library.SSHUser
	onto.SSHPassword = library.SSHPassword
	onto.SSHPrivateKey = library.SSHPrivateKey
	onto.SSHKeyPassphrase = library.SSHKeyPassphrase
	return onto
}

func syncServerSlice(libraryByID map[string]Server, list []Server) []Server {
	if len(list) == 0 {
		return list
	}
	out := make([]Server, len(list))
	for i, mapped := range list {
		if lib, ok := libraryByID[mapped.ID]; ok {
			out[i] = applyServerLibrary(lib, mapped)
		} else {
			out[i] = mapped
		}
	}
	return out
}

func (a *App) syncServersLibraryLocked() {
	byID := make(map[string]Server, len(a.config.Servers))
	for _, s := range a.config.Servers {
		byID[s.ID] = s
	}
	a.config.MapServers = syncServerSlice(byID, a.config.MapServers)
	a.config.DeployedServers = syncServerSlice(byID, a.config.DeployedServers)
	for i := range a.config.Routes {
		route := &a.config.Routes[i]
		route.MapServers = syncServerSlice(byID, route.MapServers)
		route.DeployedServers = syncServerSlice(byID, route.DeployedServers)
	}
}