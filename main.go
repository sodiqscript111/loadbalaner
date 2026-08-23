package main

import (
	"fmt"
	"io"
	"log"
	"net"

	"loadbalancer/utils"
)

func handleConnection(clientConn net.Conn, backends []string) {
	defer clientConn.Close()

	tuple, err := utils.GetFiveTuple(clientConn)
	if err != nil {
		log.Printf("Error extracting 5-tuple: %v", err)
		return
	}

	selectedBackend := utils.SelectBackend(tuple, backends)
	log.Printf("Routing %s -> backend %s", clientConn.RemoteAddr(), selectedBackend)

	backendConn, err := net.Dial("tcp", selectedBackend)
	if err != nil {
		log.Printf("Failed to connect to backend %s: %v", selectedBackend, err)
		return
	}
	defer backendConn.Close()

	done := make(chan struct{}, 2)

	go func() {
		io.Copy(backendConn, clientConn)
		done <- struct{}{}
	}()

	go func() {
		io.Copy(clientConn, backendConn)
		done <- struct{}{}
	}()

	<-done
}

func main() {
	backends := []string{
		"127.0.0.1:8081",
		"127.0.0.1:8082",
		"127.0.0.1:8083",
	}

	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	defer listener.Close()

	fmt.Println("L4 Load balancer running on :8080...")

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}

		go handleConnection(conn, backends)
	}
}
