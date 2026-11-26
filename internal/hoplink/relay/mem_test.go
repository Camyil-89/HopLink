package relay

import (
	"context"
	"net"
	"runtime"
	"testing"
	"time"
)

func heapInuseBytes() uint64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapInuse
}

func quicHandshake(t *testing.T, peer *net.UDPConn) {
	t.Helper()
	initial := quicLikePacket(1200)
	if _, err := peer.Write(initial); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, maxUDPPacketSize)
	if _, err := peer.Read(buf); err != nil {
		t.Fatal(err)
	}
}

// Tests RSS-like heap footprint of idle relay sessions (Go heap only; kernel UDP buffers are separate).
func TestRelayHeapIdleSessions(t *testing.T) {
	const sessions = 64
	echo := udpEcho(t)
	r := startTestRelay(t, echo.LocalAddr().String(), time.Minute)

	heapBefore := heapInuseBytes()
	peers := make([]*net.UDPConn, sessions)
	for i := range peers {
		p, err := net.DialUDP("udp", nil, r.LocalAddr().(*net.UDPAddr))
		if err != nil {
			t.Fatal(err)
		}
		peers[i] = p
		defer p.Close()
		quicHandshake(t, p)
	}
	time.Sleep(50 * time.Millisecond)
	runtime.GC()
	heapAfter := heapInuseBytes()
	if heapAfter < heapBefore {
		heapBefore = heapAfter
	}
	perSession := (heapAfter - heapBefore) / sessions
	t.Logf("relay idle: sessions=%d heap_inuse_total_mb=%.2f delta_mb=%.2f per_session_kb=%d",
		sessions,
		float64(heapAfter)/(1024*1024),
		float64(heapAfter-heapBefore)/(1024*1024),
		perSession/1024,
	)
	// Per upstream session: was 8MiB×2 (GrowUDPBuffers); now 512KiB×2 (GrowRelaySessionBuffers).
	t.Logf("kernel_udp_upstream_kb_per_session: before=%d after=%d", (16<<20)/1024, (1<<20)/1024)
	if r.SessionCount() != sessions {
		t.Fatalf("sessions want %d got %d", sessions, r.SessionCount())
	}
}

func TestRelayHeapUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("load test")
	}
	const (
		sessions   = 32
		payload    = 1200
		iterations = 4000
	)
	echo := udpEcho(t)
	r := startTestRelay(t, echo.LocalAddr().String(), time.Minute)

	peers := make([]*net.UDPConn, sessions)
	bufs := make([][]byte, sessions)
	packet := quicLikePacket(payload)
	packet[0] = 0x40
	for i := range peers {
		p, err := net.DialUDP("udp", nil, r.LocalAddr().(*net.UDPAddr))
		if err != nil {
			t.Fatal(err)
		}
		peers[i] = p
		defer p.Close()
		quicHandshake(t, p)
		bufs[i] = make([]byte, maxUDPPacketSize)
	}

	heapPeak := heapInuseBytes()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for i := range peers {
			if _, err := peers[i].Write(packet); err != nil {
				t.Fatal(err)
			}
			if _, err := peers[i].Read(bufs[i]); err != nil {
				t.Fatal(err)
			}
		}
		if h := heapInuseBytes(); h > heapPeak {
			heapPeak = h
		}
	}
	runtime.GC()
	heapAfter := heapInuseBytes()
	t.Logf("relay load: sessions=%d heap_inuse_mb=%.2f heap_peak_mb=%.2f queued_bytes=%d",
		sessions,
		float64(heapAfter)/(1024*1024),
		float64(heapPeak)/(1024*1024),
		r.QueuedBytes(),
	)
}

func BenchmarkRelayMemRoundTrip(b *testing.B) {
	echo := benchEchoUDP(b)
	r := benchRelay(b, echo.LocalAddr().String(), 30*time.Second)
	peer, err := net.DialUDP("udp", nil, r.LocalAddr().(*net.UDPAddr))
	if err != nil {
		b.Fatal(err)
	}
	defer peer.Close()
	initial := quicLikePacket(1200)
	if _, err := peer.Write(initial); err != nil {
		b.Fatal(err)
	}
	buf := make([]byte, maxUDPPacketSize)
	if _, err := peer.Read(buf); err != nil {
		b.Fatal(err)
	}
	packet := quicLikePacket(1200)
	packet[0] = 0x40
	b.ReportAllocs()
	b.SetBytes(1200)
	b.ResetTimer()
	for b.Loop() {
		if _, err := peer.Write(packet); err != nil {
			b.Fatal(err)
		}
		if _, err := peer.Read(buf); err != nil {
			b.Fatal(err)
		}
	}
}

func TestRelayServeDoesNotLeakOnShutdown(t *testing.T) {
	echo := udpEcho(t)
	r, err := New("127.0.0.1:0", echo.LocalAddr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Serve(ctx) }()
	p, err := net.DialUDP("udp", nil, r.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatal(err)
	}
	quicHandshake(t, p)
	_ = p.Close()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("relay shutdown timeout")
	}
	if r.QueuedBytes() != 0 {
		t.Fatalf("queued bytes after close: %d", r.QueuedBytes())
	}
}