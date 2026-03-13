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

var (
	logsServer  string
	logsTail    int32
	logsLogFile string
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Stream logs from a remote service",
	RunE:  runLogs,
}

func init() {
	logsCmd.Flags().StringVar(&logsServer, "server", "", "server name")
	logsCmd.Flags().Int32Var(&logsTail, "tail", 50, "number of lines to show from end")
	logsCmd.Flags().StringVar(&logsLogFile, "file", "", "explicit log file path on server")
}

func runLogs(cmd *cobra.Command, args []string) error {
	global, err := client.LoadGlobalConfig()
	if err != nil {
		return err
	}

	projCfg, err := client.LoadProjectConfig()
	if err != nil {
		return err
	}

	serverName := logsServer
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

	stream, err := agentClient.StreamLogs(context.Background(), &agentv1.LogRequest{
		ProjectPath: projCfg.Path,
		LogFile:     logsLogFile,
		Tail:        logsTail,
	})
	if err != nil {
		return fmt.Errorf("logs RPC: %w", err)
	}

	for {
		line, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("stream: %w", err)
		}
		fmt.Fprintln(os.Stdout, line.Content)
	}
	return nil
}
