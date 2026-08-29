package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"loadbalancer/backend"
	"loadbalancer/config"
	"loadbalancer/health"
	"loadbalancer/proxy"
	"loadbalancer/utils"
)

var bufferPool = sync.Pool{
	New: func() interface{} {
		// Allocate a 32KB buffer (standard io.Copy buffer size)
		return make([]byte, 32*1024)
	},
}

func handleConnection(clientConn net.Conn, pool *backend.ServerPool) {
	defer clientConn.Close()

	tuple, err := utils.GetFiveTuple(clientConn)
	if err != nil {
		log.Printf("Error extracting 5-tuple: %v", err)
		return
	}

	selectedBackend := utils.SelectHealthyBackend(tuple, pool)
	if selectedBackend == nil {
		log.Printf("No healthy backends available to route connection from %s", clientConn.RemoteAddr())
		return
	}

	selectedBackend.IncConns()
	defer selectedBackend.DecConns()

	backendConn, err := net.DialTimeout("tcp", selectedBackend.Addr, 2*time.Second)
	if err != nil {
		log.Printf("Failed to connect to backend %s: %v", selectedBackend.Addr, err)
		return
	}
	defer backendConn.Close()

	done := make(chan struct{}, 2)

	go func() {
		buf := bufferPool.Get().([]byte)
		defer bufferPool.Put(buf)
		_, _ = io.CopyBuffer(backendConn, clientConn, buf)
		done <- struct{}{}
	}()

	go func() {
		buf := bufferPool.Get().([]byte)
		defer bufferPool.Put(buf)
		_, _ = io.CopyBuffer(clientConn, backendConn, buf)
		done <- struct{}{}
	}()

	<-done
}

func main() {
	cfg, err := config.ParseFlags()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v, shutting down...", sig)
		cancel()
		os.Exit(0)
	}()

	pool := backend.NewServerPool()
	for _, b := range cfg.Backends {
		var rawURL *url.URL
		if cfg.Mode == "l7" {
			rawURL, _ = url.Parse(fmt.Sprintf("http://%s", b))
		}
		pool.AddBackend(backend.NewBackend(b, rawURL))
	}

	checkerCfg := health.DefaultCheckerConfig()
	if cfg.Mode == "l7" {
		checkerCfg.CheckType = health.CheckTypeHTTP
	} else {
		checkerCfg.CheckType = health.CheckTypeTCP
	}
	
	checkerCfg.OnStatusChange = func(b *backend.Backend, alive bool) {
		status := "DOWN"
		if alive {
			status = "UP"
		}
		log.Printf("[Event] Backend %s status changed to %s (Alive backends: %d/%d)",
			b.Addr, status, pool.GetAliveCount(), pool.GetBackendCount())
	}

	checker := health.NewHealthChecker(pool, checkerCfg)
	go checker.Start(ctx)

	if cfg.Mode == "l7" {
		startL7Proxy(cfg, pool)
	} else {
		startL4Proxy(ctx, cfg, pool)
	}
}

func startL7Proxy(cfg *config.Config, pool *backend.ServerPool) {
	l7proxy := proxy.NewL7Proxy(pool, cfg)
	server := &http.Server{
		Addr:    cfg.Port,
		Handler: l7proxy,
	}

	log.Printf("Starting L7 HTTP(S) Reverse Proxy on %s...", cfg.Port)
	var err error
	if cfg.TLSCert != "" && cfg.TLSKey != "" {
		log.Printf("TLS Termination enabled.")
		err = server.ListenAndServeTLS(cfg.TLSCert, cfg.TLSKey)
	} else {
		err = server.ListenAndServe()
	}

	if err != nil {
		log.Fatalf("L7 Proxy failed: %v", err)
	}
}

func startL4Proxy(ctx context.Context, cfg *config.Config, pool *backend.ServerPool) {
	var listener net.Listener
	var err error

	if cfg.TLSCert != "" && cfg.TLSKey != "" {
		log.Printf("TLS Termination enabled.")
		cer, errLoad := tls.LoadX509KeyPair(cfg.TLSCert, cfg.TLSKey)
		if errLoad != nil {
			log.Fatalf("Failed to load TLS keys: %v", errLoad)
		}
		tlsConfig := &tls.Config{Certificates: []tls.Certificate{cer}}
		listener, err = tls.Listen("tcp", cfg.Port, tlsConfig)
	} else {
		listener, err = net.Listen("tcp", cfg.Port)
	}

	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	defer listener.Close()

	log.Printf("Starting L4 TCP(S) Load Balancer on %s...", cfg.Port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				log.Printf("Failed to accept connection: %v", err)
				continue
			}
		}

		go handleConnection(conn, pool)
	}
}
