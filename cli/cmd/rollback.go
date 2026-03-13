package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/yourusername/kaptan/cli/client"
	agentv1 "github.com/yourusername/kaptan/proto/agent/v1"
)

var rollbackServer string

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Roll back the current project to the previous release",
	RunE:  runRollback,
}

func init() {
	rollbackCmd.Flags().StringVar(&rollbackServer, "server", "", "override server name from config")
}

func runRollback(cmd *cobra.Command, args []string) error {
	global, err := client.LoadGlobalConfig()
	if err != nil {
		return err
	}

	projCfg, err := client.LoadProjectConfig()
	if err != nil {
		return err
	}

	serverName := rollbackServer
	if serverName == "" {
		serverName = projCfg.Server
	}

	srv, err := client.FindServer(global, serverName)
	if err != nil {
		return err
	}

	agentClient, conn, err := client.Connect(srv)
	if err != nil {
		return err
	}
	defer conn.Close()

	stream, err := agentClient.Rollback(context.Background(), &agentv1.RollbackRequest{
		ProjectPath: projCfg.Path,
	})
	if err != nil {
		return fmt.Errorf("rollback RPC: %w", err)
	}

	for {
		ev, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("stream: %w", err)
		}
		if ev.IsStderr {
			fmt.Fprintf(os.Stderr, "%s\n", ev.Line)
		} else if ev.Line != "" {
			fmt.Println(ev.Line)
		}
		if ev.Done && ev.ExitCode != 0 {
			return fmt.Errorf("rollback exited with code %d", ev.ExitCode)
		}
	}
	return nil
}
