package wire

import (
	"bytes"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	payload := bytes.Repeat([]byte{0xcd}, 1200)
	frame, err := EncodeBytes("127.0.0.1:54321", payload)
	if err != nil {
		t.Fatal(err)
	}
	src, out, err := DecodeBytes(frame)
	if err != nil {
		t.Fatal(err)
	}
	if src != "127.0.0.1:54321" || !bytes.Equal(out, payload) {
		t.Fatal("round trip mismatch")
	}
	ReleaseFrame(frame)
}

func TestWalkFramesBatches(t *testing.T) {
	a, err := EncodeBytes("10.0.0.1:1", []byte("a"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := EncodeBytes("10.0.0.2:2", []byte("bb"))
	if err != nil {
		t.Fatal(err)
	}
	batch := append(append([]byte(nil), a...), b...)
	ReleaseFrame(a)
	ReleaseFrame(b)
	var got []string
	if err := WalkFrames(batch, func(src string, payload []byte) error {
		got = append(got, src+":"+string(payload))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "10.0.0.1:1:a" || got[1] != "10.0.0.2:2:bb" {
		t.Fatalf("batch mismatch: %#v", got)
	}
}

func TestWriteReadRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	payload := []byte("hoplink-wire")
	if err := Write(&buf, "127.0.0.1:1", payload); err != nil {
		t.Fatal(err)
	}
	src, out, err := Read(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if src != "127.0.0.1:1" || !bytes.Equal(out, payload) {
		t.Fatal("stream round trip mismatch")
	}
}
