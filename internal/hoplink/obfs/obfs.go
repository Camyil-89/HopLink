package obfs

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"sync"
	"time"

	"hoplink/internal/hoplink/log"

	"golang.org/x/crypto/blake2b"
)

const (
	SalamanderSaltSize = 8
	GeckoMinPacketSize = 512
	geckoMaxPacketSize = 1200
	maxUDPPacketSize   = 65535
)

// PacketConn implements Hysteria v2 Salamander and Gecko at the UDP boundary.
// Intermediate relays keep these packets opaque.
type PacketConn struct {
	pc       net.PacketConn
	mode     string
	password []byte
	writeMu  sync.Mutex
	readMu   sync.Mutex
	queue    []queuedPacket
	msgID    byte
	pending  map[string]*geckoAssembly
}

type queuedPacket struct {
	data []byte
	addr net.Addr
}

type geckoAssembly struct {
	created time.Time
	count   int
	chunks  [][]byte
	seen    int
}

func NewPacketConn(pc net.PacketConn, mode, password string) (net.PacketConn, error) {
	if mode == "" {
		return pc, nil
	}
	if mode != "salamander" && mode != "gecko" {
		return nil, fmt.Errorf("unsupported UDP obfuscation %q", mode)
	}
	if len([]byte(password)) < 4 {
		return nil, errors.New("UDP obfuscation password must contain at least 4 bytes")
	}
	return &PacketConn{pc: pc, mode: mode, password: []byte(password), pending: make(map[string]*geckoAssembly)}, nil
}

func (c *PacketConn) ReadFrom(out []byte) (int, net.Addr, error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()
	for {
		if len(c.queue) != 0 {
			p := c.queue[0]
			c.queue = c.queue[1:]
			return copy(out, p.data), p.addr, nil
		}
		wire := make([]byte, maxUDPPacketSize)
		n, addr, err := c.pc.ReadFrom(wire)
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				log.Warnf("[HY2_UDP_READ_ERROR] peer=%s err=%v", addr, err)
			}
			return 0, addr, err
		}
		log.Tracef("[HY2_UDP_RX] peer=%s wire_bytes=%d obfs=%s", addr, n, c.mode)
		plain, ok := c.unmask(wire[:n])
		if !ok {
			log.Tracef("[HY2_UDP_DROP] peer=%s wire_bytes=%d reason=invalid_obfs_packet obfs=%s", addr, n, c.mode)
			continue
		}
		if c.mode == "gecko" {
			plain, ok = c.ungecko(plain, addr)
			if !ok {
				log.Tracef("[HY2_UDP_FRAGMENT_PENDING_OR_DROP] peer=%s wire_bytes=%d obfs=%s", addr, n, c.mode)
				continue
			}
		}
		log.Tracef("[HY2_UDP_DEOBFS] peer=%s packet_bytes=%d obfs=%s", addr, len(plain), c.mode)
		return copy(out, plain), addr, nil
	}
}

func (c *PacketConn) WriteTo(packet []byte, addr net.Addr) (int, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.mode == "gecko" && len(packet) > 0 && packet[0]&0x80 != 0 {
		return c.writeGecko(packet, addr)
	}
	wire, err := c.mask(packet)
	if err != nil {
		return 0, err
	}
	_, err = c.pc.WriteTo(wire, addr)
	if err != nil {
		log.Warnf("[HY2_UDP_WRITE_ERROR] peer=%s packet_bytes=%d wire_bytes=%d obfs=%s err=%v", addr, len(packet), len(wire), c.mode, err)
		return 0, err
	}
	log.Tracef("[HY2_UDP_TX] peer=%s packet_bytes=%d wire_bytes=%d obfs=%s", addr, len(packet), len(wire), c.mode)
	return len(packet), nil
}

