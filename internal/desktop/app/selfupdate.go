package desktop

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const desktopBinaryLimit = 200 << 20

// InstallDesktopUpdate downloads and stages the current platform's executable,
// then replaces it after this process has closed.
func (a *App) InstallDesktopUpdate() error {
	update, err := a.CheckDesktopUpdate()
	if err != nil {
		return err
	}
	if !update.Available {
		return fmt.Errorf("актуальная версия уже установлена")
	}

	request, err := http.NewRequestWithContext(a.ctx, http.MethodGet, update.URL, nil)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", "HopLink-desktop")
	response, err := releaseClient.Do(request)
	if err != nil {
		return fmt.Errorf("скачивание обновления: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("скачивание обновления: HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, desktopBinaryLimit+1))
	if err != nil {
		return fmt.Errorf("чтение обновления: %w", err)
	}
	if len(data) > desktopBinaryLimit {
		return fmt.Errorf("файл обновления больше 200 МБ")
	}
	if err = validateDesktopBinary(data); err != nil {
		return err
	}
	if strings.HasPrefix(update.Digest, "sha256:") {
		digest := sha256.Sum256(data)
		if !strings.EqualFold(hex.EncodeToString(digest[:]), strings.TrimPrefix(update.Digest, "sha256:")) {
			return fmt.Errorf("контрольная сумма обновления не совпадает")
		}
	}

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("путь к приложению: %w", err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return fmt.Errorf("путь к приложению: %w", err)
	}
	appBundle := ""
	if runtime.GOOS == "darwin" {
		appBundle, err = findAppBundle(executable)
		if err != nil {
			return err
		}
	}

	info, err := os.Stat(executable)
	if err != nil {
		return fmt.Errorf("не удалось проверить исполняемый файл: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(executable), ".hoplink-update-*")
	if err != nil {
		return fmt.Errorf("папка приложения недоступна для обновления: %w", err)
	}
	stagedPath := temp.Name()
	if _, err = temp.Write(data); err == nil {
		err = temp.Sync()
	}
	if closeErr := temp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(stagedPath)
		return fmt.Errorf("не удалось подготовить новый исполняемый файл: %w", err)
	}
	if err = os.Chmod(stagedPath, info.Mode().Perm()|0o111); err != nil {
		_ = os.Remove(stagedPath)
		return fmt.Errorf("не удалось разрешить запуск обновления: %w", err)
	}

	if err = startUpdateWorker(stagedPath, executable, appBundle); err != nil {
		_ = os.Remove(stagedPath)
		return err
	}
	a.log("Обновление desktop v" + update.Latest + " подготовлено; приложение перезапустится")
	go func() {
		time.Sleep(800 * time.Millisecond)
		wailsruntime.Quit(a.ctx)
	}()
	return nil
}

func findAppBundle(executable string) (string, error) {
	for current := filepath.Dir(executable); ; current = filepath.Dir(current) {
		if strings.EqualFold(filepath.Ext(current), ".app") {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("не удалось найти .app bundle для автоматического обновления")
		}
	}
}

func validateDesktopBinary(data []byte) error {
	if len(data) < 4 {
		return fmt.Errorf("файл обновления слишком короткий")
	}
	valid := false
	switch runtime.GOOS {
	case "windows":
		valid = data[0] == 'M' && data[1] == 'Z'
	case "linux":
		valid = data[0] == 0x7f && string(data[1:4]) == "ELF"
	case "darwin":
		magic := string(data[:4])
		valid = magic == string([]byte{0xcf, 0xfa, 0xed, 0xfe}) ||
			magic == string([]byte{0xfe, 0xed, 0xfa, 0xcf}) ||
			magic == string([]byte{0xca, 0xfe, 0xba, 0xbe}) ||
			magic == string([]byte{0xbe, 0xba, 0xfe, 0xca})
	}
	if !valid {
		return fmt.Errorf("загруженный файл не является исполняемым файлом для %s", runtime.GOOS)
	}
	return nil
}

func startUpdateWorker(stagedPath, targetPath, appBundle string) error {
	if runtime.GOOS == "windows" {
		return startWindowsUpdateWorker(stagedPath, targetPath)
	}
	return startUnixUpdateWorker(stagedPath, targetPath, appBundle)
}

func startWindowsUpdateWorker(stagedPath, targetPath string) error {
	scriptFile, err := os.CreateTemp("", "hoplink-update-*.ps1")
	if err != nil {
		return fmt.Errorf("не удалось создать помощник обновления: %w", err)
	}
	scriptPath := scriptFile.Name()
	script := `param([int]$ParentPid, [string]$StagedPath, [string]$TargetPath, [string]$ScriptPath, [string]$LogPath)
$ErrorActionPreference = 'Stop'
$deadline = [DateTime]::UtcNow.AddSeconds(90)
try {
  while (Get-Process -Id $ParentPid -ErrorAction SilentlyContinue) {
    if ([DateTime]::UtcNow -gt $deadline) { throw 'HopLink did not exit in time' }
    Start-Sleep -Milliseconds 250
  }
  Move-Item -LiteralPath $StagedPath -Destination $TargetPath -Force
  Start-Process -FilePath $TargetPath -WorkingDirectory (Split-Path -Parent $TargetPath)
} catch {
  [System.IO.File]::AppendAllText($LogPath, "$(Get-Date -Format o) $($_.Exception.Message)" + [Environment]::NewLine)
  if (Test-Path -LiteralPath $TargetPath) {
    try { Start-Process -FilePath $TargetPath -WorkingDirectory (Split-Path -Parent $TargetPath) } catch {}
  }
  exit 1
} finally {
  Remove-Item -LiteralPath $ScriptPath -Force -ErrorAction SilentlyContinue
}`
	if _, err = scriptFile.WriteString(script); err == nil {
		err = scriptFile.Close()
	} else {
		_ = scriptFile.Close()
	}
	if err != nil {
		_ = os.Remove(scriptPath)
		return err
	}
	command := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-ExecutionPolicy", "Bypass", "-File", scriptPath,
		strconv.Itoa(os.Getpid()), stagedPath, targetPath, scriptPath, filepath.Join(os.TempDir(), "HopLink-update.log"))
	if err := startDetachedCommand(command); err != nil {
		_ = os.Remove(scriptPath)
		return err
	}
	return nil
}

