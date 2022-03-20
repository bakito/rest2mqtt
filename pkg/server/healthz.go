package server

import (
	"fmt"
	"net/http"
)

// Healthz health check
func Healthz() int {
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d%s", port, healthzPath))
	if err == nil {
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode == http.StatusOK {
			return 0
		}
	}
	return 1
}
