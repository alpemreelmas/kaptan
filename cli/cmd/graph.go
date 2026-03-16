package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/alpemreelmas/kaptan/cli/client"
	"github.com/alpemreelmas/kaptan/cli/tui"
	agentv1 "github.com/alpemreelmas/kaptan/proto/agent/v1"
)

var (
	graphServer  string
	graphLogFile string
)

var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Show dependency graph from server access logs",
	RunE:  runGraph,
}

func init() {
	graphCmd.Flags().StringVar(&graphServer, "server", "", "server name")
	graphCmd.Flags().StringVar(&graphLogFile, "log-file", "/var/log/nginx/access.log", "access log path on server")
}

func runGraph(cmd *cobra.Command, args []string) error {
	global, err := client.LoadGlobalConfig()
	if err != nil {
		return err
	}

	projCfg, err := client.LoadProjectConfig()
	if err != nil {
		return err
	}

	serverName := graphServer
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

	resp, err := agentClient.GetDependencyGraph(context.Background(), &agentv1.GraphRequest{
		LogFile: graphLogFile,
	})
	if err != nil {
		return err
	}

	return tui.RunGraph(srv.Name, resp.Edges)
}
