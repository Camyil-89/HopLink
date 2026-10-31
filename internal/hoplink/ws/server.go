package ws

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"hoplink/internal/hoplink/log"
	"hoplink/internal/hoplink/wire"

	"github.com/gorilla/websocket"
)

type ServerConfig struct {
	Listen       string
	CertFile     string
	KeyFile      string
	LocalUDPPort int
	IdleTimeout  time.Duration
}

func StartServer(ctx context.Context, cfg ServerConfig) error {
	if cfg.Listen == "" {
		return nil
	}
	if cfg.LocalUDPPort <= 0 {
		return fmt.Errorf("relay ws server requires local UDP port")
	}
	if cfg.CertFile == "" || cfg.KeyFile == "" {
		return fmt.Errorf("relay ws server requires TLS certificate and key")
	}
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc(relayPath, func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Warnf("[RELAY_WS_UPGRADE_ERROR] peer=%s err=%v", r.RemoteAddr, err)
			return
		}
		tuneConn(conn)
		go serveConn(conn, cfg.LocalUDPPort, cfg.IdleTimeout)
	})
	server := &http.Server{
		Addr:      cfg.Listen,
		Handler:   mux,
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12},
	}
	ln, err := tls.Listen("tcp", cfg.Listen, server.TLSConfig)
	if err != nil {
		return err
	}
	log.Infof("[RELAY_WS_LISTEN] addr=%s local_udp=:%d", cfg.Listen, cfg.LocalUDPPort)
	go func() {
		<-ctx.Done()
		_ = server.Close()
	}()
	if err := server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) && ctx.Err() == nil {
		return err
	}
	return nil
}

func serveConn(wsConn *websocket.Conn, localPort int, idle time.Duration) {
	defer wsConn.Close()
	target := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: localPort}
	bridge := newUDPBridge(target, idle, wsConn)
	defer bridge.closeAll()
	for {
		_ = wsConn.SetReadDeadline(time.Now().Add(90 * time.Second))
		_, data, err := wsConn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Debugf("[RELAY_WS_READ_ERROR] err=%v", err)
			}
			return
		}
		_ = wire.WalkFrames(data, func(src string, payload []byte) error {
			conn, err := bridge.get(src)
			if err != nil {
				log.Warnf("[RELAY_WS_UDP_DIAL_ERROR] src=%s err=%v", src, err)
				return nil
			}
			if _, err := conn.Write(payload); err != nil {
				log.Warnf("[RELAY_WS_UDP_WRITE_ERROR] src=%s err=%v", src, err)
				bridge.remove(src)
			}
			return nil
		})
	}
}

type udpBridge struct {
	target   *net.UDPAddr
	idle     time.Duration
	outbound chan []byte
	mu       sync.Mutex
	items    map[string]*net.UDPConn
	closed   bool
}

func newUDPBridge(target *net.UDPAddr, idle time.Duration, back *websocket.Conn) *udpBridge {
	b := &udpBridge{
		target:   target,
		idle:     idle,
		outbound: make(chan []byte, wsWriteQueue),
		items:    make(map[string]*net.UDPConn),
	}
	go b.writePump(back)
	return b
}

func (m *udpBridge) writePump(back *websocket.Conn) {
	scratch := make([]byte, 0, coalesceLimit)
	var pending []byte
	for {
		first := pending
		pending = nil
		if first == nil {
			frame, ok := <-m.outbound
			if !ok {
				return
			}
			first = frame
		}
		var batch []byte
		batch, pending = packFrames(m.outbound, first, scratch, nil)
		if len(batch) == 0 {
			return
		}
		if err := back.WriteMessage(websocket.BinaryMessage, batch); err != nil {
			wire.ReleaseFrame(pending)
			return
		}
	}
}

func (m *udpBridge) get(src string) (*net.UDPConn, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, errors.New("relay ws bridge closed")
	}
	if conn, ok := m.items[src]; ok {
		return conn, nil
	}
	conn, err := net.DialUDP("udp", nil, m.target)
	if err != nil {
		return nil, err
	}
	m.items[src] = conn
	go m.readLoop(src, conn)
	return conn, nil
}

func (m *udpBridge) enqueue(frame []byte) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return false
	}
	select {
	case m.outbound <- frame:
		return true
	default:
		return false
	}
}

func (m *udpBridge) readLoop(src string, conn *net.UDPConn) {
	buf := make([]byte, wire.MaxPayload)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(m.idle))
		n, err := conn.Read(buf)
		if err != nil {
			m.remove(src)
			return
		}
		frame, err := wire.EncodeBytes(src, buf[:n])
		if err != nil {
			m.remove(src)
			return
		}
		if !m.enqueue(frame) {
			wire.ReleaseFrame(frame)
		}
	}
}

func (m *udpBridge) remove(src string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	conn, ok := m.items[src]
	if !ok {
		return
	}
	delete(m.items, src)
	_ = conn.Close()
}

func (m *udpBridge) closeAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	for src, conn := range m.items {
		delete(m.items, src)
		_ = conn.Close()
	}
	close(m.outbound)
}
