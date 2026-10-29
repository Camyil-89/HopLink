//go:build windows

package desktop

import (
	"fmt"
	"os/exec"
	"syscall"
)

func startDetachedCommand(command *exec.Cmd) error {
	command.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
	if err := command.Start(); err != nil {
		return fmt.Errorf("не удалось запустить помощник обновления: %w", err)
	}
	_ = command.Process.Release()
	return nil
}
