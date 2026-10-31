package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"hoplink/internal/hoplink/log"

	"golang.org/x/crypto/ssh"
)

type ServerConfig struct {
	Listen             string
	HostKeyFile        string
	AuthorizedKeysFile string
	LocalUDPPort       int
	IdleTimeout        time.Duration
}

func StartServer(ctx context.Context, cfg ServerConfig) error {
	if cfg.Listen == "" {
		return nil
	}
	if cfg.LocalUDPPort <= 0 {
		return fmt.Errorf("relay ssh server requires local UDP port")
	}
	hostKey, err := loadPrivateKey(cfg.HostKeyFile)
	if err != nil {
		return err
	}
	authorized, err := readAuthorizedKeys(cfg.AuthorizedKeysFile)
	if err != nil {
		return err
	}
	serverCfg := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			for _, allowed := range authorized {
				if publicKeysEqual(key, allowed) {
					return nil, nil
				}
			}
			return nil, fmt.Errorf("relay ssh: unauthorized key from %s", conn.RemoteAddr())
		},
	}
	serverCfg.AddHostKey(hostKey)
	ln, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		return err
	}
	log.Infof("[RELAY_SSH_LISTEN] addr=%s local_udp=:%d", cfg.Listen, cfg.LocalUDPPort)
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	for {
		raw, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			log.Warnf("[RELAY_SSH_ACCEPT_ERROR] err=%v", err)
			continue
		}
		go func(c net.Conn) {
			defer c.Close()
			conn, chans, reqs, err := ssh.NewServerConn(c, serverCfg)
			if err != nil {
				log.Warnf("[RELAY_SSH_HANDSHAKE_ERROR] peer=%s err=%v", c.RemoteAddr(), err)
				return
			}
			defer conn.Close()
			go ssh.DiscardRequests(reqs)
			for newChannel := range chans {
				if newChannel.ChannelType() != udpChannel {
					_ = newChannel.Reject(ssh.UnknownChannelType, "unsupported channel")
					continue
				}
				ch, requests, err := newChannel.Accept()
				if err != nil {
					log.Warnf("[RELAY_SSH_CHANNEL_ACCEPT_ERROR] err=%v", err)
					continue
				}
				go ssh.DiscardRequests(requests)
				go serveUDPChannel(ch, cfg.LocalUDPPort, cfg.IdleTimeout)
			}
		}(raw)
	}
}

func serveUDPChannel(stream ssh.Channel, localPort int, idle time.Duration) {
	defer stream.Close()
	target := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: localPort}
	conns := newUDPConns(target, idle)
	defer conns.closeAll()
	for {
		src, payload, err := readUDP(stream)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				log.Debugf("[RELAY_SSH_UDP_READ_ERROR] err=%v", err)
			}
			return
		}
		conn, err := conns.get(src, stream)
		if err != nil {
			log.Warnf("[RELAY_SSH_UDP_DIAL_ERROR] src=%s err=%v", src, err)
			continue
		}
		if _, err := conn.Write(payload); err != nil {
			log.Warnf("[RELAY_SSH_UDP_WRITE_ERROR] src=%s err=%v", src, err)
			conns.remove(src)
		}
	}
}

type udpConnItem struct {
	id   string
	conn *net.UDPConn
}

type udpConns struct {
	target *net.UDPAddr
	idle   time.Duration
	mu     sync.Mutex
	items  map[string]*udpConnItem
	closed bool
}

func newUDPConns(target *net.UDPAddr, idle time.Duration) *udpConns {
	return &udpConns{target: target, idle: idle, items: make(map[string]*udpConnItem)}
}

func (m *udpConns) get(src string, back io.Writer) (*net.UDPConn, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, errors.New("relay ssh udp conns closed")
	}
	if item, ok := m.items[src]; ok {
		return item.conn, nil
	}
	conn, err := net.DialUDP("udp", nil, m.target)
	if err != nil {
		return nil, err
	}
	item := &udpConnItem{id: src, conn: conn}
	m.items[src] = item
	go m.readLoop(item, back)
	return conn, nil
}

func (m *udpConns) readLoop(item *udpConnItem, back io.Writer) {
	buf := make([]byte, maxUDPPacketSize)
	for {
		_ = item.conn.SetReadDeadline(time.Now().Add(m.idle))
		n, err := item.conn.Read(buf)
		if err != nil {
			m.remove(item.id)
			return
		}
		if err := writeUDP(back, item.id, buf[:n]); err != nil {
			m.remove(item.id)
			return
		}
	}
}

func (m *udpConns) remove(src string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	item, ok := m.items[src]
	if !ok {
		return
	}
	delete(m.items, src)
	_ = item.conn.Close()
}

func (m *udpConns) closeAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	for src, item := range m.items {
		delete(m.items, src)
		_ = item.conn.Close()
	}
}

func readAuthorizedKeys(path string) ([]ssh.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var keys []ssh.PublicKey
	for len(data) > 0 {
		pub, _, _, rest, err := ssh.ParseAuthorizedKey(data)
		if err != nil {
			return nil, err
		}
		keys = append(keys, pub)
		data = rest
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("authorized keys file %s is empty", path)
	}
	return keys, nil
}
