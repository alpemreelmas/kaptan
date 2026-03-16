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

func TestParseNginxLog(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "access.log")
	if err := os.WriteFile(logPath, []byte(sampleLog), 0644); err != nil {
		t.Fatal(err)
	}

	edges, err := graph.ParseNginxLog(logPath)
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

	// check error count for 500 edge
	for _, e := range edges {
		if e.To == "my-python-api" && e.StatusCode == 500 {
			if e.ErrorCount == 0 {
				t.Error("expected non-zero error count for 500 edge")
			}
		}
	}
}

func TestParseNginxLog_Missing(t *testing.T) {
	_, err := graph.ParseNginxLog("/nonexistent/access.log")
	if err == nil {
		t.Fatal("expected error for missing log file")
	}
}
