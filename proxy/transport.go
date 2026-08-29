package proxy

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"

	"loadbalancer/backend"
	"loadbalancer/utils"
)

// RetryTransport wraps an http.RoundTripper to automatically retry failed requests
// on healthy backends and circuit-break failing ones.
type RetryTransport struct {
	Pool       *backend.ServerPool
	MaxRetries int
	Transport  http.RoundTripper
}

func (t *RetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var bodyBytes []byte
	var err error

	// If the request has a body, read it into memory so we can retry the request
	if req.Body != nil {
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		req.Body.Close()
	}

	transport := t.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	var resp *http.Response
	var lastErr error

	for attempt := 0; attempt <= t.MaxRetries; attempt++ {
		// On retries, pick a new backend
		if attempt > 0 {
			selectedBackend := utils.SelectRoundRobinHealthyBackend(t.Pool)
			if selectedBackend == nil {
				log.Printf("Retry attempt %d failed: no healthy backends available", attempt)
				break
			}
			req.URL.Scheme = selectedBackend.URL.Scheme
			if req.URL.Scheme == "" {
				req.URL.Scheme = "http"
			}
			req.URL.Host = selectedBackend.URL.Host
			if req.URL.Host == "" {
				req.URL.Host = selectedBackend.Addr
			}
			req.Host = req.URL.Host
			log.Printf("Retrying request on %s (attempt %d/%d)", req.URL.Host, attempt, t.MaxRetries)
		}

		// Repopulate body for each attempt
		if bodyBytes != nil {
			req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		resp, err = transport.RoundTrip(req)

		// Success condition: No transport error, and status code is not a 5xx Server Error
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}

		// The backend failed. We need to circuit break it.
		lastErr = err
		if err == nil {
			lastErr = fmt.Errorf("backend returned HTTP status %d", resp.StatusCode)
		}

		log.Printf("Backend %s failed: %v", req.URL.Host, lastErr)

		// Record the failure against the specific backend in the pool
		for _, b := range t.Pool.GetBackends() {
			host := b.Addr
			if b.URL != nil {
				host = b.URL.Host
			}
			if host == req.URL.Host {
				fails := b.RecordFailure()
				log.Printf("Recorded failure for %s (total consecutive failures: %d)", host, fails)
				break
			}
		}
	}

	if resp != nil {
		return resp, nil
	}

	return nil, lastErr
}
