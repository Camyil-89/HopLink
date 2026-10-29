package desktop

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"regexp"
	"strings"
	"time"
)

func (a *App) SaveSettings(sni string, port int, entryDomain string) (AppConfig, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.config.SNI = sni
	a.config.Port = port
	a.config.EntryDomain = strings.TrimSpace(entryDomain)
	if a.config.SNI == "" {
		a.config.SNI = "www.cloudflare.com"
	}
	if a.config.Port < 1 || a.config.Port > 65535 {
		return a.config, fmt.Errorf("порт должен быть от 1 до 65535")
	}
	if err := validateEntryDomain(a.config.EntryDomain); err != nil {
		return a.config, err
	}
	return a.config, a.saveLocked()
}

func validateEntryDomain(domain string) error {
	if domain == "" {
		return nil
	}
	if strings.ContainsAny(domain, ":/@?#[]\\ \t\r\n") {
		return fmt.Errorf("домен входного узла укажите без протокола, пути и порта")
	}
	if net.ParseIP(domain) != nil {
		return nil
	}
	if len(domain) > 253 || !regexp.MustCompile(`(?i)^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)(?:\.(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?))*\.?$`).MatchString(domain) {
		return fmt.Errorf("укажите корректный домен входного узла без порта")
	}
	return nil
}
func (a *App) SaveSecrets(chainKey, cert, certKey string) (AppConfig, error) {
	if (cert == "") != (certKey == "") {
		return a.GetState(), fmt.Errorf("сертификат и приватный ключ нужно задать вместе")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.config.ChainKey = chainKey
	a.config.Certificate = cert
	a.config.CertificateKey = certKey
	return a.config, a.saveLocked()
}

// Maintain the existing route key and certificate; only the exit receives them.
func (a *App) ensureSecrets() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.config.ChainKey == "" {
		b := make([]byte, 32)
		if _, e := rand.Read(b); e != nil {
			return e
		}
		a.config.ChainKey = hex.EncodeToString(b)
	}
	if a.config.Certificate == "" {
		sni := a.config.SNI
		if sni == "" {
			sni = "www.cloudflare.com"
		}
		key, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if e != nil {
			return e
		}
		serial, e := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
		if e != nil {
			return e
		}
		now := time.Now()
		tmpl := x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: sni}, NotBefore: now.Add(-time.Minute), NotAfter: now.AddDate(5, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true}
		if ip := net.ParseIP(sni); ip != nil {
			tmpl.IPAddresses = []net.IP{ip}
		} else {
			tmpl.DNSNames = []string{sni}
		}
		der, e := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
		if e != nil {
			return e
		}
		priv, e := x509.MarshalPKCS8PrivateKey(key)
		if e != nil {
			return e
		}
		a.config.Certificate = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
		a.config.CertificateKey = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: priv}))
	}
	return a.saveLocked()
}
