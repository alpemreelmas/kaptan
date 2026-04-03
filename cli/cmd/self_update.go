package cmd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/alpemreelmas/kaptan/cli/internal/version"
	"github.com/spf13/cobra"
)

const (
	repo    = "alpemreelmas/kaptan"
	baseURL = "https://github.com/" + repo + "/releases/download"
	apiURL  = "https://api.github.com/repos/" + repo + "/releases/latest"
)

var selfUpdateCmd = &cobra.Command{
	Use:   "self-update",
	Short: "Update kaptan to the latest version",
	Long:  "Downloads and installs the latest release of kaptan from GitHub.",
	RunE:  runSelfUpdate,
}

func runSelfUpdate(cmd *cobra.Command, args []string) error {
	current := version.Version
	fmt.Printf("Current version: %s\n", current)

	latest, err := fetchLatestVersion()
	if err != nil {
		return fmt.Errorf("failed to fetch latest version: %w", err)
	}

	if current == latest {
		fmt.Println("Already on the latest version")
		return nil
	}

	fmt.Printf("Updating from %s to %s...\n", current, latest)

	checksums, err := fetchChecksums(latest)
	if err != nil {
		return fmt.Errorf("failed to fetch checksums: %w", err)
	}

	if err := downloadAndInstall(latest, checksums); err != nil {
		return fmt.Errorf("failed to update: %w", err)
	}

	fmt.Printf("Successfully updated to %s\n", latest)
	return nil
}

func fetchLatestVersion() (string, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, apiURL, nil)
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

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	return strings.TrimPrefix(release.TagName, "v"), nil
}

func fetchChecksums(version string) (map[string]string, error) {
	checksumURL := fmt.Sprintf("%s/v%s/checksums.txt", baseURL, version)
	resp, err := http.DefaultClient.Get(checksumURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("checksums not found")
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	checksums := make(map[string]string)
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) == 2 {
			checksums[parts[1]] = parts[0]
		}
	}

	return checksums, nil
}

func downloadAndInstall(version string, checksums map[string]string) error {
	osName := runtime.GOOS
	arch := runtime.GOARCH
	if arch == "arm64" {
		arch = "arm64"
	}

	binaryName := fmt.Sprintf("kaptan_%s_%s_%s", version, osName, arch)
	downloadURL := fmt.Sprintf("%s/v%s/%s", baseURL, version, binaryName)

	checksum, ok := checksums[binaryName]
	if !ok {
		return fmt.Errorf("checksum not found for %s", binaryName)
	}

	tmpDir, err := os.MkdirTemp("", "kaptan-update")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	tmpPath := filepath.Join(tmpDir, "kaptan")

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, downloadURL, nil)
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

	if err := os.WriteFile(tmpPath, body, 0755); err != nil {
		return err
	}

	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	if err := os.Rename(tmpPath, execPath); err != nil {
		return fmt.Errorf("failed to replace binary: %w (try running with sudo)", err)
	}

	return nil
}

func restartSelf() error {
	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	execCmd := exec.Command(execPath, os.Args[1:]...)
	execCmd.Stdin = os.Stdin
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	execCmd.Run()

	return nil
}
