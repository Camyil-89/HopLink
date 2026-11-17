package cascade

import (
	"bytes"
	"testing"
)

// Typical entry-route ruleset (Clash/sing-box style), sized like a modest deploy list.
const benchPatternsText = `domain:ru
domain:su
domain:vk.com
domain:yandex.ru
domain:mail.ru
keyword:2gis
keyword:moscow
full:localhost
(?i)\.xn--p1ai$
domain:cn
keyword:baidu
domain:telegram.org
domain:t.me
domain:googlevideo.com
keyword:whatsapp
domain:apple.com
domain:icloud.com
domain:microsoft.com
domain:office.com
domain:github.com
domain:stackoverflow.com
domain:netflix.com
domain:spotify.com
domain:twitter.com
domain:instagram.com
domain:facebook.com
domain:amazon.com
domain:cloudflare.com
domain:fastly.net
domain:akamai.net
domain:doubleclick.net
domain:googlesyndication.com
domain:googleapis.com
domain:gstatic.com
domain:mozilla.org
domain:wikipedia.org
domain:reddit.com
domain:discord.com
domain:slack.com
domain:zoom.us
domain:teams.microsoft.com
`

func benchMatcher(b *testing.B) Matcher {
	b.Helper()
	m, err := LoadPatternsFromText(benchPatternsText)
	if err != nil {
		b.Fatal(err)
	}
	return m
}

func BenchmarkMatcher_LocalDomain(b *testing.B) {
	m := benchMatcher(b)
	host := "api.vk.com"
	b.ReportAllocs()
	b.SetBytes(int64(len(host)))
	for b.Loop() {
		_ = m.Match(host)
	}
}

func BenchmarkMatcher_ForeignDomain(b *testing.B) {
	m := benchMatcher(b)
	host := "www.google.com"
	b.ReportAllocs()
	b.SetBytes(int64(len(host)))
	for b.Loop() {
		_ = m.Match(host)
	}
}

func BenchmarkMatcher_MixedHosts(b *testing.B) {
	m := benchMatcher(b)
	hosts := []string{
		"mail.ru", "google.com", "yandex.ru", "github.com", "cdn.example.net",
		"2gis.ru", "netflix.com", "foo.bar", "telegram.org", "wikipedia.org",
	}
	b.ReportAllocs()
	for b.Loop() {
		h := hosts[b.N%len(hosts)]
		_ = m.Match(h)
	}
}

func BenchmarkRoutingDecision_DomainOnly(b *testing.B) {
	m := benchMatcher(b)
	req := "example.com:443"
	host := hostFromReqAddr(req)
	b.ReportAllocs()
	for b.Loop() {
		_ = m.Match(host)
	}
}

func BenchmarkRoutingDecision_DomainAndSNI(b *testing.B) {
	domainM := benchMatcher(b)
	sniM, err := LoadPatternsFromText(`domain:ru\nkeyword:cdn`)
	if err != nil {
		b.Fatal(err)
	}
	host := "cdn.cloudflare.com"
	sni := "edge.example.ru"
	b.ReportAllocs()
	for b.Loop() {
		local := domainM.Match(host)
		if !local && sni != "" {
			local = sniM.Match(sni)
		}
		_ = local
	}
}

func BenchmarkParseTLSSNI(b *testing.B) {
	record := minimalTLSClientHello("video.google.com")
	if _, ok := parseTLSSNI(record); !ok {
		b.Fatal("fixture must parse")
	}
	b.SetBytes(int64(len(record)))
	b.ReportAllocs()
	for b.Loop() {
		_, _ = parseTLSSNI(record)
	}
}

func BenchmarkParseTLSSNI_NoTLS(b *testing.B) {
	record := []byte("GET / HTTP/1.1\r\nHost: example.com\r\n\r\n")
	b.SetBytes(int64(len(record)))
	b.ReportAllocs()
	for b.Loop() {
		_, _ = parseTLSSNI(record)
	}
}

// minimalTLSClientHello builds a TLS 1.2 ClientHello with one SNI hostname.
func minimalTLSClientHello(host string) []byte {
	hostBytes := []byte(host)
	sniEntry := append([]byte{0}, byte(len(hostBytes)>>8), byte(len(hostBytes)))
	sniEntry = append(sniEntry, hostBytes...)
	sniList := append([]byte{byte(len(sniEntry) >> 8), byte(len(sniEntry))}, sniEntry...)
	sniExt := append([]byte{0, 0, byte(len(sniList) >> 8), byte(len(sniList))}, sniList...)

	body := make([]byte, 0, 96+len(sniExt))
	body = append(body, 0x03, 0x03)
	body = append(body, bytes.Repeat([]byte{0x11}, 32)...)
	body = append(body, 0)                // session id length
	body = append(body, 0, 2, 0xff, 0x01) // cipher suites
	body = append(body, 1, 0)             // compression
	extLen := len(sniExt)
	body = append(body, byte(extLen>>8), byte(extLen))
	body = append(body, sniExt...)

	handshake := make([]byte, 0, 4+len(body))
	handshake = append(handshake, 0x01)
	hsLen := len(body)
	handshake = append(handshake, byte(hsLen>>16), byte(hsLen>>8), byte(hsLen))
	handshake = append(handshake, body...)

	record := make([]byte, 0, 5+len(handshake))
	record = append(record, 0x16, 0x03, 0x01)
	record = append(record, byte(len(handshake)>>8), byte(len(handshake)))
	record = append(record, handshake...)
	return record
}

func BenchmarkHostFromReqAddr(b *testing.B) {
	req := "api.vk.com:443"
	b.ReportAllocs()
	for b.Loop() {
		_ = hostFromReqAddr(req)
	}
}

func BenchmarkEntryPath_TCP(b *testing.B) {
	m := benchMatcher(b)
	record := minimalTLSClientHello("video.google.com")
	req := "cdn.example.ru:443"
	host := hostFromReqAddr(req)
	b.ReportAllocs()
	for b.Loop() {
		var sni string
		if parsed, ok := parseTLSSNI(record); ok {
			sni = parsed
		}
		local := m.Match(host)
		if !local && sni != "" {
			local = m.Match(sni)
		}
		_ = local
	}
}
