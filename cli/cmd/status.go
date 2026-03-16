package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/alpemreelmas/kaptan/cli/client"
	"github.com/alpemreelmas/kaptan/cli/tui"
	agentv1 "github.com/alpemreelmas/kaptan/proto/agent/v1"
)

var statusTag string

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show health status of all services",
	RunE:  runStatus,
}

func init() {
	statusCmd.Flags().StringVar(&statusTag, "tag", "", "filter servers by tag")
}

func runStatus(cmd *cobra.Command, args []string) error {
	global, err := client.LoadGlobalConfig()
	if err != nil {
		return err
	}

	servers := global.Servers
	if statusTag != "" {
		tagged := client.FindServersByTag(global, statusTag)
		servers = make([]client.ServerEntry, len(tagged))
		for i, s := range tagged {
			servers[i] = *s
		}
	}

	type serverResult struct {
		server   string
		services []*agentv1.ServiceStatus
		err      error
	}

	results := make(chan serverResult, len(servers))

	for _, srv := range servers {
		go func(s client.ServerEntry) {
			agentClient, conn, err := client.Connect(&s)
			if err != nil {
				results <- serverResult{server: s.Name, err: err}
				return
			}
			defer conn.Close()

			resp, err := agentClient.GetStatus(context.Background(), &agentv1.StatusRequest{})
			if err != nil {
				results <- serverResult{server: s.Name, err: err}
				return
			}
			results <- serverResult{server: s.Name, services: resp.Services}
		}(srv)
	}

	var rows []tui.StatusRow
	for range servers {
		r := <-results
		if r.err != nil {
			fmt.Fprintf(os.Stderr, "warning: %s: %v\n", r.server, r.err)
			continue
		}
		for _, svc := range r.services {
			rows = append(rows, tui.StatusRow{
				Server:     r.server,
				Service:    svc.ServiceName,
				Healthy:    svc.Healthy,
				StatusCode: int(svc.StatusCode),
			})
		}
	}

	tui.RenderStatus(rows)
	return nil
}
