package relay

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"hoplink/internal/hoplink/log"
	"hoplink/internal/hoplink/obfs"
)

const (
	DefaultIdleTimeout = 30 * time.Second
	queueSize          = 8
	maxUDPPacketSize   = 65535
	maxQueuedBytes     = 16 << 20
)

// Upstream carries relay traffic over SSH instead of raw UDP to the next hop.
type Upstream interface {
	Register(peer netip.AddrPort) chan []byte
	Unregister(peer netip.AddrPort)
	Write(peer netip.AddrPort, packet []byte) error
	Close()
}

// Relay forwards opaque QUIC packets. Each source address has its own
// connected upstream socket and bounded queue; no QUIC/TLS is terminated here.
type Relay struct {
	conn        *net.UDPConn
	target      *net.UDPAddr
	idleTimeout time.Duration
	obfsMode    string
	upstream    Upstream
	mu          sync.Mutex
	sessions    map[netip.AddrPort]*session
	closed      bool
	wg          sync.WaitGroup
	queuedBytes atomic.Int64
}

type session struct {
	relay      *Relay
	peer       netip.AddrPort
	conn       *net.UDPConn
	sshDeliver chan []byte
	queue      chan []byte
	done       chan struct{}
	once       sync.Once
	mu         sync.Mutex
	closed     bool
	lastSeen   atomic.Int64
}

func New(listen, forward string, idleTimeout time.Duration) (*Relay, error) {
	return NewWithTransport(listen, forward, idleTimeout, "", false)
}

func NewWithObfs(listen, forward string, idleTimeout time.Duration, obfsMode string) (*Relay, error) {
	return NewWithTransport(listen, forward, idleTimeout, obfsMode, false)
}

func NewWithTransport(listen, forward string, idleTimeout time.Duration, obfsMode string, sshTransport bool) (*Relay, error) {
	if idleTimeout < time.Second {
		return nil, fmt.Errorf("relay requires idle timeout >= 1s")
	}
	var target *net.UDPAddr
	if !sshTransport {
		var err error
		target, err = net.ResolveUDPAddr("udp", forward)
		if err != nil {
			return nil, fmt.Errorf("resolve relay target: %w", err)
		}
		if target.Port == 0 || target.IP.IsUnspecified() {
			return nil, fmt.Errorf("relay target must have a concrete address and nonzero port")
		}
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
	return &Relay{conn: conn, target: target, idleTimeout: idleTimeout, obfsMode: obfsMode, sessions: make(map[netip.AddrPort]*session)}, nil
}

func (r *Relay) SetUpstream(u Upstream) { r.upstream = u }

func (r *Relay) LocalAddr() net.Addr { return r.conn.LocalAddr() }

func (r *Relay) QueuedBytes() int64 { return r.queuedBytes.Load() }

func (r *Relay) SessionCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.sessions)
}

// ParseForwardTarget normalizes the next UDP hop address for relay mode.
func ParseForwardTarget(s string) string {
	if s == "" {
		return ""
	}
	if _, _, err := net.SplitHostPort(s); err == nil {
		return s
	}
	return net.JoinHostPort(s, strconv.Itoa(22033))
}

func (r *Relay) Serve(ctx context.Context) error {
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
		log.Tracef("[UDP_RELAY_RX] peer=%s packet_bytes=%d obfs=%s", peer, n, r.obfsMode)
		s := r.session(peer, buf[:n])
		if s == nil {
			continue
		}
		s.offer(buf[:n])
	}
}

func (s *session) offer(packet []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || len(s.queue) == cap(s.queue) {
		return
	}
	n := int64(len(packet))
	for {
		used := s.relay.queuedBytes.Load()
		if used+n > maxQueuedBytes {
			return
		}
		if s.relay.queuedBytes.CompareAndSwap(used, used+n) {
			break
		}
	}
	s.queue <- append([]byte(nil), packet...)
}

func IsQUICInitial(p []byte) bool {
	if len(p) < 1200 || p[0]&0xc0 != 0xc0 {
		return false
	}
	version := binary.BigEndian.Uint32(p[1:5])
	typ := (p[0] >> 4) & 3
	return (version == 1 && typ == 0) || (version == 0x6b3343cf && typ == 1)
}

func IsRelayInitial(packet []byte, obfsMode string) bool {
	switch obfsMode {
	case "salamander":
		return len(packet) >= 1200+obfs.SalamanderSaltSize
	case "gecko":
		return len(packet) >= obfs.GeckoMinPacketSize
	default:
		return IsQUICInitial(packet)
	}
}

