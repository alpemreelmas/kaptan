package graph_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alpemreelmas/kaptan/agent/graph"
)

const sampleLog = `127.0.0.1 - - [13/Mar/2026:12:00:00 +0000] "GET http://my-nodejs-app:3000/api HTTP/1.1" 200 1234
127.0.0.1 - - [13/Mar/2026:12:00:01 +0000] "POST http://my-python-api:5000/predict HTTP/1.1" 500 56
127.0.0.1 - - [13/Mar/2026:12:00:02 +0000] "GET http://my-python-api:5000/health HTTP/1.1" 200 12
`

func writeLog(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "access.log")
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return logPath
}

func TestParseNginxLog(t *testing.T) {
	logPath := writeLog(t, sampleLog)

	edges, err := graph.ParseNginxLog(logPath, nil)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	hosts := map[string]bool{}
	for _, e := range edges {
		hosts[e.To] = true
	}

	if !hosts["my-nodejs-app"] {
		t.Error("expected edge to my-nodejs-app")
	}
	if !hosts["my-python-api"] {
		t.Error("expected edge to my-python-api")
	}

	for _, e := range edges {
		if e.To == "my-python-api" && e.StatusCode == 500 {
			if e.ErrorCount == 0 {
				t.Error("expected non-zero error count for 500 edge")
			}
		}
	}
}

func TestParseNginxLog_Missing(t *testing.T) {
	_, err := graph.ParseNginxLog("/nonexistent/access.log", nil)
	if err == nil {
		t.Fatal("expected error for missing log file")
	}
}

// TestEdgeKind_EmptyDomainList: internal_domains boşsa tüm edge'ler Internal sayılır.
func TestEdgeKind_EmptyDomainList(t *testing.T) {
	logPath := writeLog(t, sampleLog)
	edges, err := graph.ParseNginxLog(logPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range edges {
		if e.Kind != graph.Internal {
			t.Errorf("expected Internal for %s with empty domain list, got External", e.To)
		}
	}
}

// TestEdgeKind_InternalMatch: listedeki host'lar Internal, diğerleri External.
func TestEdgeKind_InternalMatch(t *testing.T) {
	const mixedLog = `127.0.0.1 - - [13/Mar/2026:12:00:00 +0000] "GET http://my-nodejs-app:3000/api HTTP/1.1" 200 1234
127.0.0.1 - - [13/Mar/2026:12:00:01 +0000] "POST http://api.stripe.com/v1/charges HTTP/1.1" 200 56
`
	logPath := writeLog(t, mixedLog)
	domains := []string{"my-nodejs-app", "*.internal"}

	edges, err := graph.ParseNginxLog(logPath, domains)
	if err != nil {
		t.Fatal(err)
	}

	kinds := map[string]graph.EdgeKind{}
	for _, e := range edges {
		kinds[e.To] = e.Kind
	}

	if kinds["my-nodejs-app"] != graph.Internal {
		t.Error("my-nodejs-app should be Internal")
	}
	if kinds["api.stripe.com"] != graph.External {
		t.Error("api.stripe.com should be External")
	}
}

// TestEdgeKind_WildcardPattern: *.internal pattern'ı wildcard olarak çalışır.
func TestEdgeKind_WildcardPattern(t *testing.T) {
	const wildcardLog = `127.0.0.1 - - [13/Mar/2026:12:00:00 +0000] "GET http://auth-service.internal:8080/verify HTTP/1.1" 200 12
127.0.0.1 - - [13/Mar/2026:12:00:01 +0000] "GET http://api.sendgrid.com/v3/mail HTTP/1.1" 202 0
`
	logPath := writeLog(t, wildcardLog)
	domains := []string{"*.internal"}

	edges, err := graph.ParseNginxLog(logPath, domains)
	if err != nil {
		t.Fatal(err)
	}

	kinds := map[string]graph.EdgeKind{}
	for _, e := range edges {
		kinds[e.To] = e.Kind
	}

	if kinds["auth-service.internal"] != graph.Internal {
		t.Error("auth-service.internal should match *.internal pattern → Internal")
	}
	if kinds["api.sendgrid.com"] != graph.External {
		t.Error("api.sendgrid.com should be External")
	}
}

// TestMatchesInternal: MatchesInternal fonksiyonu unit testi.
func TestMatchesInternal(t *testing.T) {
	patterns := []string{"my-api", "*.internal", "*.svc.cluster.local"}

	cases := []struct {
		host     string
		expected bool
	}{
		{"my-api", true},
		{"auth-service.internal", true},
		{"payments.svc.cluster.local", true},
		{"api.stripe.com", false},
		{"amazonaws.com", false},
		{"my-api.external.com", false},
	}

	for _, c := range cases {
		got := graph.MatchesInternal(c.host, patterns)
		if got != c.expected {
			t.Errorf("MatchesInternal(%q) = %v, want %v", c.host, got, c.expected)
		}
	}
}
