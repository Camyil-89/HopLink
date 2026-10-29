package desktop

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// serverSourceRoot requires the desktop executable to live in deploy/, one
// directory below the repository root containing the service sources.
func serverSourceRoot(executable string) (string, error) {
	root := filepath.Dir(filepath.Dir(executable))
	for _, name := range []string{"go.mod", filepath.Join("cmd", "hoplink", "main.go")} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			return "", fmt.Errorf("исходники HopLink не найдены в %s: нужен %s рядом с каталогом приложения", root, name)
		}
	}
	return root, nil
}

// buildServerBinary compiles the service from current sources each time Apply
// runs. Removing the prior output first prevents a failed build from reusing it.
func (a *App) buildServerBinary(ctx context.Context, arch string) (string, error) {
	if arch != "amd64" && arch != "arm64" {
		return "", fmt.Errorf("неподдерживаемая архитектура Linux: %s", arch)
	}
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("путь к приложению: %w", err)
	}
	root, err := serverSourceRoot(executable)
	if err != nil {
		return "", err
	}
	goBinary, err := exec.LookPath("go")
	if err != nil {
		return "", fmt.Errorf("для сборки Linux-бинарника установите Go и добавьте go в PATH: %w", err)
	}
	out := filepath.Join(filepath.Dir(executable), "HopLink-linux-"+arch)
	if err := os.Remove(out); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("не удалось удалить предыдущий Linux-бинарник %s: %w", out, err)
	}
	a.log(fmt.Sprintf("Собираю Linux/%s из %s → %s", arch, root, out))
	command := exec.CommandContext(ctx, goBinary, "build", "-trimpath", "-o", out, "./cmd/hoplink")
	command.Dir = root
	command.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+arch, "CGO_ENABLED=0")
	output, err := command.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("сборка HopLink для linux/%s: %w: %s", arch, err, strings.TrimSpace(string(output)))
	}
	data, err := os.ReadFile(out)
	if err != nil || len(data) == 0 {
		return "", fmt.Errorf("сборка linux/%s не создала бинарник %s: %v", arch, out, err)
	}
	digest := sha256.Sum256(data)
	a.log(fmt.Sprintf("Linux/%s собран: %d байт, SHA256 %x", arch, len(data), digest[:]))
	return out, nil
}
