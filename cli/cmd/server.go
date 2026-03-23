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

var bootstrapInit bool

var serverBootstrapCmd = &cobra.Command{
	Use:   "bootstrap <name> <ssh-user@host>",
	Short: "Install reis on a server via SSH",
	Long: `Install the reis agent on a remote server via SSH.

If run from inside a kaptan project directory (with .kaptan/config.yaml),
use --init to also clone the project repo on the server so it is
ready for the first 'kaptan deploy'.`,
	Args: cobra.ExactArgs(2),
	RunE: runServerBootstrap,
}

func init() {
	serverBootstrapCmd.Flags().BoolVar(&bootstrapInit, "init", false,
		"Clone the project repo on the server after installing reis (reads .kaptan/config.yaml)")
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
	sshTarget := args[1] // e.g. root@1.2.3.4

	home, _ := os.UserHomeDir()
	caPath := filepath.Join(home, ".kaptan", "certs", "ca.crt")

	if _, err := os.Stat(caPath); err != nil {
		return fmt.Errorf("CA cert not found at %s — run 'kaptan cert init' first", caPath)
	}

	fmt.Printf("Bootstrapping reis on %s (%s)...\n", name, sshTarget)

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
	scpCmd := exec.Command("scp", caPath, sshTarget+":~/.reis/certs/ca.crt")
	scpCmd.Stdout = os.Stdout
	scpCmd.Stderr = os.Stderr
	if err := scpCmd.Run(); err != nil {
		return fmt.Errorf("scp CA cert failed: %w", err)
	}

	// restart agent
	restartCmd := exec.Command("ssh", sshTarget, "systemctl restart reis")
	restartCmd.Stdout = os.Stdout
	restartCmd.Stderr = os.Stderr
	if err := restartCmd.Run(); err != nil {
		return fmt.Errorf("restart agent failed: %w", err)
	}

	fmt.Printf("Successfully bootstrapped %s\n", name)

	// --init: clone project repo on the server
	if bootstrapInit {
		proj, err := client.LoadProjectConfig()
		if err != nil {
			return fmt.Errorf("--init requires a .kaptan/config.yaml in the current directory: %w", err)
		}
		if proj.Repo == "" {
			return fmt.Errorf("--init requires 'repo' field in .kaptan/config.yaml")
		}

		fmt.Printf("Initialising project %s at %s...\n", proj.Service, proj.Path)

		// Create the directory and clone (idempotent: skip if .git already exists)
		initScript := fmt.Sprintf(
			`set -e
mkdir -p %s
if [ ! -d %s/.git ]; then
  git clone --depth=1 %s %s
  echo "Cloned %s"
else
  echo "Repo already exists, skipping clone."
fi`,
			proj.Path, proj.Path, proj.Repo, proj.Path, proj.Repo,
		)

		cloneCmd := exec.Command("ssh", sshTarget, initScript)
		cloneCmd.Stdout = os.Stdout
		cloneCmd.Stderr = os.Stderr
		if err := cloneCmd.Run(); err != nil {
			return fmt.Errorf("project init failed: %w", err)
		}

		fmt.Printf("Project ready. Run 'kaptan deploy' to deploy.\n")
	} else {
		fmt.Println("Now add the server: kaptan server add", name, "<host:7000>")
	}

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
