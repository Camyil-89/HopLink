package wire

import (
	"bytes"
	"io"
	"testing"
)

func BenchmarkEncodeDecode_256B(b *testing.B) {
	benchmarkEncodeDecode(b, 256)
}

func BenchmarkEncodeDecode_1200B(b *testing.B) {
	benchmarkEncodeDecode(b, 1200)
}

func BenchmarkEncodeDecode_8192B(b *testing.B) {
	benchmarkEncodeDecode(b, 8192)
}

func benchmarkEncodeDecode(b *testing.B, payLen int) {
	src := "203.0.113.42:54321"
	payload := make([]byte, payLen)
	b.SetBytes(int64(payLen + len(src)))
	b.ReportAllocs()
	for b.Loop() {
		frame, err := EncodeBytes(src, payload)
		if err != nil {
			b.Fatal(err)
		}
		_, out, err := DecodeBytes(frame)
		if err != nil {
			b.Fatal(err)
		}
		ReleaseFrame(frame)
		if len(out) != payLen {
			b.Fatal("size mismatch")
		}
	}
}

func BenchmarkWriteRead_1200B(b *testing.B) {
	src := "127.0.0.1:22033"
	payload := make([]byte, 1200)
	var buf bytes.Buffer
	b.SetBytes(int64(1200))
	b.ReportAllocs()
	for b.Loop() {
		buf.Reset()
		if err := Write(&buf, src, payload); err != nil {
			b.Fatal(err)
		}
		_, out, err := Read(&buf)
		if err != nil {
			b.Fatal(err)
		}
		if len(out) != 1200 {
			b.Fatal("size mismatch")
		}
	}
}

func BenchmarkWriteRead_Pipe_1200B(b *testing.B) {
	src := "10.0.0.1:1"
	payload := make([]byte, 1200)
	pr, pw := io.Pipe()
	done := make(chan struct{})
	go func() {
		buf := make([]byte, 1200)
		for {
			_, out, err := Read(pr)
			if err != nil {
				close(done)
				return
			}
			if len(out) != len(buf) {
				close(done)
				return
			}
		}
	}()
	b.Cleanup(func() {
		_ = pr.Close()
		<-done
	})

	b.SetBytes(int64(1200))
	b.ReportAllocs()
	for b.Loop() {
		if err := Write(pw, src, payload); err != nil {
			b.Fatal(err)
		}
	}
}