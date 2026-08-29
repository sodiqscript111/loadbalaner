package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"loadbalancer/backend"
	"loadbalancer/config"
)

func TestL7Proxy_HeaderSterilization(t *testing.T) {
	// 1. Create a mock backend server
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers are sterilized
		if r.Header.Get("X-Internal-Secret") != "" {
			t.Errorf("Expected X-Internal-Secret to be stripped, but it was present")
		}
		if r.Header.Get("Server") != "" {
			t.Errorf("Expected Server header to be stripped, but it was present")
		}
		
		// Verify new headers are injected
		if r.Header.Get("X-Forwarded-For") == "" {
			t.Errorf("Expected X-Forwarded-For to be injected")
		}
		
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer backendServer.Close()

	// 2. Setup the ServerPool with our mock backend
	pool := backend.NewServerPool()
	u, _ := url.Parse(backendServer.URL)
	b := backend.NewBackend(backendServer.URL, u)
	b.SetAlive(true)
	pool.AddBackend(b)

	// 3. Create the L7 Proxy
	cfg := &config.Config{MaxRetries: 3}
	l7proxy := NewL7Proxy(pool, cfg)

	// 4. Send a request with malicious headers
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Internal-Secret", "super-secret-key")
	req.Header.Set("Server", "hacker-server")
	req.RemoteAddr = "192.168.1.100:12345"

	rr := httptest.NewRecorder()
	l7proxy.ServeHTTP(rr, req)

	// 5. Verify the response
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 OK, got %d", rr.Code)
	}
}

func TestL7Proxy_AutomaticRetries(t *testing.T) {
	var requestCount int32

	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		if count == 1 {
			// First attempt: simulate a crash / bad gateway
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		// Second attempt: success
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("SUCCESS"))
	}))
	defer backendServer.Close()

	pool := backend.NewServerPool()
	u, _ := url.Parse(backendServer.URL)
	
	// Add 2 backends pointing to the same mock server to test round robin / retries
	b1 := backend.NewBackend(backendServer.URL, u)
	b1.SetAlive(true)
	pool.AddBackend(b1)
	
	b2 := backend.NewBackend(backendServer.URL, u)
	b2.SetAlive(true)
	pool.AddBackend(b2)

	cfg := &config.Config{MaxRetries: 3}
	l7proxy := NewL7Proxy(pool, cfg)

	// Send a standard request
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	l7proxy.ServeHTTP(rr, req)

	// The proxy should have caught the 502, retried, and eventually gotten the 200 OK.
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 OK after retry, got %d", rr.Code)
	}

	body, _ := io.ReadAll(rr.Body)
	if string(body) != "SUCCESS" {
		t.Errorf("Expected body 'SUCCESS', got %s", string(body))
	}

	if atomic.LoadInt32(&requestCount) != 2 {
		t.Errorf("Expected exactly 2 requests to reach the backend, got %d", requestCount)
	}
}

func TestL7Proxy_NoBackendsAvailable(t *testing.T) {
	pool := backend.NewServerPool()
	// No backends added

	cfg := &config.Config{MaxRetries: 3}
	l7proxy := NewL7Proxy(pool, cfg)

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	l7proxy.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected 503 Service Unavailable, got %d", rr.Code)
	}
}
