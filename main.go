package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"loadbalancer/backend"
	"loadbalancer/health"
	"loadbalancer/utils"
)

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

	log.Printf("Routing %s -> backend %s (Active conns: %d)",
		clientConn.RemoteAddr(), selectedBackend.Addr, selectedBackend.GetActiveConns())

	backendConn, err := net.DialTimeout("tcp", selectedBackend.Addr, 2*time.Second)
	if err != nil {
		log.Printf("Failed to connect to backend %s: %v", selectedBackend.Addr, err)
		return
	}
	defer backendConn.Close()

	done := make(chan struct{}, 2)

	go func() {
		_, _ = io.Copy(backendConn, clientConn)
		done <- struct{}{}
	}()

	go func() {
		_, _ = io.Copy(clientConn, backendConn)
		done <- struct{}{}
	}()

	<-done
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v, shutting down...", sig)
		cancel()
		os.Exit(0)
	}()

	backendAddrs := []string{
		"127.0.0.1:8081",
		"127.0.0.1:8082",
		"127.0.0.1:8083",
	}

	pool := backend.NewServerPool()
	for _, addr := range backendAddrs {
		pool.AddBackend(backend.NewBackend(addr, nil))
	}

	// Configure and start active health checker
	checkerCfg := health.DefaultCheckerConfig()
	checkerCfg.Interval = 5 * time.Second
	checkerCfg.Timeout = 2 * time.Second
	checkerCfg.CheckType = health.CheckTypeTCP
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

	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	defer listener.Close()

	fmt.Println("L4 Load balancer with Active Health Checking running on :8080...")

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