func (c *PacketConn) mask(packet []byte) ([]byte, error) {
	salt := make([]byte, SalamanderSaltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	hash := salamanderHash(c.password, salt)
	wire := make([]byte, SalamanderSaltSize+len(packet))
	copy(wire, salt)
	for i, value := range packet {
		wire[SalamanderSaltSize+i] = value ^ hash[i%len(hash)]
	}
	return wire, nil
}

func (c *PacketConn) unmask(wire []byte) ([]byte, bool) {
	if len(wire) <= SalamanderSaltSize {
		return nil, false
	}
	salt := wire[:SalamanderSaltSize]
	hash := salamanderHash(c.password, salt)
	plain := make([]byte, len(wire)-SalamanderSaltSize)
	for i, value := range wire[SalamanderSaltSize:] {
		plain[i] = value ^ hash[i%len(hash)]
	}
	return plain, true
}

func salamanderHash(password, salt []byte) []byte {
	data := make([]byte, 0, len(password)+len(salt))
	data = append(data, password...)
	data = append(data, salt...)
	hash := blake2b.Sum256(data)
	return hash[:]
}

func (c *PacketConn) writeGecko(packet []byte, addr net.Addr) (int, error) {
	minCount := (len(packet) + geckoMaxPacketSize - SalamanderSaltSize - 5 - 1) / (geckoMaxPacketSize - SalamanderSaltSize - 5)
	if minCount < 2 {
		minCount = 2
	}
	maxCount := len(packet)
	if maxCount > 8 {
		maxCount = 8
	}
	if maxCount < minCount {
		return 0, fmt.Errorf("Gecko cannot fragment a %d-byte QUIC packet into at most eight datagrams", len(packet))
	}
	n, err := randomBetween(minCount, maxCount)
	if err != nil {
		return 0, err
	}
	chunks := make([][]byte, 0, n)
	remaining := packet
	for index := 0; index < n-1; index++ {
		average := len(remaining) / (n - index)
		low, high := average*3/4, average*5/4
		if low < 1 {
			low = 1
		}
		if high > geckoMaxPacketSize-SalamanderSaltSize-5 {
			high = geckoMaxPacketSize - SalamanderSaltSize - 5
		}
		if high >= len(remaining)-(n-index-1) {
			high = len(remaining) - (n - index - 1)
		}
		if low > high {
			low = high
		}
		cut, err := randomBetween(low, high)
		if err != nil {
			return 0, err
		}
		chunks = append(chunks, remaining[:cut])
		remaining = remaining[cut:]
	}
	chunks = append(chunks, remaining)
	c.msgID++
	id := c.msgID
	for index, chunk := range chunks {
		frame := make([]byte, 5, geckoMaxPacketSize-SalamanderSaltSize)
		frame[0], frame[1] = 0x80, id
		frame[2] = byte(index<<4 | n)
		minWire := GeckoMinPacketSize
		if minWire < SalamanderSaltSize+len(frame)+len(chunk) {
			minWire = SalamanderSaltSize + len(frame) + len(chunk)
		}
		maxWire := geckoMaxPacketSize
		if minWire > maxWire {
			return 0, fmt.Errorf("Gecko fragment exceeds %d bytes", geckoMaxPacketSize)
		}
		targetSize, err := randomBetween(minWire, maxWire)
		if err != nil {
			return 0, err
		}
		need := targetSize - SalamanderSaltSize - len(frame) - len(chunk)
		binary.BigEndian.PutUint16(frame[3:5], uint16(need))
		padding := make([]byte, need)
		if _, err := io.ReadFull(rand.Reader, padding); err != nil {
			return 0, err
		}
		frame = append(frame, padding...)
		frame = append(frame, chunk...)
		wire, err := c.mask(frame)
		if err != nil {
			return 0, err
		}
		if _, err = c.pc.WriteTo(wire, addr); err != nil {
			return 0, err
		}
	}
	return len(packet), nil
}

func randomBetween(min, max int) (int, error) {
	if min > max {
		return 0, fmt.Errorf("invalid random range %d..%d", min, max)
	}
	value, err := rand.Int(rand.Reader, big.NewInt(int64(max-min+1)))
	if err != nil {
		return 0, err
	}
	return min + int(value.Int64()), nil
}

func (c *PacketConn) ungecko(packet []byte, addr net.Addr) ([]byte, bool) {
	if len(packet) == 0 || packet[0]&0x80 == 0 {
		return packet, len(packet) != 0
	}
	if len(packet) < 5 || packet[0] != 0x80 {
		return nil, false
	}
	id := packet[1]
	index, count := int(packet[2]>>4), int(packet[2]&0x0f)
	pad := int(binary.BigEndian.Uint16(packet[3:5]))
	if count < 2 || count > 8 || index >= count || pad > len(packet)-5 {
		return nil, false
	}
	chunk := append([]byte(nil), packet[5+pad:]...)
	key := addr.String() + "/" + string([]byte{id})
	for k, old := range c.pending {
		if time.Since(old.created) > 3*time.Second {
			delete(c.pending, k)
		}
	}
	assembly := c.pending[key]
	if assembly == nil {
		if len(c.pending) >= 2048 {
			return nil, false
		}
		assembly = &geckoAssembly{created: time.Now(), count: count, chunks: make([][]byte, count)}
		c.pending[key] = assembly
	}
	if assembly.count != count || assembly.chunks[index] != nil {
		delete(c.pending, key)
		return nil, false
	}
	assembly.chunks[index] = chunk
	assembly.seen++
	if assembly.seen != assembly.count {
		return nil, false
	}
	var complete []byte
	for _, part := range assembly.chunks {
		complete = append(complete, part...)
	}
	delete(c.pending, key)
	return complete, len(complete) != 0
}

func (c *PacketConn) Close() error                       { return c.pc.Close() }
func (c *PacketConn) LocalAddr() net.Addr                { return c.pc.LocalAddr() }
func (c *PacketConn) SetDeadline(t time.Time) error      { return c.pc.SetDeadline(t) }
func (c *PacketConn) SetReadDeadline(t time.Time) error  { return c.pc.SetReadDeadline(t) }
func (c *PacketConn) SetWriteDeadline(t time.Time) error { return c.pc.SetWriteDeadline(t) }
