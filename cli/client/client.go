package client

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	agentv1 "github.com/alpemreelmas/kaptan/proto/agent/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/yaml.v3"
)

// GlobalConfig is the ~/.kaptan/config.yaml structure.
type GlobalConfig struct {
	Servers []ServerEntry `yaml:"servers"`
	Graph   GraphConfig   `yaml:"graph"`
}

// GraphConfig holds dependency graph settings.
type GraphConfig struct {
	InternalDomains []string `yaml:"internal_domains"`
}

type ServerEntry struct {
	Name string   `yaml:"name"`
	Host string   `yaml:"host"`
	Tags []string `yaml:"tags"`
	TLS  struct {
		Cert string `yaml:"cert"`
		Key  string `yaml:"key"`
		CA   string `yaml:"ca"`
	} `yaml:"tls"`
}

// ProjectConfig is .kaptan/config.yaml in a project repo.
type ProjectConfig struct {
	Service   string `yaml:"service"`
	Server    string `yaml:"server"`
	Path      string `yaml:"path"`
	HealthURL string `yaml:"health_url"`
}

// LoadGlobalConfig reads ~/.kaptan/config.yaml.
func LoadGlobalConfig() (*GlobalConfig, error) {
	home, _ := os.UserHomeDir()
	data, err := os.ReadFile(filepath.Join(home, ".kaptan", "config.yaml"))
	if err != nil {
		return nil, fmt.Errorf("read global config: %w", err)
	}
	var cfg GlobalConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse global config: %w", err)
	}
	return &cfg, nil
}

// LoadProjectConfig reads .kaptan/config.yaml from cwd.
func LoadProjectConfig() (*ProjectConfig, error) {
	data, err := os.ReadFile(".kaptan/config.yaml")
	if err != nil {
		return nil, fmt.Errorf(".kaptan/config.yaml not found — is this a kaptan project?\n  Run: mkdir .kaptan && touch .kaptan/config.yaml .kaptan/deploy.sh")
	}
	var cfg ProjectConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse project config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Validate checks that all required fields are present in the project config.
func (c *ProjectConfig) Validate() error {
	var missing []string
	if c.Service == "" {
		missing = append(missing, "service")
	}
	if c.Server == "" {
		missing = append(missing, "server")
	}
	if c.Path == "" {
		missing = append(missing, "path")
	}
	if len(missing) > 0 {
		return fmt.Errorf(".kaptan/config.yaml is missing required fields: %s", strings.Join(missing, ", "))
	}
	return nil
}

// FindServer returns the ServerEntry for the given name.
func FindServer(global *GlobalConfig, name string) (*ServerEntry, error) {
	for i := range global.Servers {
		if global.Servers[i].Name == name {
			return &global.Servers[i], nil
		}
	}
	return nil, fmt.Errorf("server %q not found in ~/.kaptan/config.yaml", name)
}

// FindServersByTag returns all servers that have the given tag.
func FindServersByTag(global *GlobalConfig, tag string) []*ServerEntry {
	var result []*ServerEntry
	for i := range global.Servers {
		for _, t := range global.Servers[i].Tags {
			if t == tag {
				result = append(result, &global.Servers[i])
				break
			}
		}
	}
	return result
}

// Connect opens a gRPC connection to the server, using mTLS if certs are configured.
func Connect(srv *ServerEntry) (agentv1.AgentServiceClient, *grpc.ClientConn, error) {
	host := srv.Host

	var dialOpts []grpc.DialOption

	certFile := expandHome(srv.TLS.Cert)
	keyFile := expandHome(srv.TLS.Key)
	caFile := expandHome(srv.TLS.CA)

	if certFile != "" && keyFile != "" && caFile != "" {
		for _, f := range []struct{ label, path string }{
			{"TLS cert", certFile}, {"TLS key", keyFile}, {"CA cert", caFile},
		} {
			if _, err := os.Stat(f.path); os.IsNotExist(err) {
				return nil, nil, fmt.Errorf("%s not found: %s\n  Run: kaptan cert init", f.label, f.path)
			}
		}
		creds, err := buildClientTLS(certFile, keyFile, caFile)
		if err != nil {
			return nil, nil, fmt.Errorf("TLS setup: %w", err)
		}
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	//nolint:staticcheck
	conn, err := grpc.Dial(host, dialOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to %s: %w", host, err)
	}

	return agentv1.NewAgentServiceClient(conn), conn, nil
}

func buildClientTLS(certFile, keyFile, caFile string) (credentials.TransportCredentials, error) {
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
		RootCAs:      caPool,
	}
	return credentials.NewTLS(cfg), nil
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}
