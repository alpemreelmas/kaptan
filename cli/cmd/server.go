package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/alpemreelmas/kaptan/cli/client"
	"gopkg.in/yaml.v3"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Manage remote servers",
}

var serverAddCmd = &cobra.Command{
	Use:   "add <name> <host:port>",
	Short: "Add a server to ~/.kaptan/config.yaml",
	Args:  cobra.ExactArgs(2),
	RunE:  runServerAdd,
}

var serverBootstrapCmd = &cobra.Command{
	Use:   "bootstrap <name> <ssh-user@host>",
	Short: "Install kaptan-agent on a server via SSH",
	Args:  cobra.ExactArgs(2),
	RunE:  runServerBootstrap,
}

func init() {
	serverCmd.AddCommand(serverAddCmd)
	serverCmd.AddCommand(serverBootstrapCmd)
}

func runServerAdd(cmd *cobra.Command, args []string) error {
	name := args[0]
	host := args[1]

	global, err := client.LoadGlobalConfig()
	if err != nil {
		// create empty config if not found
		global = &client.GlobalConfig{}
	}

	for _, s := range global.Servers {
		if s.Name == name {
			return fmt.Errorf("server %q already exists", name)
		}
	}

	global.Servers = append(global.Servers, client.ServerEntry{
		Name: name,
		Host: host,
	})

	return saveGlobalConfig(global)
}

func runServerBootstrap(cmd *cobra.Command, args []string) error {
	name := args[0]
	sshTarget := args[1] // e.g. forge@1.2.3.4

	home, _ := os.UserHomeDir()
	caPath := filepath.Join(home, ".kaptan", "certs", "ca.crt")

	if _, err := os.Stat(caPath); err != nil {
		return fmt.Errorf("CA cert not found at %s — run 'm cert init' first", caPath)
	}

	fmt.Printf("Bootstrapping kaptan-agent on %s (%s)...\n", name, sshTarget)

	// install agent via remote install.sh
	installCmd := exec.Command("ssh", sshTarget,
		"curl -fsSL https://raw.githubusercontent.com/alpemreelmas/kaptan/main/install.sh | bash")
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("remote install failed: %w", err)
	}

	// copy CA cert to agent
	fmt.Println("Copying CA certificate...")
	scpCmd := exec.Command("scp", caPath, sshTarget+":~/.kaptan-agent/certs/ca.crt")
	scpCmd.Stdout = os.Stdout
	scpCmd.Stderr = os.Stderr
	if err := scpCmd.Run(); err != nil {
		return fmt.Errorf("scp CA cert failed: %w", err)
	}

	// restart agent
	restartCmd := exec.Command("ssh", sshTarget, "systemctl restart kaptan-agent")
	restartCmd.Stdout = os.Stdout
	restartCmd.Stderr = os.Stderr
	if err := restartCmd.Run(); err != nil {
		return fmt.Errorf("restart agent failed: %w", err)
	}

	fmt.Printf("Successfully bootstrapped %s\n", name)
	fmt.Println("Now add the server: m server add", name, "<host:7000>")
	return nil
}

func saveGlobalConfig(cfg *client.GlobalConfig) error {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".kaptan")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, data, 0600); err != nil {
		return err
	}
	fmt.Printf("Saved %s\n", path)
	return nil
}
