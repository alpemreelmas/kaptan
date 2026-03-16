package server

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"time"

	agentv1 "github.com/alpemreelmas/kaptan/proto/agent/v1"
	"github.com/alpemreelmas/kaptan/agent/executor"
	"github.com/alpemreelmas/kaptan/agent/health"
	"github.com/alpemreelmas/kaptan/agent/graph"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"gopkg.in/yaml.v3"
)

type projectConfig struct {
	Service   string `yaml:"service"`
	Server    string `yaml:"server"`
	Path      string `yaml:"path"`
	HealthURL string `yaml:"health_url"`
}

type agentServer struct {
	agentv1.UnimplementedAgentServiceServer
}

func Run(addr, certFile, keyFile, caFile string) error {
	var opts []grpc.ServerOption

	if certFile != "" && keyFile != "" && caFile != "" {
		creds, err := buildTLSCredentials(certFile, keyFile, caFile)
		if err != nil {
			return fmt.Errorf("build TLS: %w", err)
		}
		opts = append(opts, grpc.Creds(creds))
		slog.Info("mTLS enabled")
	} else {
		slog.Warn("running without TLS — for development only")
	}

	srv := grpc.NewServer(opts...)
	agentv1.RegisterAgentServiceServer(srv, &agentServer{})

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	slog.Info("listening", "addr", addr)
	return srv.Serve(lis)
}

func buildTLSCredentials(certFile, keyFile, caFile string) (credentials.TransportCredentials, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	caCert, err := os.ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA cert")
	}

	cfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caPool,
	}
	return credentials.NewTLS(cfg), nil
}

func (s *agentServer) Deploy(req *agentv1.DeployRequest, stream agentv1.AgentService_DeployServer) error {
	scriptName := req.Script
	if scriptName == "" {
		scriptName = "deploy"
	}
	scriptPath := filepath.Join(req.ProjectPath, ".kaptan", scriptName+".sh")

	if req.DryRun {
		slog.Info("deploy dry-run", "script", scriptPath)
		_ = stream.Send(&agentv1.ExecEvent{Line: fmt.Sprintf("[dry-run] would execute: %s", scriptPath)})
		_ = stream.Send(&agentv1.ExecEvent{Line: "[dry-run] done", Done: true, ExitCode: 0})
		return nil
	}

	slog.Info("deploy started", "path", req.ProjectPath, "script", scriptPath)
	exitCode, err := executor.RunScript(scriptPath, stream.Context(), func(line string, isErr bool) error {
		return stream.Send(&agentv1.ExecEvent{Line: line, IsStderr: isErr})
	})

	if err != nil {
		return status.Errorf(codes.Internal, "executor: %v", err)
	}

	slog.Info("deploy finished", "path", req.ProjectPath, "exit_code", exitCode)
	if sendErr := stream.Send(&agentv1.ExecEvent{
		Done:     true,
		ExitCode: int32(exitCode),
	}); sendErr != nil {
		return sendErr
	}

	// if deploy succeeded, check health
	if exitCode == 0 {
		cfg, _ := loadProjectConfig(req.ProjectPath)
		if cfg != nil && cfg.HealthURL != "" {
			ok, code, _ := health.Check(cfg.HealthURL, 30*time.Second)
			if !ok {
				slog.Warn("health check failed, triggering rollback", "url", cfg.HealthURL, "status_code", code)
				_ = stream.Send(&agentv1.ExecEvent{
					Line: fmt.Sprintf("[health] %s returned %d — triggering rollback", cfg.HealthURL, code),
				})
				s.runRollback(req.ProjectPath, stream.Context(), func(line string, isErr bool) error {
					return stream.Send(&agentv1.ExecEvent{Line: "[rollback] " + line, IsStderr: isErr})
				})
				return status.Errorf(codes.Internal, "health check failed (%d), rolled back", code)
			}
			slog.Info("health check passed", "url", cfg.HealthURL, "status_code", code)
			_ = stream.Send(&agentv1.ExecEvent{
				Line: fmt.Sprintf("[health] %s → %d OK", cfg.HealthURL, code),
			})
		}
	}

	return nil
}

func (s *agentServer) Rollback(req *agentv1.RollbackRequest, stream agentv1.AgentService_RollbackServer) error {
	exitCode, err := s.runRollback(req.ProjectPath, stream.Context(), func(line string, isErr bool) error {
		return stream.Send(&agentv1.ExecEvent{Line: line, IsStderr: isErr})
	})
	if err != nil {
		return status.Errorf(codes.Internal, "rollback: %v", err)
	}
	return stream.Send(&agentv1.ExecEvent{Done: true, ExitCode: int32(exitCode)})
}

