# Enterprise Load Balancer

A high-performance, multi-tier load balancer written in Go. This project is designed to handle massive scale by supporting both Layer 4 (TCP) and Layer 7 (HTTP) proxying, allowing for a highly scalable architecture where an L4 load balancer can distribute traffic across multiple L7 reverse proxies to prevent bottlenecks.

## Features

- **Multi-Tier Architecture:** Run as a Layer 4 TCP load balancer or a Layer 7 HTTP Reverse Proxy.
- **TLS Termination:** Built-in support for SSL/TLS certificates at both the L4 and L7 levels.
- **Active Health Checking:** Background goroutines actively probe backend servers (TCP and HTTP) and dynamically remove unhealthy nodes from the active pool.
- **Intelligent Routing:** 
  - L4 uses 5-tuple consistent hashing (Source IP, Destination IP, Source Port, Destination Port, Protocol).
  - L7 uses Round-Robin selection.
- **Resilience and Circuit Breaking:** The L7 proxy includes a custom transport that catches backend failures mid-flight, marks the backend as down, and silently retries the request on a healthy server.
- **Header Sterilization:** Automatically strips internal and potentially malicious headers from client requests while injecting verified X-Forwarded-For headers.
- **Wait-Free Concurrency:** Implements a Left-Right (Double-Buffered) server pool pattern, ensuring that millions of concurrent read requests are never blocked or delayed, even when health checkers actively add or remove backends in the background.
- **High Performance:** Utilizes sync.Pool for byte buffers in L4 mode to drastically reduce garbage collection overhead and memory allocation during heavy TCP traffic.

## Architecture and Scalability

This load balancer is designed for scalability. A common bottleneck in web infrastructure is the Layer 7 proxy itself, as parsing HTTP headers and terminating TLS can be CPU-intensive.

To solve this, this project allows you to deploy an L4 Load Balancer at the edge of your network. This L4 instance simply forwards TCP packets at extremely high speeds to a pool of L7 instances. The L7 instances then handle the heavy lifting (TLS termination, header manipulation, retries) before passing the traffic to your backend application servers.

### Concurrency Model

A major challenge for load balancers is dealing with massive read concurrency while background tasks (like health checks or admin scaling) modify the server pool. Using a standard `sync.RWMutex` causes severe latency spikes whenever a write occurs.

This project solves this by using a **Left-Right Wait-Free Pattern**. The server pool maintains two identical lists of the active servers. Incoming user traffic reads from the "active" list without requiring *any* locks. When a server goes down, the background health checker mutates the "inactive" list, atomically flips a pointer to make it the new active list, and waits for the old readers to drain. This ensures that high-volume traffic is never halted by internal state changes.

## Usage

Start the load balancer using the CLI flags:

- `--mode` : Set to "l4" or "l7" (default: "l4")
- `--port` : The port to listen on (default: ":8080")
- `--backends` : A comma-separated list of backend addresses (default: "127.0.0.1:8081")
- `--max-retries` : Maximum number of retries for failed requests in L7 mode (default: 3)
- `--tls-cert` : Path to TLS certificate file (optional)
- `--tls-key` : Path to TLS private key file (optional)

### Example: L7 Reverse Proxy
```bash
go run main.go --mode=l7 --port=:8081 --backends=127.0.0.1:9001,127.0.0.1:9002
```

### Example: L4 Edge Balancer with TLS
```bash
go run main.go --mode=l4 --port=:443 --backends=127.0.0.1:8081,127.0.0.1:8082 --tls-cert=cert.pem --tls-key=key.pem
```
