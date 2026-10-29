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
	"testing"
	"time"
)

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

func TestJournalPeriodValidationAndUnitScope(t *testing.T) {
	if validLogLevel("verbose") || !validLogLevel("trace") {
		t.Fatal("log level validation failed")
	}
	_, unit := routeRemoteLayout("route-a", []Route{{ID: "route-a", Index: 2}})
	if unit != "hoplink-2.service" {
		t.Fatalf("unit = %q", unit)
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
	// Same URL format/pin as before the certificate moved to the exit.
	want := (&url.URL{Scheme: "hysteria2", User: url.User(users[0].Password), Host: "entry.example:22033", Path: "/", RawQuery: url.Values{"pinSHA256": {hex.EncodeToString(pin[:])}, "sni": {cfg.SNI}}.Encode(), Fragment: "HopLink-alice"}).String()
	if links["alice"] != want || len(links) != 1 {
		t.Fatalf("links changed or disabled user exposed: %v", links)
	}
}
