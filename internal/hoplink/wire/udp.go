package wire

import (
	"encoding/binary"
	"fmt"
	"io"
	"sync"
	"unsafe"
)

const MaxPayload = 65535

const (
	smallFrameCap = 2048
	largeFrameCap = 16 << 10
)

var (
	smallFramePool = sync.Pool{New: func() any { return new([smallFrameCap]byte) }}
	largeFramePool = sync.Pool{New: func() any { return new([largeFrameCap]byte) }}
)

func Write(w io.Writer, src string, payload []byte) error {
	frame, err := EncodeBytes(src, payload)
	if err != nil {
		return err
	}
	_, err = w.Write(frame)
	ReleaseFrame(frame)
	return err
}

func Read(r io.Reader) (string, []byte, error) {
	var hdr [6]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return "", nil, err
	}
	srcLen := int(binary.BigEndian.Uint16(hdr[0:2]))
	payLen := binary.BigEndian.Uint32(hdr[2:6])
	if payLen > MaxPayload {
		return "", nil, fmt.Errorf("relay udp payload too large")
	}
	body := make([]byte, srcLen+int(payLen))
	if _, err := io.ReadFull(r, body); err != nil {
		return "", nil, err
	}
	return string(body[:srcLen]), body[srcLen:], nil
}

func EncodeBytes(src string, payload []byte) ([]byte, error) {
	if len(src) > 65535 || len(payload) > MaxPayload {
		return nil, fmt.Errorf("relay udp frame too large")
	}
	need := 6 + len(src) + len(payload)
	buf := takeFrame(need)
	binary.BigEndian.PutUint16(buf[0:2], uint16(len(src)))
	binary.BigEndian.PutUint32(buf[2:6], uint32(len(payload)))
	copy(buf[6:], src)
	copy(buf[6+len(src):], payload)
	return buf, nil
}

func takeFrame(n int) []byte {
	switch {
	case n <= smallFrameCap:
		a := smallFramePool.Get().(*[smallFrameCap]byte)
		return a[:n]
	case n <= largeFrameCap:
		a := largeFramePool.Get().(*[largeFrameCap]byte)
		return a[:n]
	default:
		return make([]byte, n)
	}
}

func ReleaseFrame(frame []byte) {
	if len(frame) == 0 && cap(frame) == 0 {
		return
	}
	switch cap(frame) {
	case smallFrameCap:
		smallFramePool.Put((*[smallFrameCap]byte)(unsafe.Pointer(unsafe.SliceData(frame))))
	case largeFrameCap:
		largeFramePool.Put((*[largeFrameCap]byte)(unsafe.Pointer(unsafe.SliceData(frame))))
	}
}

func WalkFrames(frame []byte, fn func(src string, payload []byte) error) error {
	for len(frame) > 0 {
		if len(frame) < 6 {
			return fmt.Errorf("relay udp frame too short")
		}
		srcLen := int(binary.BigEndian.Uint16(frame[0:2]))
		payLen := binary.BigEndian.Uint32(frame[2:6])
		end := 6 + srcLen + int(payLen)
		if end > len(frame) {
			return fmt.Errorf("relay udp frame truncated")
		}
		if payLen > MaxPayload {
			return fmt.Errorf("relay udp payload too large")
		}
		if err := fn(string(frame[6:6+srcLen]), frame[6+srcLen:end]); err != nil {
			return err
		}
		frame = frame[end:]
	}
	return nil
}

func DecodeBytes(frame []byte) (string, []byte, error) {
	if len(frame) < 6 {
		return "", nil, fmt.Errorf("relay udp frame too short")
	}
	srcLen := binary.BigEndian.Uint16(frame[0:2])
	payLen := binary.BigEndian.Uint32(frame[2:6])
	end := 6 + int(srcLen) + int(payLen)
	if end > len(frame) {
		return "", nil, fmt.Errorf("relay udp frame truncated")
	}
	if payLen > MaxPayload {
		return "", nil, fmt.Errorf("relay udp payload too large")
	}
	src := string(frame[6 : 6+srcLen])
	payload := frame[6+srcLen : end]
	return src, payload, nil
}

func PayloadCopy(payload []byte) []byte {
	return append([]byte(nil), payload...)
}
