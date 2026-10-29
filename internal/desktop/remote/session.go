package remote

import (
	"bytes"
	"context"

	"golang.org/x/crypto/ssh"
)

func RunRemoteContext(ctx context.Context, client *ssh.Client, command string, input []byte) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()
	if input != nil {
		session.Stdin = bytes.NewReader(input)
	}
	type result struct {
		output string
		err    error
	}
	done := make(chan result, 1)
	go func() {
		output, err := session.CombinedOutput(command)
		done <- result{output: string(output), err: err}
	}()
	select {
	case <-ctx.Done():
		_ = session.Close()
		return "", ctx.Err()
	case completed := <-done:
		return completed.output, completed.err
	}
}

func RunServerContext(ctx context.Context, client *ssh.Client, sshUser, sshPassword, command string, input []byte) (string, error) {
	if sshUser == "root" {
		return RunRemoteContext(ctx, client, command, input)
	}
	payload := append([]byte(sshPassword+"\n"), input...)
	return RunRemoteContext(ctx, client, "sudo -S -p '' sh -c "+ShellQuote(command), payload)
}
