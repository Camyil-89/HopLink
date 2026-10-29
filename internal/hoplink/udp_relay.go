package hoplink

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultRelayIdleTimeout = 30 * time.Second
	relayQueueSize          = 8
	maxUDPPacketSize        = 65535
	relayMaxQueuedBytes     = 16 << 20
)

// udpRelay forwards opaque QUIC packets. Each source address has its own
// connected upstream socket and bounded queue; no QUIC/TLS is terminated here.
type udpRelay struct {
	conn        *net.UDPConn
	target      *net.UDPAddr
	idleTimeout time.Duration
	obfsMode    string
	mu          sync.Mutex
	sessions    map[netip.AddrPort]*udpRelaySession
	closed      bool
	wg          sync.WaitGroup
	queuedBytes atomic.Int64
}

type udpRelaySession struct {
	relay    *udpRelay
	peer     netip.AddrPort
	conn     *net.UDPConn
	queue    chan []byte
	done     chan struct{}
	once     sync.Once
	mu       sync.Mutex
	closed   bool
	lastSeen atomic.Int64
}

func newUDPRelay(listen, forward string, idleTimeout time.Duration) (*udpRelay, error) {
	return newUDPRelayWithObfs(listen, forward, idleTimeout, "")
}

func newUDPRelayWithObfs(listen, forward string, idleTimeout time.Duration, obfsMode string) (*udpRelay, error) {
	if idleTimeout < time.Second {
		return nil, fmt.Errorf("relay requires idle timeout >= 1s")
	}
	target, err := net.ResolveUDPAddr("udp", forward)
	if err != nil {
		return nil, fmt.Errorf("resolve relay target: %w", err)
	}
	if target.Port == 0 || target.IP.IsUnspecified() {
		return nil, fmt.Errorf("relay target must have a concrete address and nonzero port")
	}
	if obfsMode != "" && obfsMode != "salamander" && obfsMode != "gecko" {
		return nil, fmt.Errorf("unsupported relay obfuscation %q", obfsMode)
	}
	addr, err := net.ResolveUDPAddr("udp", listen)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	return &udpRelay{conn: conn, target: target, idleTimeout: idleTimeout, obfsMode: obfsMode, sessions: make(map[netip.AddrPort]*udpRelaySession)}, nil
}

func (r *udpRelay) Serve(ctx context.Context) error {
	defer r.Close()
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			_ = r.conn.Close()
		case <-stop:
		}
	}()
	buf := make([]byte, maxUDPPacketSize)
	for {
		n, peer, err := r.conn.ReadFromUDPAddrPort(buf)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		logTracef("[UDP_RELAY_RX] peer=%s packet_bytes=%d obfs=%s", peer, n, r.obfsMode)
		s := r.session(peer, buf[:n])
		if s == nil {
			continue
		}
		s.offer(buf[:n])
	}
}

func (s *udpRelaySession) offer(packet []byte) {
	// Serialize admission against close so no packet/budget can be stranded
	// after the sender exits. The shared reader never waits for socket I/O.
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || len(s.queue) == cap(s.queue) {
		return
	}
	n := int64(len(packet))
	for {
		used := s.relay.queuedBytes.Load()
		if used+n > relayMaxQueuedBytes {
			return
		}
		if s.relay.queuedBytes.CompareAndSwap(used, used+n) {
			break
		}
	}
	s.queue <- append([]byte(nil), packet...)
}

// Only a QUIC Initial can allocate state. Existing sessions also forward short
// headers, retries and path validation unchanged. This is admission filtering,
// not authentication: passwords are checked exclusively at the exit server.
func isQUICInitial(p []byte) bool {
	if len(p) < 1200 || p[0]&0xc0 != 0xc0 {
		return false
	}
	version := binary.BigEndian.Uint32(p[1:5])
	typ := (p[0] >> 4) & 3
	return (version == 1 && typ == 0) || (version == 0x6b3343cf && typ == 1)
}