func startUnixUpdateWorker(stagedPath, targetPath, appBundle string) error {
	scriptFile, err := os.CreateTemp("", "hoplink-update-*.sh")
	if err != nil {
		return fmt.Errorf("не удалось создать помощник обновления: %w", err)
	}
	scriptPath := scriptFile.Name()
	var script string
	var args []string
	if runtime.GOOS == "darwin" {
		script = `#!/bin/sh
set -eu
parent_pid=$1; staged_path=$2; target_path=$3; app_bundle=$4; script_path=$5
while kill -0 "$parent_pid" 2>/dev/null; do sleep 0.25; done
mv -f "$staged_path" "$target_path"
chmod 755 "$target_path"
codesign --force --deep --sign - "$app_bundle"
open -a "$app_bundle"
rm -f "$script_path"
`
		args = []string{strconv.Itoa(os.Getpid()), stagedPath, targetPath, appBundle, scriptPath}
	} else {
		script = `#!/bin/sh
set -eu
parent_pid=$1; staged_path=$2; target_path=$3; script_path=$4
while kill -0 "$parent_pid" 2>/dev/null; do sleep 0.25; done
mv -f "$staged_path" "$target_path"
chmod 755 "$target_path"
nohup "$target_path" >/dev/null 2>&1 </dev/null &
rm -f "$script_path"
`
		args = []string{strconv.Itoa(os.Getpid()), stagedPath, targetPath, scriptPath}
	}
	if _, err = scriptFile.WriteString(script); err == nil {
		err = scriptFile.Close()
	} else {
		_ = scriptFile.Close()
	}
	if err != nil {
		_ = os.Remove(scriptPath)
		return err
	}
	if err = os.Chmod(scriptPath, 0o700); err != nil {
		_ = os.Remove(scriptPath)
		return err
	}
	command := exec.Command("sh", append([]string{scriptPath}, args...)...)
	if err := startDetachedCommand(command); err != nil {
		_ = os.Remove(scriptPath)
		return err
	}
	return nil
}
