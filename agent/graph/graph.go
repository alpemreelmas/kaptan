package graph

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// EdgeKind distinguishes internal service-to-service calls from external API calls.
type EdgeKind int

const (
	Internal EdgeKind = iota
	External
)

// Edge represents a dependency between two services observed in access logs.
type Edge struct {
	From       string
	To         string
	StatusCode int
	ErrorCount int
	Kind       EdgeKind
}

// MatchesInternal reports whether host matches any pattern in the internal domain list.
// Patterns support glob wildcards (e.g. "*.internal", "*.svc.cluster.local").
func MatchesInternal(host string, patterns []string) bool {
	for _, p := range patterns {
		if matched, _ := filepath.Match(p, host); matched {
			return true
		}
		// also match bare hostname (no dots) as internal by default
		if p == host {
			return true
		}
	}
	return false
}

// reNginx matches lines like:
// 127.0.0.1 - - [date] "GET http://my-nodejs-app:3000/api HTTP/1.1" 200 1234
var reNginx = regexp.MustCompile(
	`"(?:GET|POST|PUT|DELETE|PATCH|HEAD) https?://([^:/\s"]+)(?::\d+)?[^\s"]* HTTP/\S+"\s+(\d{3})`,
)

// serviceNameFromPath extracts a human-readable service name from a log file path.
// /var/log/nginx/my-api.access.log → "my-api"
// /var/log/nginx/my-api.log        → "my-api"
// access.log                       → "access"
func serviceNameFromPath(logFile string) string {
	base := filepath.Base(logFile)
	name := strings.TrimSuffix(base, filepath.Ext(base))      // strip last ext
	name = strings.TrimSuffix(name, ".access")                 // strip .access if present
	if name == "" {
		return "unknown"
	}
	return name
}

// ParseNginxLog reads an nginx access log and returns edges grouped by
// (from, to, status_code) with accurate error counts.
// internalDomains is a list of glob patterns used to classify edges as Internal or External.
// An empty list causes all edges to be classified as Internal.
func ParseNginxLog(logFile string, internalDomains []string) ([]Edge, error) {
	f, err := os.Open(logFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	from := serviceNameFromPath(logFile)

	type edgeKey struct {
		to   string
		code int
	}
	counts := map[edgeKey]int{}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		m := reNginx.FindStringSubmatch(scanner.Text())
		if m == nil {
			continue
		}
		code, _ := strconv.Atoi(m[2])
		counts[edgeKey{to: m[1], code: code}]++
	}

	edges := make([]Edge, 0, len(counts))
	for k, cnt := range counts {
		errCount := 0
		if k.code >= 400 {
			errCount = cnt
		}
		kind := External
		if len(internalDomains) == 0 || MatchesInternal(k.to, internalDomains) {
			kind = Internal
		}
		edges = append(edges, Edge{
			From:       from,
			To:         k.to,
			StatusCode: k.code,
			ErrorCount: errCount,
			Kind:       kind,
		})
	}
	return edges, nil
}