func (r *Relay) session(peer netip.AddrPort, packet []byte) *session {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	if s := r.sessions[peer]; s != nil {
		return s
	}
	if !IsRelayInitial(packet, r.obfsMode) {
		log.Tracef("[UDP_RELAY_DROP] peer=%s packet_bytes=%d reason=not_initial_or_too_short obfs=%s", peer, len(packet), r.obfsMode)
		return nil
	}
	var conn *net.UDPConn
	var sshDeliver chan []byte
	if r.upstream != nil {
		sshDeliver = r.upstream.Register(peer)
		log.Infof("[UDP_RELAY_SESSION_START] peer=%s transport=ssh obfs=%s", peer, r.obfsMode)
	} else {
		var err error
		conn, err = net.DialUDP("udp", nil, r.target)
		if err != nil {
			log.Warnf("[UDP_RELAY_DIAL_ERROR] peer=%s target=%s err=%v", peer, r.target, err)
			return nil
		}
		log.Infof("[UDP_RELAY_SESSION_START] peer=%s target=%s obfs=%s", peer, r.target, r.obfsMode)
	}
	s := &session{relay: r, peer: peer, conn: conn, sshDeliver: sshDeliver, queue: make(chan []byte, queueSize), done: make(chan struct{})}
	s.lastSeen.Store(time.Now().UnixNano())
	r.sessions[peer] = s
	r.wg.Add(2)
	go s.send()
	go s.receive()
	return s
}

func (s *session) close() {
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
		if s.conn != nil {
			_ = s.conn.Close()
		}
		if s.sshDeliver != nil {
			s.relay.upstream.Unregister(s.peer)
		}
		s.relay.mu.Lock()
		if s.relay.sessions[s.peer] == s {
			delete(s.relay.sessions, s.peer)
		}
		s.relay.mu.Unlock()
	})
}

func (s *session) send() {
	defer s.relay.wg.Done()
	defer s.close()
	for {
		select {
		case <-s.done:
			return
		case packet := <-s.queue:
			var err error
			if s.relay.upstream != nil {
				err = s.relay.upstream.Write(s.peer, packet)
			} else {
				_ = s.conn.SetWriteDeadline(time.Now().Add(time.Second))
				_, err = s.conn.Write(packet)
			}
			s.relay.queuedBytes.Add(-int64(len(packet)))
			if err != nil {
				log.Warnf("[UDP_RELAY_SEND_ERROR] peer=%s target=%s err=%v", s.peer, s.relay.target, err)
				return
			}
			s.lastSeen.Store(time.Now().UnixNano())
		}
	}
}

func (s *session) receive() {
	defer s.relay.wg.Done()
	defer s.close()
	if s.sshDeliver != nil {
		idle := s.relay.idleTimeout
		idleTimer := time.NewTimer(idle)
		defer idleTimer.Stop()
		for {
			select {
			case <-s.done:
				return
			case packet := <-s.sshDeliver:
				log.Tracef("[UDP_RELAY_TX] peer=%s transport=tunnel packet_bytes=%d", s.peer, len(packet))
				s.lastSeen.Store(time.Now().UnixNano())
				if !idleTimer.Stop() {
					select {
					case <-idleTimer.C:
					default:
					}
				}
				idleTimer.Reset(idle)
				if _, err := s.relay.conn.WriteToUDPAddrPort(packet, s.peer); err != nil {
					return
				}
			case <-idleTimer.C:
				if time.Since(time.Unix(0, s.lastSeen.Load())) >= idle {
					log.Debugf("[UDP_RELAY_UPSTREAM_CLOSED] peer=%s transport=tunnel reason=idle", s.peer)
					return
				}
				idleTimer.Reset(idle)
			}
		}
	}
	buf := make([]byte, maxUDPPacketSize)
	for {
		_ = s.conn.SetReadDeadline(time.Now().Add(s.relay.idleTimeout))
		n, err := s.conn.Read(buf)
		if err != nil {
			var timeout net.Error
			if errors.As(err, &timeout) && timeout.Timeout() && time.Since(time.Unix(0, s.lastSeen.Load())) < s.relay.idleTimeout {
				continue
			}
			log.Debugf("[UDP_RELAY_UPSTREAM_CLOSED] peer=%s target=%s err=%v", s.peer, s.relay.target, err)
			return
		}
		log.Tracef("[UDP_RELAY_TX] peer=%s target=%s packet_bytes=%d", s.peer, s.relay.target, n)
		s.lastSeen.Store(time.Now().UnixNano())
		if _, err := s.relay.conn.WriteToUDPAddrPort(buf[:n], s.peer); err != nil {
			return
		}
	}
}

func (r *Relay) Close() {
	r.mu.Lock()
	r.closed = true
	sessions := make([]*session, 0, len(r.sessions))
	for _, s := range r.sessions {
		sessions = append(sessions, s)
	}
	upstream := r.upstream
	r.mu.Unlock()
	_ = r.conn.Close()
	for _, s := range sessions {
		s.close()
	}
	r.wg.Wait()
	if upstream != nil {
		upstream.Close()
	}
}
