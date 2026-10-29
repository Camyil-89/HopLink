//go:build !windows

package desktop

import (
	"fmt"
	"os/exec"
)

func startDetachedCommand(command *exec.Cmd) error {
	if err := command.Start(); err != nil {
		return fmt.Errorf("не удалось запустить помощник обновления: %w", err)
	}
	_ = command.Process.Release()
	return nil
}
