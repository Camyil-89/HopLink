package cascade

import (
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"strings"

	"hoplink/internal/hoplink/hysteria"
	"hoplink/internal/hoplink/obfs"

	hyclient "github.com/apernet/hysteria/core/v2/client"
)

type obfsConnFactory struct {
	mode, password string
}

func (f obfsConnFactory) New(_ net.Addr) (net.PacketConn, error) {
	// Local ephemeral port — same as hysteria udpConnFactory (must not bind to ServerAddr).
	pc, err := net.ListenUDP("udp", nil)
	if err != nil {
		return nil, err
	}
	hysteria.GrowUDPBuffers(pc)
	if f.mode == "" {
		return pc, nil
	}
	return obfs.NewPacketConn(pc, f.mode, f.password)
}

func loadCertPin(path string) (pin []byte, serverName string, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, "", fmt.Errorf("invalid PEM in %s", path)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(block.Bytes)
	return sum[:], cert.Subject.CommonName, nil
}

func newUpstreamClient(nextAddr, chainKey, sni, trustCert, obfsMode, obfsPassword string) (hyclient.Client, error) {
	nextAddr = strings.TrimSpace(nextAddr)
	if nextAddr == "" {
		return nil, nil
	}
	addr, err := net.ResolveUDPAddr("udp", nextAddr)
	if err != nil {
		return nil, fmt.Errorf("resolve cascade next %s: %w", nextAddr, err)
	}
	pin, certCN, err := loadCertPin(trustCert)
	if err != nil {
		return nil, err
	}
	tlsServerName := strings.TrimSpace(sni)
	if tlsServerName == "" {
		tlsServerName = certCN
	}
	configFunc := func() (*hyclient.Config, error) {
		return &hyclient.Config{
			ServerAddr: addr,
			Auth:       chainKey,
			ConnFactory: obfsConnFactory{
				mode:     obfsMode,
				password: obfsPassword,
			},
			TLSConfig: hyclient.TLSConfig{
				ServerName:         tlsServerName,
				InsecureSkipVerify: true,
				VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
					if len(rawCerts) == 0 {
						return fmt.Errorf("upstream certificate missing")
					}
					got := sha256.Sum256(rawCerts[0])
					if subtle.ConstantTimeCompare(pin, got[:]) != 1 {
						return fmt.Errorf("upstream certificate pin mismatch")
					}
					return nil
				},
			},
			CongestionConfig: hyclient.CongestionConfig{Type: "bbr", BBRProfile: "standard"},
			QUICConfig: hyclient.QUICConfig{
				InitialStreamReceiveWindow:     16 << 20,
				MaxStreamReceiveWindow:         16 << 20,
				InitialConnectionReceiveWindow: 32 << 20,
				MaxConnectionReceiveWindow:     32 << 20,
			},
			BandwidthConfig: hyclient.BandwidthConfig{MaxRx: 1_000_000_000},
		}, nil
	}
	return hyclient.NewReconnectableClient(configFunc, nil, true)
}