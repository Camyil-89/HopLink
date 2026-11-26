//go:build !linux

package cascade

import "syscall"

func controlOutbound(_, _ string, _ syscall.RawConn) error { return nil }
