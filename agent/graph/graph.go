package graph

import (
	"bufio"
	"os"
	"regexp"
	"strconv"
)

// Edge represents a dependency between two services observed in access logs.
type Edge struct {
	From       string
	To         string
	StatusCode int
	ErrorCount int
}

// reNginx matches lines like:
// 127.0.0.1 - - [date] "GET http://my-nodejs-app:3000/api HTTP/1.1" 200 1234
var reNginx = regexp.MustCompile(
	`"(?:GET|POST|PUT|DELETE|PATCH|HEAD) (https?://([^:/\s"]+)(?::\d+)?[^\s"]*) HTTP/\S+"?\s+(\d{3})`,
)

// ParseNginxLog reads an nginx access log and returns deduplicated edges.
func ParseNginxLog(logFile string) ([]Edge, error) {
	f, err := os.Open(logFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	type edgeKey struct{ from, to string; code int }
	counts := map[edgeKey]int{}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		m := reNginx.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		host := m[2]
		code, _ := strconv.Atoi(m[3])
		// derive "from" from server hostname in log (simplistic: use log filename)
		from := "unknown"
		if logFile != "" {
			from = logFile
		}
		key := edgeKey{from: from, to: host, code: code}
		counts[key]++
	}

	var edges []Edge
	seen := map[string]bool{}
	for k, cnt := range counts {
		pair := k.from + "->" + k.to
		isErr := k.code >= 400
		errCount := 0
		if isErr {
			errCount = cnt
		}
		if !seen[pair] {
			seen[pair] = true
			edges = append(edges, Edge{
				From:       k.from,
				To:         k.to,
				StatusCode: k.code,
				ErrorCount: errCount,
			})
		}
	}
	return edges, nil
}
