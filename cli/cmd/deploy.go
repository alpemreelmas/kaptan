package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/alpemreelmas/kaptan/cli/client"
	"github.com/alpemreelmas/kaptan/cli/tui"
	agentv1 "github.com/alpemreelmas/kaptan/proto/agent/v1"
)

var (
	deployServer string
	deployDryRun bool
	deployAll    bool
	deployTag    string
	deployNoTUI  bool
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy the current project to its configured server",
	RunE:  runDeploy,
}

func init() {
	deployCmd.Flags().StringVar(&deployServer, "server", "", "override server name from config")
	deployCmd.Flags().BoolVar(&deployDryRun, "dry-run", false, "print what would be done without executing")
	deployCmd.Flags().BoolVar(&deployAll, "all", false, "deploy to all servers matching --tag")
	deployCmd.Flags().StringVar(&deployTag, "tag", "", "filter servers by tag (used with --all)")
	deployCmd.Flags().BoolVar(&deployNoTUI, "no-tui", false, "plain text output without TUI")
}

func runDeploy(cmd *cobra.Command, args []string) error {
	global, err := client.LoadGlobalConfig()
	if err != nil {
		return err
	}

	if deployAll {
		return deployToTag(global)
	}

	projCfg, err := client.LoadProjectConfig()
	if err != nil {
		return err
	}

	serverName := deployServer
	if serverName == "" {
		serverName = projCfg.Server
	}

	srv, err := client.FindServer(global, serverName)
	if err != nil {
		return err
	}

	return deployToServer(srv, projCfg.Path, "deploy", deployDryRun)
}

func deployToTag(global *client.GlobalConfig) error {
	if deployTag == "" {
		return fmt.Errorf("--tag is required with --all")
	}
	servers := client.FindServersByTag(global, deployTag)
	if len(servers) == 0 {
		return fmt.Errorf("no servers found with tag %q", deployTag)
	}

	projCfg, err := client.LoadProjectConfig()
	if err != nil {
		return err
	}

	errCh := make(chan error, len(servers))
	for _, srv := range servers {
		go func(s *client.ServerEntry) {
			errCh <- deployToServer(s, projCfg.Path, "deploy", deployDryRun)
		}(srv)
	}

	var errs []error
	for range servers {
		if err := <-errCh; err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "error: %v\n", e)
		}
		return fmt.Errorf("%d deploy(s) failed", len(errs))
	}
	return nil
}

const deployMaxAttempts = 3

func deployToServer(srv *client.ServerEntry, projectPath, script string, dryRun bool) error {
	var lastErr error
	for attempt := 1; attempt <= deployMaxAttempts; attempt++ {
		if attempt > 1 {
			wait := time.Duration(attempt-1) * time.Second
			fmt.Fprintf(os.Stderr, "[%s] retrying in %s (attempt %d/%d)...\n", srv.Name, wait, attempt, deployMaxAttempts)
			time.Sleep(wait)
		}

		agentClient, conn, err := client.Connect(srv)
		if err != nil {
			lastErr = fmt.Errorf("[%s] connect: %w", srv.Name, err)
			continue
		}

		stream, err := agentClient.Deploy(context.Background(), &agentv1.DeployRequest{
			ProjectPath: projectPath,
			Script:      script,
			DryRun:      dryRun,
		})
		if err != nil {
			conn.Close()
			lastErr = fmt.Errorf("[%s] deploy RPC: %w", srv.Name, err)
			continue
		}

		// Stream started — script errors are not retried.
		var runErr error
		if deployNoTUI {
			runErr = streamToStdout(srv.Name, stream)
		} else {
			runErr = tui.RunDeploy(srv.Name, srv.Host, projectPath+"/"+script+".sh", stream)
		}
		conn.Close()
		return runErr
	}
	return lastErr
}

func streamToStdout(serverName string, stream agentv1.AgentService_DeployClient) error {
	for {
		ev, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("[%s] stream: %w", serverName, err)
		}
		if ev.IsStderr {
			fmt.Fprintf(os.Stderr, "[%s] %s\n", serverName, ev.Line)
		} else if ev.Line != "" {
			fmt.Printf("[%s] %s\n", serverName, ev.Line)
		}
		if ev.Done && ev.ExitCode != 0 {
			return fmt.Errorf("[%s] exited with code %d", serverName, ev.ExitCode)
		}
	}
	return nil
}
