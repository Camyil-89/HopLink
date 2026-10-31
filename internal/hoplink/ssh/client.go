package ssh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"sync"
	"time"

	"hoplink/internal/hoplink/log"

	"golang.org/x/crypto/ssh"
)

type ClientConfig struct {
	NextAddr       string
	User           string
	ClientKeyFile  string
	HostPubKeyFile string
}

type Upstream struct {
	cfg ClientConfig

	mu      sync.Mutex
	client  *ssh.Client
	channel ssh.Channel
	writeMu sync.Mutex

	peersMu sync.Mutex
	peers   map[netip.AddrPort]chan []byte

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewUpstream(ctx context.Context, cfg ClientConfig) (*Upstream, error) {
	if cfg.NextAddr == "" || cfg.ClientKeyFile == "" || cfg.HostPubKeyFile == "" {
		return nil, fmt.Errorf("relay ssh client requires next address, client key, and host public key")
	}
	child, cancel := context.WithCancel(ctx)
	u := &Upstream{cfg: cfg, peers: make(map[netip.AddrPort]chan []byte), ctx: child, cancel: cancel}
	if err := u.connect(); err != nil {
		cancel()
		return nil, err
	}
	u.wg.Add(1)
	go u.maintain()
	return u, nil
}

func (u *Upstream) maintain() {
	defer u.wg.Done()
	backoff := time.Second
	for {
		select {
		case <-u.ctx.Done():
			return
		default:
		}
		u.mu.Lock()
		ch := u.channel
		u.mu.Unlock()
		if ch == nil {
			if err := u.connect(); err != nil {
				log.Warnf("[RELAY_SSH_CLIENT_RECONNECT] err=%v", err)
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
			ch = u.channel
			u.mu.Unlock()
		}
		if ch == nil {
			continue
		}
		src, payload, err := readUDP(ch)
		if err != nil {
			u.dropConnection()
			if u.ctx.Err() != nil {
				return
			}
			continue
		}
		peer, err := netip.ParseAddrPort(src)
		if err != nil {
			continue
		}
		u.peersMu.Lock()
		deliver := u.peers[peer]
		u.peersMu.Unlock()
		if deliver == nil {
			continue
		}
		packet := append([]byte(nil), payload...)
		select {
		case deliver <- packet:
		default:
		}
	}
}

func (u *Upstream) connect() error {
	signer, err := loadPrivateKey(u.cfg.ClientKeyFile)
	if err != nil {
		return err
	}
	hostKey, err := readSingleHostPublicKey(u.cfg.HostPubKeyFile)
	if err != nil {
		return err
	}
	user := u.cfg.User
	if user == "" {
		user = "hoplink"
	}
	clientCfg := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			if publicKeysEqual(key, hostKey) {
				return nil
			}
			return fmt.Errorf("relay ssh host key mismatch for %s", hostname)
		},
		Timeout: 15 * time.Second,
	}
	client, err := ssh.Dial("tcp", u.cfg.NextAddr, clientCfg)
	if err != nil {
		return err
	}
	ch, reqs, err := client.OpenChannel(udpChannel, nil)
	if err != nil {
		_ = client.Close()
		return err
	}
	go ssh.DiscardRequests(reqs)
	u.mu.Lock()
	if u.client != nil {
		_ = u.client.Close()
	}
	u.client = client
	u.channel = ch
	u.mu.Unlock()
	log.Infof("[RELAY_SSH_CLIENT_CONNECTED] next=%s", u.cfg.NextAddr)
	return nil
}

func (u *Upstream) dropConnection() {
	u.mu.Lock()
	if u.channel != nil {
		_ = u.channel.Close()
		u.channel = nil
	}
	if u.client != nil {
		_ = u.client.Close()
		u.client = nil
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
	u.writeMu.Lock()
	defer u.writeMu.Unlock()
	u.mu.Lock()
	ch := u.channel
	u.mu.Unlock()
	if ch == nil {
		return errors.New("relay ssh channel unavailable")
	}
	return writeUDP(ch, peer.String(), packet)
}

func (u *Upstream) Close() {
	u.cancel()
	u.dropConnection()
	u.wg.Wait()
}

func readSingleHostPublicKey(path string) (ssh.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	pub, _, _, rest, err := ssh.ParseAuthorizedKey(data)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(rest)) > 0 {
		return nil, fmt.Errorf("host public key file must contain exactly one key")
	}
	return pub, nil
}

func loadPrivateKey(path string) (ssh.Signer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	key, err := ssh.ParsePrivateKey(data)
	if err != nil {
		return nil, err
	}
	return key, nil
}
