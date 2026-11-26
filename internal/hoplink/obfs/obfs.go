package obfs

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
	"unsafe"

	"hoplink/internal/hoplink/log"

	"golang.org/x/crypto/blake2b"
)

const (
	SalamanderSaltSize = 8
	GeckoMinPacketSize = 512
	geckoMaxPacketSize = 1200
	maxUDPPacketSize   = 65535

	modeSalamander byte = 1
	modeGecko      byte = 2
)

var (
	wirePool = sync.Pool{
		New: func() any {
			b := make([]byte, 0, 2048)
			return &b
		},
	}
	datagramPool = sync.Pool{
		New: func() any {
			b := make([]byte, maxUDPPacketSize)
			return &b
		},
	}
)

func getWire(n int) (bufp *[]byte, buf []byte) {
	bufp = wirePool.Get().(*[]byte)
	buf = *bufp
	if cap(buf) < n {
		b := make([]byte, n)
		return &b, b
	}
	return bufp, buf[:n]
}

func putWire(bufp *[]byte) {
	if bufp == nil {
		return
	}
	*bufp = (*bufp)[:0]
	wirePool.Put(bufp)
}

// PacketConn implements Hysteria v2 Salamander and Gecko at the UDP boundary.
// Intermediate relays keep these packets opaque.
//
// Read and write locks sit on separate cache lines so a reader and a writer
// on the same socket do not false-share. mode is a byte so the salamander
// path is a single predicted compare. The 4 KiB salt block stays at the end
// so it does not push the hot fields out of the first lines.
type PacketConn struct {
	readMu      sync.Mutex
	mode        byte
	readScratch []byte
	_           [64]byte

	writeMu      sync.Mutex
	saltOff      int
	msgID        byte
	writeScratch []byte
	pc           net.PacketConn
	queue        []queuedPacket
	pending      map[geckoKey]*geckoAssembly
	modeName     string
	salt         [4096]byte
}

type geckoKey struct {
	addr string
	id   byte
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
	pw := []byte(password)
	readScratch := make([]byte, len(pw)+SalamanderSaltSize)
	writeScratch := make([]byte, len(pw)+SalamanderSaltSize)
	copy(readScratch, pw)
	copy(writeScratch, pw)
	var modeByte byte
	switch mode {
	case "salamander":
		modeByte = modeSalamander
	case "gecko":
		modeByte = modeGecko
	}
	return &PacketConn{
		pc:           pc,
		mode:         modeByte,
		modeName:     mode,
		readScratch:  readScratch,
		writeScratch: writeScratch,
		pending:      make(map[geckoKey]*geckoAssembly),
	}, nil
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
		wirep := datagramPool.Get().(*[]byte)
		wire := *wirep
		n, addr, err := c.pc.ReadFrom(wire)
		if err != nil {
			datagramPool.Put(wirep)
			if !errors.Is(err, net.ErrClosed) {
				log.Warnf("[HY2_UDP_READ_ERROR] peer=%s err=%v", addr, err)
			}
			return 0, addr, err
		}
		if log.TraceEnabled() {
			log.Tracef("[HY2_UDP_RX] peer=%s wire_bytes=%d obfs=%s", addr, n, c.modeName)
		}
		if c.mode == modeSalamander {
			copied, ok := c.unmaskInto(wire[:n], out)
			datagramPool.Put(wirep)
			if !ok {
				if log.TraceEnabled() {
					log.Tracef("[HY2_UDP_DROP] peer=%s wire_bytes=%d reason=invalid_obfs_packet obfs=%s", addr, n, c.modeName)
				}
				continue
			}
			return copied, addr, nil
		}
		plainp, plain, ok := c.unmask(wire[:n])
		datagramPool.Put(wirep)
		if !ok {
			if log.TraceEnabled() {
				log.Tracef("[HY2_UDP_DROP] peer=%s wire_bytes=%d reason=invalid_obfs_packet obfs=%s", addr, n, c.modeName)
			}
			continue
		}
		pooledPlain := plain
		plain, ok = c.ungecko(plain, addr)
		if !ok {
			putWire(plainp)
			if log.TraceEnabled() {
				log.Tracef("[HY2_UDP_FRAGMENT_PENDING_OR_DROP] peer=%s wire_bytes=%d obfs=%s", addr, n, c.modeName)
			}
			continue
		}
		releasePooled := cap(plain) == cap(pooledPlain)
		if !releasePooled {
			putWire(plainp)
		}
		if log.TraceEnabled() {
			log.Tracef("[HY2_UDP_DEOBFS] peer=%s packet_bytes=%d obfs=%s", addr, len(plain), c.modeName)
		}
		copied := copy(out, plain)
		if releasePooled {
			putWire(plainp)
		}
		return copied, addr, nil
	}
}

