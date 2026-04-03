package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/alpemreelmas/kaptan/agent/internal/version"
)

const (
	repo   = "alpemreelmas/kaptan"
	apiURL = "https://api.github.com/repos/" + repo + "/releases/latest"
)

type release struct {
	TagName string `json:"tag_name"`
}

func Start(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	check(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			check(ctx)
		}
	}
}

func check(ctx context.Context) {
	latest, err := fetchLatestVersion(ctx)
	if err != nil {
		slog.Debug("failed to check for updates", "err", err)
		return
	}

	current := version.Version
	if current == "dev" {
		return
	}

	if latest == "" || latest == current {
		return
	}

	slog.Info("update available", "current", current, "latest", latest)

	newVersion := strings.TrimPrefix(latest, "v")
	if err := install(ctx, newVersion); err != nil {
		slog.Error("failed to install update", "version", newVersion, "err", err)
		return
	}

	slog.Info("update installed, restarting", "version", newVersion)
	restart()
}

func fetchLatestVersion(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var r release
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", err
	}

	return r.TagName, nil
}

func install(ctx context.Context, version string) error {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	binaryName := fmt.Sprintf("reis_%s_%s_%s", version, osName, arch)
	baseURL := fmt.Sprintf("https://github.com/%s/releases/download/v%s", repo, version)
	checksumURL := fmt.Sprintf("%s/checksums.txt", baseURL)
	downloadURL := fmt.Sprintf("%s/%s", baseURL, binaryName)

	checksum, err := fetchChecksum(ctx, checksumURL, binaryName)
	if err != nil {
		return fmt.Errorf("fetch checksum: %w", err)
	}

	checksum = strings.TrimSpace(checksum)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	hash := sha256.Sum256(body)
	if hex.EncodeToString(hash[:]) != checksum {
		return fmt.Errorf("checksum mismatch")
	}

	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	tmpPath := execPath + ".new"
	if err := os.WriteFile(tmpPath, body, 0755); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, execPath); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return nil
}

func fetchChecksum(ctx context.Context, url, filename string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("checksum not found")
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == filename {
			return parts[0], nil
		}
	}

	return "", fmt.Errorf("checksum not found for %s", filename)
}

func restart() {
	if err := exec.Command("systemctl", "restart", "reis").Run(); err != nil {
		slog.Debug("systemctl restart failed", "err", err)
	}
	slog.Info("restart signal sent")
}