func (r *udpRelay) session(peer netip.AddrPort, packet []byte) *udpRelaySession {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	if s := r.sessions[peer]; s != nil {
		return s
	}
	if !isRelayInitial(packet, r.obfsMode) {
		logTracef("[UDP_RELAY_DROP] peer=%s packet_bytes=%d reason=not_initial_or_too_short obfs=%s", peer, len(packet), r.obfsMode)
		return nil
	}
	conn, err := net.DialUDP("udp", nil, r.target)
	if err != nil {
		logWarnf("[UDP_RELAY_DIAL_ERROR] peer=%s target=%s err=%v", peer, r.target, err)
		return nil // Only this source fails; keep every established session.
	}
	logInfof("[UDP_RELAY_SESSION_START] peer=%s target=%s obfs=%s", peer, r.target, r.obfsMode)
	s := &udpRelaySession{relay: r, peer: peer, conn: conn, queue: make(chan []byte, relayQueueSize), done: make(chan struct{})}
	s.lastSeen.Store(time.Now().UnixNano())
	r.sessions[peer] = s
	r.wg.Add(2)
	go s.send()
	go s.receive()
	return s
}

func isRelayInitial(packet []byte, obfsMode string) bool {
	switch obfsMode {
	case "salamander":
		return len(packet) >= 1200+salamanderSaltSize
	case "gecko":
		return len(packet) >= geckoMinPacketSize
	default:
		return isQUICInitial(packet)
	}
}

func (s *udpRelaySession) close() {
	s.once.Do(func() {
		s.mu.Lock()
		s.closed = true
		close(s.done)
	cleanup:
		for {
			select {
			case packet := <-s.queue:
				s.relay.queuedBytes.Add(-int64(len(packet)))
			default:
				break cleanup
			}
		}
		s.mu.Unlock()
		_ = s.conn.Close()
		s.relay.mu.Lock()
		if s.relay.sessions[s.peer] == s {
			delete(s.relay.sessions, s.peer)
		}
		s.relay.mu.Unlock()
	})
}

func (s *udpRelaySession) send() {
	defer s.relay.wg.Done()
	defer s.close()
	for {
		select {
		case <-s.done:
			return
		case packet := <-s.queue:
			_ = s.conn.SetWriteDeadline(time.Now().Add(time.Second))
			_, err := s.conn.Write(packet)
			s.relay.queuedBytes.Add(-int64(len(packet)))
			if err != nil {
				// UDP ICMP errors and a blocked socket affect only this source.
				logWarnf("[UDP_RELAY_SEND_ERROR] peer=%s target=%s err=%v", s.peer, s.relay.target, err)
				return
			}
			s.lastSeen.Store(time.Now().UnixNano())
		}
	}
}

func (s *udpRelaySession) receive() {
	defer s.relay.wg.Done()
	defer s.close()
	buf := make([]byte, maxUDPPacketSize)
	for {
		_ = s.conn.SetReadDeadline(time.Now().Add(s.relay.idleTimeout))
		n, err := s.conn.Read(buf)
		if err != nil {
			var timeout net.Error
			if errors.As(err, &timeout) && timeout.Timeout() && time.Since(time.Unix(0, s.lastSeen.Load())) < s.relay.idleTimeout {
				continue
			}
			logDebugf("[UDP_RELAY_UPSTREAM_CLOSED] peer=%s target=%s err=%v", s.peer, s.relay.target, err)
			return
		}
		logTracef("[UDP_RELAY_TX] peer=%s target=%s packet_bytes=%d", s.peer, s.relay.target, n)
		s.lastSeen.Store(time.Now().UnixNano())
		if _, err := s.relay.conn.WriteToUDPAddrPort(buf[:n], s.peer); err != nil {
			return
		}
	}
}

func (r *udpRelay) Close() {
	r.mu.Lock()
	r.closed = true
	sessions := make([]*udpRelaySession, 0, len(r.sessions))
	for _, s := range r.sessions {
		sessions = append(sessions, s)
	}
	r.mu.Unlock()
	_ = r.conn.Close()
	for _, s := range sessions {
		s.close()
	}
	r.wg.Wait()
}
