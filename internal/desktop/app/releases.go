package desktop

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const releaseAPI = "https://api.github.com/repos/Camyil-89/HopLink/releases?per_page=100"

var DesktopVersion = "dev"
var releaseClient = &http.Client{Timeout: 90 * time.Second}

func (a *App) GetDesktopVersion() string { return DesktopVersion }

type releaseAsset struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Digest string `json:"digest"`
}

type githubRelease struct {
	Tag        string         `json:"tag_name"`
	URL        string         `json:"html_url"`
	Draft      bool           `json:"draft"`
	Prerelease bool           `json:"prerelease"`
	Assets     []releaseAsset `json:"assets"`
}

type DesktopUpdate struct {
	Installed string `json:"installed"`
	Latest    string `json:"latest"`
	Available bool   `json:"available"`
	URL       string `json:"url"`
}

func fetchReleases(ctx context.Context) ([]githubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseAPI, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "HopLink-desktop")
	resp, err := releaseClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub releases: HTTP %d", resp.StatusCode)
	}
	var releases []githubRelease
	err = json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&releases)
	return releases, err
}

func parseVersion(value string) ([3]int, bool) {
	var parts [3]int
	if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(value) {
		return parts, false
	}
	for i, part := range strings.Split(value, ".") {
		n, err := strconv.Atoi(part)
		if err != nil {
			return parts, false
		}
		parts[i] = n
	}
	return parts, true
}

func newerVersion(a, b string) bool {
	left, ok := parseVersion(a)
	if !ok {
		return false
	}
	right, ok := parseVersion(b)
	if !ok {
		return true
	}
	for i := range left {
		if left[i] != right[i] {
			return left[i] > right[i]
		}
	}
	return false
}

func latestComponent(releases []githubRelease, component string) (githubRelease, string, error) {
	var selected githubRelease
	version := ""
	for _, item := range releases {
		if item.Draft || item.Prerelease {
			continue
		}
		prefix := component + "-v"
		if component == "desktop-hoplink" && strings.HasPrefix(item.Tag, "hoplink-v") {
			// Unified releases use the hoplink-v tag for both applications.
			prefix = "hoplink-v"
		}
		if !strings.HasPrefix(item.Tag, prefix) {
			continue
		}
		candidate := strings.TrimPrefix(item.Tag, prefix)
		if newerVersion(candidate, version) {
			selected, version = item, candidate
		}
	}
	if version == "" {
		return selected, "", fmt.Errorf("релиз %s ещё не опубликован", component)
	}
	return selected, version, nil
}

func findAsset(release githubRelease, name string) (releaseAsset, error) {
	for _, asset := range release.Assets {
		if asset.Name == name {
			return asset, nil
		}
	}
	return releaseAsset{}, fmt.Errorf("в релизе %s нет файла %s", release.Tag, name)
}

func (a *App) CheckDesktopUpdate() (DesktopUpdate, error) {
	result := DesktopUpdate{Installed: DesktopVersion}
	releases, err := fetchReleases(a.ctx)
	if err != nil {
		return result, err
	}
	release, version, err := latestComponent(releases, "desktop-hoplink")
	if err != nil {
		return result, err
	}
	platform := runtime.GOOS
	asset, err := findAsset(release, fmt.Sprintf("desktop-hoplink-%s.zip", platform))
	if err != nil {
		return result, err
	}
	result.Latest = version
	result.URL = asset.URL
	result.Available = DesktopVersion == "dev" || newerVersion(version, DesktopVersion)
	return result, nil
}

// OpenDesktopUpdate downloads the matching release archive in the user's browser.
// The running executable and encrypted settings are left in place.
func (a *App) OpenDesktopUpdate() error {
	update, err := a.CheckDesktopUpdate()
	if err != nil {
		return err
	}
	if !update.Available {
		return fmt.Errorf("актуальная версия уже установлена")
	}
	wailsruntime.BrowserOpenURL(a.ctx, update.URL)
	return nil
}

func (a *App) SetInstallSource(source string) error {
	if source != "source" && source != "github" {
		return fmt.Errorf("неизвестный способ установки")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	previous := a.config.InstallSource
	a.config.InstallSource = source
	if err := a.saveLocked(); err != nil {
		a.config.InstallSource = previous
		return err
	}
	return nil
}

func (a *App) downloadServerBinary(ctx context.Context, arch string) (string, error) {
	if arch != "amd64" && arch != "arm64" {
		return "", fmt.Errorf("неподдерживаемая архитектура Linux: %s", arch)
	}
	releases, err := fetchReleases(ctx)
	if err != nil {
		return "", err
	}
	release, version, err := latestComponent(releases, "hoplink")
	if err != nil {
		return "", err
	}
	asset, err := findAsset(release, "hoplink-linux.zip")
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "HopLink-desktop")
	resp, err := releaseClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("скачивание HopLink: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 150<<20))
	if err != nil {
		return "", err
	}
	if len(data) >= 150<<20 {
		return "", fmt.Errorf("архив HopLink слишком большой")
	}
	if strings.HasPrefix(asset.Digest, "sha256:") {
		digest := sha256.Sum256(data)
		if !strings.EqualFold(hex.EncodeToString(digest[:]), strings.TrimPrefix(asset.Digest, "sha256:")) {
			return "", fmt.Errorf("контрольная сумма архива HopLink не совпадает")
		}
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	name := "hoplink-Linux-" + arch
	for _, file := range archive.File {
		if file.Name != name {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			return "", err
		}
		defer reader.Close()
		out, err := os.CreateTemp("", "hoplink-linux-"+arch+"-*")
		if err != nil {
			return "", err
		}
		defer out.Close()
		if _, err = io.Copy(out, io.LimitReader(reader, 100<<20)); err != nil {
			os.Remove(out.Name())
			return "", err
		}
		if err = out.Chmod(0700); err != nil {
			os.Remove(out.Name())
			return "", err
		}
		a.log("Скачан HopLink v" + version + " для Linux/" + arch)
		return out.Name(), nil
	}
	return "", fmt.Errorf("архив %s не содержит %s", asset.Name, name)
}
