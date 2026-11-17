package cascade

import (
	"net"
	"sync"
	"time"

	"hoplink/internal/hoplink/log"

	hyclient "github.com/apernet/hysteria/core/v2/client"
	hyserver "github.com/apernet/hysteria/core/v2/server"
)

type directOutbound struct {
	dialer net.Dialer
}

func newDirectOutbound() *directOutbound {
	return &directOutbound{dialer: net.Dialer{Timeout: 10 * time.Second}}
}

func (o *directOutbound) TCP(reqAddr string) (net.Conn, error) {
	return o.dialer.Dial("tcp", reqAddr)
}

func (o *directOutbound) UDP(reqAddr string) (hyserver.UDPConn, error) {
	conn, err := net.ListenUDP("udp", nil)
	if err != nil {
		return nil, err
	}
	return &directUDPConn{conn}, nil
}

func (o *directOutbound) CheckUDP(_ string) error { return nil }

type directUDPConn struct{ *net.UDPConn }

func (c *directUDPConn) ReadFrom(b []byte) (int, string, error) {
	n, addr, err := c.UDPConn.ReadFrom(b)
	if addr != nil {
		return n, addr.String(), err
	}
	return n, "", err
}

func (c *directUDPConn) WriteTo(b []byte, addr string) (int, error) {
	uAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return 0, err
	}
	return c.UDPConn.WriteTo(b, uAddr)
}

type chainOutbound struct {
	client hyclient.Client
}

func newChainOutbound(client hyclient.Client) *chainOutbound {
	return &chainOutbound{client: client}
}

func (o *chainOutbound) TCP(reqAddr string) (net.Conn, error) {
	if o.client == nil {
		return nil, net.ErrClosed
	}
	conn, err := o.client.TCP(reqAddr)
	if err != nil {
		log.Warnf("[CASCADE_UPSTREAM_TCP] target=%s err=%v", reqAddr, err)
	}
	return conn, err
}

func (o *chainOutbound) UDP(reqAddr string) (hyserver.UDPConn, error) {
	if o.client == nil {
		return nil, net.ErrClosed
	}
	hyUDP, err := o.client.UDP()
	if err != nil {
		return nil, err
	}
	return &chainUDPConn{conn: hyUDP, reqAddr: reqAddr}, nil
}

func (o *chainOutbound) CheckUDP(_ string) error { return nil }

type chainUDPConn struct {
	conn    hyclient.HyUDPConn
	reqAddr string
}

func (c *chainUDPConn) ReadFrom(b []byte) (int, string, error) {
	data, _, err := c.conn.Receive()
	if err != nil {
		return 0, "", err
	}
	n := copy(b, data)
	return n, c.reqAddr, err
}

func (c *chainUDPConn) WriteTo(b []byte, addr string) (int, error) {
	if err := c.conn.Send(b, addr); err != nil {
		return 0, err
	}
	return len(b), nil
}

func (c *chainUDPConn) Close() error { return c.conn.Close() }

// entryRouter sniffs TLS SNI on the entry hop and picks direct vs upstream outbound per stream.
type entryRouter struct {
	nextAddr       string
	domainPatterns Matcher
	sniPatterns    Matcher
	direct         *directOutbound
	chain          *chainOutbound

	mu       sync.Mutex
	route    streamRoute
	hasRoute bool
}

type streamRoute struct {
	local bool
}

func newEntryRouter(next string, domainPatterns, sniPatterns Matcher, chain *chainOutbound) *entryRouter {
	return &entryRouter{
		nextAddr:       next,
		domainPatterns: domainPatterns,
		sniPatterns:    sniPatterns,
		direct:         newDirectOutbound(),
		chain:          chain,
	}
}

func (r *entryRouter) localForAddr(host, sni string) bool {
	if r.domainPatterns.Match(host) {
		return true
	}
	if sni != "" && r.sniPatterns.Match(sni) {
		return true
	}
	return false
}

func (r *entryRouter) Check(isUDP bool, _ string) bool {
	if isUDP {
		return len(r.domainPatterns.patterns) > 0
	}
	return len(r.sniPatterns.patterns) > 0
}

func (r *entryRouter) TCP(stream hyserver.HyStream, reqAddr *string) ([]byte, error) {
	host := hostFromReqAddr(*reqAddr)
	buf := make([]byte, 4096)
	n, err := stream.Read(buf)
	if err != nil {
		return nil, err
	}
	var sni string
	if n > 0 {
		if parsed, ok := parseTLSSNI(buf[:n]); ok {
			sni = parsed
		}
	}
	local := r.localForAddr(host, sni)
	if r.nextAddr == "" {
		local = true
	}
	r.mu.Lock()
	r.route = streamRoute{local: local}
	r.hasRoute = true
	r.mu.Unlock()
	if n == 0 {
		return nil, nil
	}
	return buf[:n], nil
}

func (r *entryRouter) UDP(_ [][]byte, reqAddr *string) (bool, error) {
	host := hostFromReqAddr(*reqAddr)
	local := r.localForAddr(host, "")
	if r.nextAddr == "" {
		local = true
	}
	r.mu.Lock()
	r.route = streamRoute{local: local}
	r.hasRoute = true
	r.mu.Unlock()
	return true, nil
}

func (r *entryRouter) takeRoute() streamRoute {
	r.mu.Lock()
	defer r.mu.Unlock()
	route := r.route
	r.hasRoute = false
	return route
}

func (r *entryRouter) pickLocal(reqAddr string) bool {
	if len(r.domainPatterns.patterns) == 0 && len(r.sniPatterns.patterns) == 0 {
		return false
	}
	r.mu.Lock()
	has := r.hasRoute
	route := r.route
	if has {
		r.hasRoute = false
	}
	r.mu.Unlock()
	if has {
		return route.local
	}
	host := hostFromReqAddr(reqAddr)
	return r.localForAddr(host, "")
}

func (r *entryRouter) TCPOut(reqAddr string) (net.Conn, error) {
	if r.pickLocal(reqAddr) || r.nextAddr == "" || r.chain == nil {
		return r.direct.TCP(reqAddr)
	}
	return r.chain.TCP(reqAddr)
}

func (r *entryRouter) UDPOut(reqAddr string) (hyserver.UDPConn, error) {
	if r.pickLocal(reqAddr) || r.nextAddr == "" || r.chain == nil {
		return r.direct.UDP(reqAddr)
	}
	return r.chain.UDP(reqAddr)
}

func (r *entryRouter) CheckUDPOut(reqAddr string) error {
	if r.pickLocal(reqAddr) || r.nextAddr == "" || r.chain == nil {
		return r.direct.CheckUDP(reqAddr)
	}
	return r.chain.CheckUDP(reqAddr)
}

// entryOutbound bridges entryRouter to hysteria Outbound.
type entryOutbound struct{ router *entryRouter }

func (o *entryOutbound) TCP(reqAddr string) (net.Conn, error)  { return o.router.TCPOut(reqAddr) }
func (o *entryOutbound) UDP(reqAddr string) (hyserver.UDPConn, error) {
	return o.router.UDPOut(reqAddr)
}
func (o *entryOutbound) CheckUDP(reqAddr string) error { return o.router.CheckUDPOut(reqAddr) }