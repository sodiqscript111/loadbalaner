package proxy

import (
	"log"
	"net/http"
	"net/http/httputil"

	"loadbalancer/backend"
	"loadbalancer/utils"
)

type L7Proxy struct {
	pool  *backend.ServerPool
	proxy *httputil.ReverseProxy
}

func NewL7Proxy(pool *backend.ServerPool) *L7Proxy {
	return &L7Proxy{
		pool: pool,
		proxy: &httputil.ReverseProxy{
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

				log.Printf("L7 Proxy routing %s -> %s", req.URL.Path, req.URL.Host)
			},
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
				log.Printf("L7 Proxy Error: %v", err)
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte("Service Unavailable"))
			},
		},
	}
}

func (p *L7Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if p.pool.GetAliveCount() == 0 {
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}

	p.proxy.ServeHTTP(w, r)
}
