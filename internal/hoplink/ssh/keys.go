package ssh

import (
	"bytes"

	"golang.org/x/crypto/ssh"
)

func publicKeysEqual(a, b ssh.PublicKey) bool {
	return bytes.Equal(a.Marshal(), b.Marshal())
}
