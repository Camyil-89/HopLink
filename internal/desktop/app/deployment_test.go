package desktop

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"math/big"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRouteNodeArgsSSHTransport(t *testing.T) {
	cfg := AppConfig{Port: 22033, SNI: "example.com", RelayTransport: "ssh"}
	order := []Server{{Host: "entry.example"}, {Host: "middle.example"}, {Host: "exit.example"}}
	entry := routeNodeArgs(cfg, order, 0, "/opt/route")
	if !containsAll(entry, "-relay-transport", "ssh", "-relay-ssh-next", "middle.example:22034") {
		t.Fatalf("entry args = %v", entry)
	}
	middle := routeNodeArgs(cfg, order, 1, "/opt/route")
	if !containsAll(middle, "-relay-ssh-listen", ":22034", "-relay-ssh-next", "exit.example:22034") {
		t.Fatalf("middle args = %v", middle)
	}
	exit := routeNodeArgs(cfg, order, 2, "/opt/route")
	if !containsAll(exit, "-mode", "hy2-forward", "-relay-ssh-listen", ":22034") {
		t.Fatalf("exit args = %v", exit)
	}
}

func containsAll(args []string, needles ...string) bool {
	joined := strings.Join(args, " ")
	for _, needle := range needles {
		if !strings.Contains(joined, needle) {
			return false
		}
	}
	return true
}

func TestRouteNodeArgsOnlyExitTerminatesTLS(t *testing.T) {
	cfg := AppConfig{Port: 22033, SNI: "example.com"}
	order := []Server{{Host: "entry.example"}, {Host: "middle.example"}, {Host: "exit.example"}}
	for i := 0; i < 2; i++ {
		args := routeNodeArgs(cfg, order, i, "/opt/route")
		want := []string{"/opt/route/HopLink", "-hy2-listen", ":22033", "-log-level", "warn", "-mode", "udp-relay", "-f", order[i+1].Host + ":22033"}
		if !reflect.DeepEqual(args, want) {
			t.Fatalf("relay args: %v, want %v", args, want)
		}
	}
	args := routeNodeArgs(cfg, order, 2, "/opt/route")
	flags := map[string]string{}
	for i := 1; i < len(args); i += 2 {
		flags[args[i]] = args[i+1]
	}
	if flags["-mode"] != "hy2-forward" || flags["-hy2-cert"] != "/opt/route/hy2-server.crt" || flags["-hy2-users-file"] != "/opt/route/users.txt" || flags["-f"] != "" || flags["-hy2-public-host"] != "" {
		t.Fatalf("exit args: %v", args)
	}
}

func TestRouteNodeArgsUseConfiguredLogLevelOnEveryNode(t *testing.T) {
	cfg := AppConfig{Port: 22034, SNI: "example.com", LogLevel: "debug"}
	order := []Server{{Host: "entry.example"}, {Host: "exit.example"}}
	for i := range order {
		args := routeNodeArgs(cfg, order, i, "/opt/route")
		if args[4] != "debug" {
			t.Fatalf("node %d log level = %q", i, args[4])
		}
	}
}

func TestRouteNodeArgsConfigureObfuscationWithoutCommandLineSecret(t *testing.T) {
	cfg := AppConfig{Port: 22033, SNI: "example.com", ObfsType: "gecko", ObfsPassword: "secret-mask"}
	order := []Server{{Host: "entry.example"}, {Host: "exit.example"}}
	relay := routeNodeArgs(cfg, order, 0, "/opt/route")
	if !reflect.DeepEqual(relay[len(relay)-2:], []string{"-relay-obfs", "gecko"}) {
		t.Fatalf("relay args = %v", relay)
	}
	exit := routeNodeArgs(cfg, order, 1, "/opt/route")
	joined := strings.Join(exit, " ")
	if !strings.Contains(joined, "-hy2-obfs gecko") || !strings.Contains(joined, "-hy2-obfs-key-file /opt/route/obfs.key") || strings.Contains(joined, cfg.ObfsPassword) {
		t.Fatalf("exit args expose password or omit settings: %v", exit)
	}
}

func TestJournalPeriodValidationAndUnitScope(t *testing.T) {
	if validLogLevel("verbose") || !validLogLevel("trace") {
		t.Fatal("log level validation failed")
	}
	_, unit := routeRemoteLayout("route-a", []Route{{ID: "route-a", Index: 2}})
	if unit != "hoplink-2.service" {
		t.Fatalf("unit = %q", unit)
	}
}

func TestRouteLinksIncludeObfuscationWhenEnabled(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(2), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	cfg := AppConfig{Port: 22033, SNI: "example.com", ObfsType: "salamander", ObfsPassword: "mask-password", EntryDomain: "entry.example", Certificate: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), CertificateKey: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))}
	links, err := routeUserLinks(cfg, Server{Host: "192.0.2.1"}, []User{{Name: "alice", Password: "user-password"}})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(links["alice"])
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Query().Get("obfs") != "salamander" || parsed.Query().Get("obfs-password") != cfg.ObfsPassword {
		t.Fatalf("missing obfuscation URI parameters: %s", links["alice"])
	}
}

func TestRouteLinksPreserveEntryAddressAndExistingCertificate(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	cfg := AppConfig{Port: 22033, SNI: "example.com", EntryDomain: "entry.example", Certificate: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), CertificateKey: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))}
	users := []User{{Name: "alice", Password: "a@b:c#%"}, {Name: "disabled", Password: "unused", Active: boolPointer(false)}}
	links, err := routeUserLinks(cfg, Server{Host: "192.0.2.1"}, users)
	if err != nil {
		t.Fatal(err)
	}
	pin := sha256.Sum256(der)
	// Preserve the certificate pin while allowing self-signed route certificates.
	want := (&url.URL{Scheme: "hysteria2", User: url.User(users[0].Password), Host: "entry.example:22033", Path: "/", RawQuery: url.Values{"insecure": {"1"}, "pinSHA256": {hex.EncodeToString(pin[:])}, "sni": {cfg.SNI}}.Encode(), Fragment: "HopLink-alice"}).String()
	if links["alice"] != want || len(links) != 1 {
		t.Fatalf("links changed or disabled user exposed: %v", links)
	}
}

func TestRouteLinksOmitSNIWhenUnset(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(3), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	cfg := AppConfig{Port: 22033, EntryDomain: "entry.example", Certificate: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), CertificateKey: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))}
	links, err := routeUserLinks(cfg, Server{Host: "192.0.2.1"}, []User{{Name: "alice", Password: "user-password"}})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(links["alice"])
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Query().Has("sni") {
		t.Fatalf("empty SNI was added to link: %s", links["alice"])
	}
}
