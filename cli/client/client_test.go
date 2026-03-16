package client_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alpemreelmas/kaptan/cli/client"
)

// --- ProjectConfig.Validate ---

func TestProjectConfig_Validate_OK(t *testing.T) {
	cfg := &client.ProjectConfig{
		Service: "my-app",
		Server:  "web-prod-1",
		Path:    "/home/forge/myapp",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProjectConfig_Validate_MissingFields(t *testing.T) {
	cases := []struct {
		name string
		cfg  client.ProjectConfig
		want string
	}{
		{
			name: "missing service",
			cfg:  client.ProjectConfig{Server: "s", Path: "/p"},
			want: "service",
		},
		{
			name: "missing server",
			cfg:  client.ProjectConfig{Service: "svc", Path: "/p"},
			want: "server",
		},
		{
			name: "missing path",
			cfg:  client.ProjectConfig{Service: "svc", Server: "s"},
			want: "path",
		},
		{
			name: "missing all",
			cfg:  client.ProjectConfig{},
			want: "service",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if msg := err.Error(); len(msg) == 0 {
				t.Fatal("empty error message")
			}
		})
	}
}

// --- LoadProjectConfig ---

func TestLoadProjectConfig_NotFound(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	_, err := client.LoadProjectConfig()
	if err == nil {
		t.Fatal("expected error for missing .kaptan/config.yaml")
	}
}

func TestLoadProjectConfig_Valid(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(".kaptan", 0755); err != nil {
		t.Fatal(err)
	}
	content := "service: my-app\nserver: web-1\npath: /home/forge/app\nhealth_url: http://localhost/health\n"
	if err := os.WriteFile(".kaptan/config.yaml", []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := client.LoadProjectConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Service != "my-app" {
		t.Errorf("got service=%q, want %q", cfg.Service, "my-app")
	}
	if cfg.Server != "web-1" {
		t.Errorf("got server=%q, want %q", cfg.Server, "web-1")
	}
	if cfg.Path != "/home/forge/app" {
		t.Errorf("got path=%q, want %q", cfg.Path, "/home/forge/app")
	}
}

func TestLoadProjectConfig_MissingRequiredFields(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(".kaptan", 0755); err != nil {
		t.Fatal(err)
	}
	// Only health_url, missing service/server/path
	content := "health_url: http://localhost/health\n"
	if err := os.WriteFile(".kaptan/config.yaml", []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := client.LoadProjectConfig()
	if err == nil {
		t.Fatal("expected validation error")
	}
}

// --- LoadGlobalConfig ---

func TestLoadGlobalConfig_NotFound(t *testing.T) {
	// Point home to an empty temp dir
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	_, err := client.LoadGlobalConfig()
	if err == nil {
		t.Fatal("expected error for missing global config")
	}
}

func TestLoadGlobalConfig_Valid(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	kaptanDir := filepath.Join(dir, ".kaptan")
	if err := os.MkdirAll(kaptanDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := `servers:
  - name: web-prod-1
    host: "1.2.3.4:7000"
    tags: [prod]
`
	if err := os.WriteFile(filepath.Join(kaptanDir, "config.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := client.LoadGlobalConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Servers) != 1 {
		t.Fatalf("got %d servers, want 1", len(cfg.Servers))
	}
	if cfg.Servers[0].Name != "web-prod-1" {
		t.Errorf("got name=%q, want %q", cfg.Servers[0].Name, "web-prod-1")
	}
}

// --- FindServer ---

func TestFindServer_Found(t *testing.T) {
	global := &client.GlobalConfig{
		Servers: []client.ServerEntry{
			{Name: "web-prod-1", Host: "1.2.3.4:7000"},
			{Name: "web-staging-1", Host: "5.6.7.8:7000"},
		},
	}
	srv, err := client.FindServer(global, "web-staging-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if srv.Host != "5.6.7.8:7000" {
		t.Errorf("got host=%q, want %q", srv.Host, "5.6.7.8:7000")
	}
}

func TestFindServer_NotFound(t *testing.T) {
	global := &client.GlobalConfig{
		Servers: []client.ServerEntry{
			{Name: "web-prod-1", Host: "1.2.3.4:7000"},
		},
	}
	_, err := client.FindServer(global, "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown server")
	}
}

// --- FindServersByTag ---

func TestFindServersByTag(t *testing.T) {
	global := &client.GlobalConfig{
		Servers: []client.ServerEntry{
			{Name: "a", Host: "1.1.1.1:7000", Tags: []string{"prod"}},
			{Name: "b", Host: "2.2.2.2:7000", Tags: []string{"staging"}},
			{Name: "c", Host: "3.3.3.3:7000", Tags: []string{"prod", "eu"}},
		},
	}

	prod := client.FindServersByTag(global, "prod")
	if len(prod) != 2 {
		t.Errorf("got %d prod servers, want 2", len(prod))
	}

	eu := client.FindServersByTag(global, "eu")
	if len(eu) != 1 {
		t.Errorf("got %d eu servers, want 1", len(eu))
	}

	none := client.FindServersByTag(global, "nonexistent")
	if len(none) != 0 {
		t.Errorf("got %d servers for unknown tag, want 0", len(none))
	}
}
