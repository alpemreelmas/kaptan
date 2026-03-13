package health

import (
	"fmt"
	"net/http"
	"time"
)

// Check performs an HTTP GET on url and returns (ok, statusCode, message).
func Check(url string, timeout time.Duration) (bool, int, string) {
	if url == "" {
		return true, 0, "no health URL configured"
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return false, 0, fmt.Sprintf("request failed: %v", err)
	}
	defer resp.Body.Close()

	ok := resp.StatusCode >= 200 && resp.StatusCode < 300
	return ok, resp.StatusCode, fmt.Sprintf("HTTP %d", resp.StatusCode)
}
