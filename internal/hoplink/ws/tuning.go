package ws

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"hoplink/internal/hoplink/wire"

	"github.com/gorilla/websocket"
)

const (
	wsBufferSize  = 256 << 10
	wsSocketSize  = 4 << 20
	wsWriteQueue  = 1024
	coalesceLimit = 32 << 10
	coalesceWait  = time.Millisecond
)

var upgrader = websocket.Upgrader{
	CheckOrigin:       func(*http.Request) bool { return true },
	EnableCompression: false,
	ReadBufferSize:    wsBufferSize,
	WriteBufferSize:   wsBufferSize,
}

func dialer(tlsCfg *tls.Config) *websocket.Dialer {
	return &websocket.Dialer{
		TLSClientConfig:   tlsCfg,
		HandshakeTimeout:  15 * time.Second,
		EnableCompression: false,
		ReadBufferSize:    wsBufferSize,
		WriteBufferSize:   wsBufferSize,
	}
}

func packFrames(ch <-chan []byte, first []byte, scratch []byte, done <-chan struct{}) (batch []byte, pending []byte) {
	scratch = append(scratch[:0], first...)
	wire.ReleaseFrame(first)
	deadline := time.Now().Add(coalesceWait)
	for len(scratch) < coalesceLimit {
		wait := time.Until(deadline)
		if wait <= 0 {
			break
		}
		timer := time.NewTimer(wait)
		select {
		case <-done:
			timer.Stop()
			return scratch, nil
		case next, ok := <-ch:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			if !ok {
				return scratch, nil
			}
			if len(scratch)+len(next) > coalesceLimit {
				return scratch, next
			}
			scratch = append(scratch, next...)
			wire.ReleaseFrame(next)
		case <-timer.C:
			return scratch, nil
		}
	}
	return scratch, nil
}

func tuneConn(conn *websocket.Conn) {
	raw := conn.UnderlyingConn()
	for raw != nil {
		switch c := raw.(type) {
		case *tls.Conn:
			raw = c.NetConn()
		case *net.TCPConn:
			_ = c.SetNoDelay(true)
			_ = c.SetReadBuffer(wsSocketSize)
			_ = c.SetWriteBuffer(wsSocketSize)
			return
		default:
			return
		}
	}
}
