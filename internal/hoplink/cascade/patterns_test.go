package cascade

import "testing"

func TestJSONStyleLines(t *testing.T) {
	m, err := LoadPatternsFromText(`"domain:ru",
"domain:vk.com",`)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Match("vk.com") || !m.Match("mail.ru") {
		t.Fatal("json-styled rules")
	}
}

func TestRoutingRuleFormats(t *testing.T) {
	m, err := LoadPatternsFromText(`domain:vk.com
domain:ru
keyword:2gis
full:exact.host`)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]bool{
		"vk.com": true, "api.vk.com": true, "notvk.com": false,
		"yandex.ru": true, "google.com": false,
		"2gis.com": true, "foo2gisbar.net": true,
		"exact.host": true, "sub.exact.host": false,
	}
	for host, want := range cases {
		if m.Match(host) != want {
			t.Fatalf("host %q want %v", host, want)
		}
	}
}

func TestMatcherMatchDomain(t *testing.T) {
	m, err := LoadPatternsFromText(`(?i)\.ru$`)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Match("yandex.ru") || m.Match("google.com") {
		t.Fatal("domain regex mismatch")
	}
}

func TestHostFromReqAddr(t *testing.T) {
	if hostFromReqAddr("example.com:443") != "example.com" {
		t.Fatal(hostFromReqAddr("example.com:443"))
	}
	if hostFromReqAddr("[2001:db8::1]:443") != "2001:db8::1" {
		t.Fatal(hostFromReqAddr("[2001:db8::1]:443"))
	}
}

func TestParseTLSSNI(t *testing.T) {
	// Minimal fake ClientHello is brittle; verify non-TLS returns false.
	if _, ok := parseTLSSNI([]byte("GET /")); ok {
		t.Fatal("expected no SNI for HTTP")
	}
}