func (s *agentServer) runRollback(projectPath string, ctx context.Context, emit func(string, bool) error) (int, error) {
	scriptPath := filepath.Join(projectPath, ".kaptan", "rollback.sh")
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		_ = emit("no rollback.sh found", false)
		return 1, nil
	}
	return executor.RunScript(scriptPath, ctx, emit)
}

func (s *agentServer) HealthCheck(ctx context.Context, req *agentv1.HealthRequest) (*agentv1.HealthResponse, error) {
	ok, code, msg := health.Check(req.Url, 10*time.Second)
	return &agentv1.HealthResponse{
		Ok:         ok,
		StatusCode: int32(code),
		Message:    msg,
	}, nil
}

func (s *agentServer) GetStatus(ctx context.Context, req *agentv1.StatusRequest) (*agentv1.StatusResponse, error) {
	var services []*agentv1.ServiceStatus
	for _, path := range req.ProjectPaths {
		cfg, err := loadProjectConfig(path)
		if err != nil {
			services = append(services, &agentv1.ServiceStatus{
				ProjectPath: path,
				ServiceName: filepath.Base(path),
				Healthy:     false,
			})
			continue
		}
		ok, code, _ := health.Check(cfg.HealthURL, 5*time.Second)
		services = append(services, &agentv1.ServiceStatus{
			ProjectPath: path,
			ServiceName: cfg.Service,
			Healthy:     ok,
			StatusCode:  int32(code),
			HealthUrl:   cfg.HealthURL,
		})
	}
	return &agentv1.StatusResponse{Services: services}, nil
}

func (s *agentServer) StreamLogs(req *agentv1.LogRequest, stream agentv1.AgentService_StreamLogsServer) error {
	logFile := req.LogFile
	if logFile == "" {
		// try to find a common log location
		candidates := []string{
			filepath.Join(req.ProjectPath, "storage", "logs", "laravel.log"),
			filepath.Join(req.ProjectPath, "logs", "app.log"),
			"/var/log/nginx/access.log",
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				logFile = c
				break
			}
		}
	}
	if logFile == "" {
		return status.Error(codes.NotFound, "no log file found")
	}

	f, err := os.Open(logFile)
	if err != nil {
		return status.Errorf(codes.Internal, "open log: %v", err)
	}
	defer f.Close()

	// seek to tail position if requested
	if req.Tail > 0 {
		seekToTail(f, int(req.Tail))
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if err := stream.Context().Err(); err != nil {
			return nil
		}
		if err := stream.Send(&agentv1.LogLine{
			Content:   scanner.Text(),
			Timestamp: time.Now().UnixMilli(),
		}); err != nil {
			return err
		}
	}

	// tail -f behaviour
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case <-time.After(500 * time.Millisecond):
		}
		for scanner.Scan() {
			if err := stream.Send(&agentv1.LogLine{
				Content:   scanner.Text(),
				Timestamp: time.Now().UnixMilli(),
			}); err != nil {
				return err
			}
		}
	}
}

func (s *agentServer) GetDependencyGraph(ctx context.Context, req *agentv1.GraphRequest) (*agentv1.GraphResponse, error) {
	edges, err := graph.ParseNginxLog(req.LogFile)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "parse log: %v", err)
	}
	var protoEdges []*agentv1.GraphEdge
	for _, e := range edges {
		protoEdges = append(protoEdges, &agentv1.GraphEdge{
			From:       e.From,
			To:         e.To,
			StatusCode: int32(e.StatusCode),
			ErrorCount: int32(e.ErrorCount),
		})
	}
	return &agentv1.GraphResponse{Edges: protoEdges}, nil
}

func loadProjectConfig(projectPath string) (*projectConfig, error) {
	data, err := os.ReadFile(filepath.Join(projectPath, ".kaptan", "config.yaml"))
	if err != nil {
		return nil, err
	}
	var cfg projectConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func seekToTail(f *os.File, lines int) {
	const chunkSize = 4096
	stat, err := f.Stat()
	if err != nil {
		return
	}
	size := stat.Size()
	if size == 0 {
		return
	}

	var offset int64
	found := 0
	buf := make([]byte, chunkSize)

	for offset < size {
		readAt := size - offset - int64(chunkSize)
		if readAt < 0 {
			readAt = 0
		}
		n, _ := f.ReadAt(buf, readAt)
		chunk := buf[:n]
		for i := n - 1; i >= 0; i-- {
			if chunk[i] == '\n' {
				found++
				if found > lines {
					_, _ = f.Seek(readAt+int64(i)+1, io.SeekStart)
					return
				}
			}
		}
		if readAt == 0 {
			break
		}
		offset += int64(chunkSize)
	}
	_, _ = f.Seek(0, io.SeekStart)
}

