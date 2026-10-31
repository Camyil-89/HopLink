package ws

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"os"
	"sync"
	"time"

	"hoplink/internal/hoplink/log"
	"hoplink/internal/hoplink/wire"

	"github.com/gorilla/websocket"
)

const relayPath = "/hoplink-udp-relay"

type ClientConfig struct {
	NextAddr      string
	TrustCertFile string
}

type Upstream struct {
	cfg ClientConfig

	mu   sync.Mutex
	conn *websocket.Conn

	peersMu sync.Mutex
	peers   map[netip.AddrPort]chan []byte

	outbound chan []byte

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewUpstream(ctx context.Context, cfg ClientConfig) (*Upstream, error) {
	if cfg.NextAddr == "" || cfg.TrustCertFile == "" {
		return nil, fmt.Errorf("relay ws client requires next address and trust certificate")
	}
	child, cancel := context.WithCancel(ctx)
	u := &Upstream{
		cfg:      cfg,
		peers:    make(map[netip.AddrPort]chan []byte),
		outbound: make(chan []byte, wsWriteQueue),
		ctx:      child,
		cancel:   cancel,
	}
	if err := u.connect(); err != nil {
		cancel()
		return nil, err
	}
	u.wg.Add(2)
	go u.writePump()
	go u.readPump()
	return u, nil
}

func (u *Upstream) readPump() {
	defer u.wg.Done()
	backoff := time.Second
	for {
		select {
		case <-u.ctx.Done():
			return
		default:
		}
		u.mu.Lock()
		conn := u.conn
		u.mu.Unlock()
		if conn == nil {
			if err := u.connect(); err != nil {
				log.Warnf("[RELAY_WS_CLIENT_RECONNECT] err=%v", err)
				select {
				case <-time.After(backoff):
				case <-u.ctx.Done():
					return
				}
				if backoff < 30*time.Second {
					backoff *= 2
				}
				continue
			}
			backoff = time.Second
			u.mu.Lock()
			conn = u.conn
			u.mu.Unlock()
		}
		if conn == nil {
			continue
		}
		_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		_, data, err := conn.ReadMessage()
		if err != nil {
			u.dropConnection()
			if u.ctx.Err() != nil {
				return
			}
			continue
		}
		_ = wire.WalkFrames(data, func(src string, payload []byte) error {
			peer, err := netip.ParseAddrPort(src)
			if err != nil {
				return nil
			}
			u.peersMu.Lock()
			deliver := u.peers[peer]
			u.peersMu.Unlock()
			if deliver == nil {
				return nil
			}
			packet := wire.PayloadCopy(payload)
			select {
			case deliver <- packet:
			default:
			}
			return nil
		})
	}
}

func (u *Upstream) writePump() {
	defer u.wg.Done()
	scratch := make([]byte, 0, coalesceLimit)
	var pending []byte
	for {
		first := pending
		pending = nil
		if first == nil {
			select {
			case <-u.ctx.Done():
				return
			case first = <-u.outbound:
			}
		}
		var batch []byte
		batch, pending = packFrames(u.outbound, first, scratch, u.ctx.Done())
		if len(batch) == 0 {
			return
		}
		u.mu.Lock()
		conn := u.conn
		u.mu.Unlock()
		if conn == nil {
			continue
		}
		if err := conn.WriteMessage(websocket.BinaryMessage, batch); err != nil {
			u.dropConnection()
		}
	}
}

func (u *Upstream) connect() error {
	tlsCfg, err := tlsConfigFromTrustFile(u.cfg.TrustCertFile)
	if err != nil {
		return err
	}
	host, port, err := net.SplitHostPort(u.cfg.NextAddr)
	if err != nil {
		return fmt.Errorf("relay ws next address: %w", err)
	}
	uURL := url.URL{Scheme: "wss", Host: net.JoinHostPort(host, port), Path: relayPath}
	conn, _, err := dialer(tlsCfg).DialContext(u.ctx, uURL.String(), nil)
	if err != nil {
		return err
	}
	tuneConn(conn)
	u.mu.Lock()
	if u.conn != nil {
		_ = u.conn.Close()
	}
	u.conn = conn
	u.mu.Unlock()
	log.Infof("[RELAY_WS_CLIENT_CONNECTED] next=%s", u.cfg.NextAddr)
	return nil
}

func tlsConfigFromTrustFile(path string) (*tls.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("trust certificate PEM is empty")
	}
	trustedDER := append([]byte(nil), block.Bytes...)
	return &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true,
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) == 0 || !bytes.Equal(rawCerts[0], trustedDER) {
				return fmt.Errorf("relay ws certificate pin mismatch")
			}
			return nil
		},
	}, nil
}

func (u *Upstream) dropConnection() {
	u.mu.Lock()
	if u.conn != nil {
		_ = u.conn.Close()
		u.conn = nil
	}
	u.mu.Unlock()
}

func (u *Upstream) Register(peer netip.AddrPort) chan []byte {
	ch := make(chan []byte, peerQueueDepth)
	u.peersMu.Lock()
	u.peers[peer] = ch
	u.peersMu.Unlock()
	return ch
}

func (u *Upstream) Unregister(peer netip.AddrPort) {
	u.peersMu.Lock()
	delete(u.peers, peer)
	u.peersMu.Unlock()
}

func (u *Upstream) Write(peer netip.AddrPort, packet []byte) error {
	frame, err := wire.EncodeBytes(peer.String(), packet)
	if err != nil {
		return err
	}
	select {
	case u.outbound <- frame:
		return nil
	default:
		wire.ReleaseFrame(frame)
		return errors.New("relay ws outbound queue full")
	}
}

func (u *Upstream) Close() {
	u.cancel()
	u.dropConnection()
	u.wg.Wait()
}

const peerQueueDepth = 256
