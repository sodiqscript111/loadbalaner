package proxy

import (
	"log"
	"net"
	"net/http"
	"net/http/httputil"

	"loadbalancer/backend"
	"loadbalancer/config"
	"loadbalancer/utils"
)

type L7Proxy struct {
	pool  *backend.ServerPool
	proxy *httputil.ReverseProxy
}

func NewL7Proxy(pool *backend.ServerPool, cfg *config.Config) *L7Proxy {
	reverseProxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
				selectedBackend := utils.SelectRoundRobinHealthyBackend(pool)
				if selectedBackend == nil {
					log.Printf("No healthy backends available to serve L7 request for %s", req.URL.Path)
					return
				}

				req.URL.Scheme = selectedBackend.URL.Scheme
				if req.URL.Scheme == "" {
					req.URL.Scheme = "http"
				}
				req.URL.Host = selectedBackend.URL.Host
				if req.URL.Host == "" {
					req.URL.Host = selectedBackend.Addr
				}

				req.Header.Del("X-Forwarded-For")
				req.Header.Del("X-Forwarded-Host")
				req.Header.Del("X-Forwarded-Proto")
				req.Header.Del("X-Internal-Secret")
				req.Header.Del("Server")

				clientIP, _, err := net.SplitHostPort(req.RemoteAddr)
				if err == nil {
					req.Header.Set("X-Forwarded-For", clientIP)
				} else {
					req.Header.Set("X-Forwarded-For", req.RemoteAddr)
				}
				
				req.Header.Set("X-Forwarded-Host", req.Host)
				if req.TLS != nil {
					req.Header.Set("X-Forwarded-Proto", "https")
				} else {
					req.Header.Set("X-Forwarded-Proto", "http")
				}

				log.Printf("L7 Proxy routing %s -> %s", req.URL.Path, req.URL.Host)
			},
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
				log.Printf("L7 Proxy Error: %v", err)
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte("Service Unavailable"))
			},
		}

	reverseProxy.Transport = &RetryTransport{
		Pool:       pool,
		MaxRetries: cfg.MaxRetries,
		Transport:  http.DefaultTransport,
	}

	return &L7Proxy{
		pool:  pool,
		proxy: reverseProxy,
	}
}

func (p *L7Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if p.pool.GetAliveCount() == 0 {
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}

	p.proxy.ServeHTTP(w, r)
}
