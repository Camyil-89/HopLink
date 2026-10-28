package hoplink

import hyserver "github.com/apernet/hysteria/core/v2/server"

// Transport is the extension point for outbound relay implementations.
// Implementations satisfy the Hysteria2 server's outbound contract.
type Transport interface {
	hyserver.Outbound
	Close()
}

var _ Transport = (*hy2ClientManager)(nil)