func (c *PacketConn) WriteTo(packet []byte, addr net.Addr) (int, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.mode == modeGecko && len(packet) > 0 && packet[0]&0x80 != 0 {
		return c.writeGecko(packet, addr)
	}
	wirep, wire, err := c.mask(packet)
	if err != nil {
		return 0, err
	}
	wireLen := len(wire)
	_, err = c.pc.WriteTo(wire, addr)
	putWire(wirep)
	if err != nil {
		log.Warnf("[HY2_UDP_WRITE_ERROR] peer=%s packet_bytes=%d wire_bytes=%d obfs=%s err=%v", addr, len(packet), wireLen, c.modeName, err)
		return 0, err
	}
	if log.TraceEnabled() {
		log.Tracef("[HY2_UDP_TX] peer=%s packet_bytes=%d wire_bytes=%d obfs=%s", addr, len(packet), wireLen, c.modeName)
	}
	return len(packet), nil
}

func (c *PacketConn) mask(packet []byte) (bufp *[]byte, wire []byte, err error) {
	bufp, wire = getWire(SalamanderSaltSize + len(packet))
	if err = c.readSalt(wire[:SalamanderSaltSize]); err != nil {
		putWire(bufp)
		return nil, nil, err
	}
	sum := c.sumSalt(c.writeScratch, wire[:SalamanderSaltSize])
	xorKeystream(wire[SalamanderSaltSize:], packet, &sum)
	return bufp, wire, nil
}

func (c *PacketConn) sumSalt(scratch, salt []byte) [32]byte {
	copy(scratch[len(scratch)-SalamanderSaltSize:], salt)
	return blake2b.Sum256(scratch)
}

func (c *PacketConn) readSalt(dst []byte) error {
	if c.saltOff+len(dst) > len(c.salt) {
		if _, err := io.ReadFull(rand.Reader, c.salt[:]); err != nil {
			return err
		}
		c.saltOff = 0
	}
	copy(dst, c.salt[c.saltOff:c.saltOff+len(dst)])
	c.saltOff += len(dst)
	return nil
}

func (c *PacketConn) unmaskInto(wire, out []byte) (int, bool) {
	if len(wire) <= SalamanderSaltSize {
		return 0, false
	}
	plainLen := len(wire) - SalamanderSaltSize
	if len(out) < plainLen {
		return 0, false
	}
	sum := c.sumSalt(c.readScratch, wire[:SalamanderSaltSize])
	xorKeystream(out[:plainLen], wire[SalamanderSaltSize:], &sum)
	return plainLen, true
}

func (c *PacketConn) unmask(wire []byte) (bufp *[]byte, plain []byte, ok bool) {
	if len(wire) <= SalamanderSaltSize {
		return nil, nil, false
	}
	plainLen := len(wire) - SalamanderSaltSize
	bufp, plain = getWire(plainLen)
	sum := c.sumSalt(c.readScratch, wire[:SalamanderSaltSize])
	xorKeystream(plain, wire[SalamanderSaltSize:], &sum)
	return bufp, plain, true
}

func xorKeystream(dst, src []byte, key *[32]byte) {
	n := len(src)
	if len(dst) < n {
		n = len(dst)
	}
	i := 0
	if n >= 32 {
		words := n / 8
		dstW := unsafe.Slice((*uint64)(unsafe.Pointer(unsafe.SliceData(dst))), words)
		srcW := unsafe.Slice((*uint64)(unsafe.Pointer(unsafe.SliceData(src))), words)
		k := (*[4]uint64)(unsafe.Pointer(key))
		blocks := words / 4
		for b := 0; b < blocks; b++ {
			base := b * 4
			dstW[base] = srcW[base] ^ k[0]
			dstW[base+1] = srcW[base+1] ^ k[1]
			dstW[base+2] = srcW[base+2] ^ k[2]
			dstW[base+3] = srcW[base+3] ^ k[3]
		}
		i = blocks * 32
	}
	for j := 0; i < n; i, j = i+1, j+1 {
		dst[i] = src[i] ^ key[j]
	}
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
	n, err := c.randomBetween(minCount, maxCount)
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
		cut, err := c.randomBetween(low, high)
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
		targetSize, err := c.randomBetween(minWire, maxWire)
		if err != nil {
			return 0, err
		}
		need := targetSize - SalamanderSaltSize - len(frame) - len(chunk)
		binary.BigEndian.PutUint16(frame[3:5], uint16(need))
		padding := make([]byte, need)
		if err := c.readSalt(padding); err != nil {
			return 0, err
		}
		frame = append(frame, padding...)
		frame = append(frame, chunk...)
		wirep, wire, err := c.mask(frame)
		if err != nil {
			return 0, err
		}
		if _, err = c.pc.WriteTo(wire, addr); err != nil {
			putWire(wirep)
			return 0, err
		}
		putWire(wirep)
	}
	return len(packet), nil
}

func (c *PacketConn) randomBetween(min, max int) (int, error) {
	if min > max {
		return 0, fmt.Errorf("invalid random range %d..%d", min, max)
	}
	span := uint64(max - min + 1)
	if span == 0 {
		return min, nil
	}
	// Unbiased draw from the same CSPRNG buffer as the salt. Rejection keeps
	// the distribution uniform, so padding lengths stay indistinguishable.
	const space = uint64(1) << 32
	limit := space / span * span
	var b [4]byte
	for {
		if err := c.readSalt(b[:]); err != nil {
			return 0, err
		}
		v := uint64(binary.LittleEndian.Uint32(b[:]))
		if v < limit {
			return min + int(v%span), nil
		}
	}
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
	key := geckoKey{addr: addr.String(), id: id}
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
