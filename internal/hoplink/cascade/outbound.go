package cascade

import (
	"net"
	"sync"
	"sync/atomic"
	"time"

	"hoplink/internal/hoplink/log"

	hyclient "github.com/apernet/hysteria/core/v2/client"
	hyserver "github.com/apernet/hysteria/core/v2/server"
)

var tcpSniffPool = sync.Pool{
	New: func() any {
		return make([]byte, 4096)
	},
}

type directOutbound struct {
	dialer net.Dialer
}

func newDirectOutbound() *directOutbound {
	return &directOutbound{dialer: net.Dialer{Timeout: 10 * time.Second, Control: controlOutbound}}
}

func (o *directOutbound) TCP(reqAddr string) (net.Conn, error) {
	return o.dialer.Dial("tcp", reqAddr)
}

func (o *directOutbound) UDP(reqAddr string) (hyserver.UDPConn, error) {
	conn, err := net.ListenUDP("udp", nil)
	if err != nil {
		return nil, err
	}
	return &directUDPConn{UDPConn: conn}, nil
}

func (o *directOutbound) CheckUDP(_ string) error { return nil }

type resolvedUDP struct {
	raw  string
	addr *net.UDPAddr
}

type directUDPConn struct {
	*net.UDPConn
	last atomic.Pointer[resolvedUDP]
}

func (c *directUDPConn) ReadFrom(b []byte) (int, string, error) {
	n, addr, err := c.UDPConn.ReadFrom(b)
	if addr != nil {
		return n, addr.String(), err
	}
	return n, "", err
}

func (c *directUDPConn) WriteTo(b []byte, addr string) (int, error) {
	if cur := c.last.Load(); cur != nil && cur.raw == addr {
		return c.UDPConn.WriteTo(b, cur.addr)
	}
	uAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return 0, err
	}
	c.last.Store(&resolvedUDP{raw: addr, addr: uAddr})
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
	reqAddr string
	conn    hyclient.HyUDPConn
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
	forceLocal bool
	direct     *directOutbound
	chain      *chainOutbound

	domainPatterns Matcher
	sniPatterns    Matcher
	pending        pendingRoutes
}

func newEntryRouter(next string, domainPatterns, sniPatterns Matcher, chain *chainOutbound) *entryRouter {
	return &entryRouter{
		domainPatterns: domainPatterns,
		sniPatterns:    sniPatterns,
		direct:         newDirectOutbound(),
		chain:          chain,
		pending:        newPendingRoutes(),
		forceLocal:     next == "",
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

func (r *entryRouter) storeRoute(local bool) {
	r.pending.store(parseCurrentGoroutineID(), local)
}

func (r *entryRouter) Check(isUDP bool, _ string) bool {
	if isUDP {
		return r.domainPatterns.HasRules()
	}
	return r.sniPatterns.HasRules()
}

func (r *entryRouter) TCP(stream hyserver.HyStream, reqAddr *string) ([]byte, error) {
	host := hostFromReqAddr(*reqAddr)
	buf := tcpSniffPool.Get().([]byte)
	n, err := stream.Read(buf)
	if err != nil {
		tcpSniffPool.Put(buf)
		return nil, err
	}
	var sni string
	if n > 0 {
		if parsed, ok := parseTLSSNI(buf[:n]); ok {
			sni = parsed
		}
	}
	local := r.localForAddr(host, sni)
	if r.forceLocal {
		local = true
	}
	r.storeRoute(local)
	if n == 0 {
		tcpSniffPool.Put(buf)
		return nil, nil
	}
	out := make([]byte, n)
	copy(out, buf[:n])
	tcpSniffPool.Put(buf)
	return out, nil
}

func (r *entryRouter) UDP(_ [][]byte, reqAddr *string) (bool, error) {
	host := hostFromReqAddr(*reqAddr)
	local := r.localForAddr(host, "")
	if r.forceLocal {
		local = true
	}
	r.storeRoute(local)
	return true, nil
}

func (r *entryRouter) pickLocal(reqAddr string) bool {
	if !r.domainPatterns.HasRules() && !r.sniPatterns.HasRules() {
		return false
	}
	if local, ok := r.pending.take(parseCurrentGoroutineID()); ok {
		return local
	}
	host := hostFromReqAddr(reqAddr)
	return r.localForAddr(host, "")
}

func (r *entryRouter) useDirect(reqAddr string) bool {
	return r.forceLocal || r.chain == nil || r.pickLocal(reqAddr)
}

func (r *entryRouter) TCPOut(reqAddr string) (net.Conn, error) {
	if r.useDirect(reqAddr) {
		return r.direct.TCP(reqAddr)
	}
	return r.chain.TCP(reqAddr)
}

func (r *entryRouter) UDPOut(reqAddr string) (hyserver.UDPConn, error) {
	if r.useDirect(reqAddr) {
		return r.direct.UDP(reqAddr)
	}
	return r.chain.UDP(reqAddr)
}

func (r *entryRouter) CheckUDPOut(reqAddr string) error {
	if r.useDirect(reqAddr) {
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