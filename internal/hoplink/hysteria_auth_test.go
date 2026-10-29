package hoplink

import (
	"crypto/x509"
	"encoding/pem"
	"net/url"
	"testing"
)

func TestHy2PasswordAuth(t *testing.T) {
	a := hy2PasswordAuth{password: "relay-secret", users: []hy2User{{name: "alice", password: "short"}, {name: "bob", password: "longer-secret"}}}
	for _, tc := range []struct {
		password, name string
		ok             bool
	}{
		{"relay-secret", "relay", true},
		{"short", "alice", true},
		{"longer-secret", "bob", true},
		{"wrong", "", false},
		{"", "", false},
	} {
		ok, name := a.Authenticate(nil, tc.password, 0)
		if ok != tc.ok || name != tc.name {
			t.Errorf("Authenticate(%q) = (%v, %q), want (%v, %q)", tc.password, ok, name, tc.ok, tc.name)
		}
	}
}

func TestMakeHy2ShareLinkEscapesPassword(t *testing.T) {
	password := "a@b:c#d% e"
	link := makeHy2ShareLink(password, "example.com", "22033", "example.com", "pin", "HopLink")
	u, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	if u.User == nil || u.User.Username() != password || u.Host != "example.com:22033" {
		t.Fatalf("unexpected parsed link: %q", link)
	}
}

func TestMakeHy2ShareLinkOmitsEmptySNI(t *testing.T) {
	link := makeHy2ShareLink("user-secret", "192.0.2.1", "22033", "", "pin", "HopLink")
	u, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	if u.Query().Has("sni") {
		t.Fatalf("empty SNI was added to link: %q", link)
	}
}

func TestGenerateHy2CertificateWithoutSNIHasNoNameSAN(t *testing.T) {
	certPEM, _, err := generateHy2Certificate("")
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("generated certificate PEM is empty")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(cert.DNSNames) != 0 || len(cert.IPAddresses) != 0 {
		t.Fatalf("empty SNI generated name SANs: DNS=%v IP=%v", cert.DNSNames, cert.IPAddresses)
	}
}
