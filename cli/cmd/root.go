package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "m",
	Short: "kaptan — centralised VPS microservice manager",
	Long: `kaptan manages microservice deployments on Forge-managed VPS instances
via mTLS/gRPC. Run 'm deploy' from a project with a .kaptan/ directory.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(rollbackCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(graphCmd)
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(certCmd)
}
