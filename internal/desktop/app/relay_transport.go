package desktop

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"net"
	"strconv"

	"golang.org/x/crypto/ssh"
)

func validRelayTransport(mode string) bool {
	return mode == "" || mode == "udp" || mode == "ssh" || mode == "ws"
}

func relayTransportPort(port int) int {
	if port <= 0 || port >= 65535 {
		return 0
	}
	return port + 1
}

func relayTransportHostPort(host string, port int) string {
	return net.JoinHostPort(host, strconv.Itoa(relayTransportPort(port)))
}

func (a *App) ensureRelayTransportSecrets() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.ensureRelayTransportSecretsLocked()
}

func (a *App) ensureRelayTransportSecretsLocked() error {
	if a.config.RelayTransport != "ssh" {
		return nil
	}
	if a.config.RelayTransportClientKey != "" && a.config.RelayTransportHostKey != "" && a.config.RelayTransportClientPub != "" && a.config.RelayTransportHostPub != "" {
		return nil
	}
	_, clientPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	_, hostPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	a.config.RelayTransportClientKey = ed25519PrivatePEM(clientPriv)
	a.config.RelayTransportClientPub = ed25519AuthorizedLine(clientPriv.Public().(ed25519.PublicKey))
	a.config.RelayTransportHostKey = ed25519PrivatePEM(hostPriv)
	a.config.RelayTransportHostPub = ed25519AuthorizedLine(hostPriv.Public().(ed25519.PublicKey))
	return a.saveLocked()
}

func ed25519PrivatePEM(key ed25519.PrivateKey) string {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		panic(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func ed25519AuthorizedLine(pub ed25519.PublicKey) string {
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		panic(err)
	}
	return string(ssh.MarshalAuthorizedKey(sshPub))
}
