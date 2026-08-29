package health

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"loadbalancer/backend"
)

// CheckType defines the probing mechanism.
type CheckType int

const (
	CheckTypeTCP CheckType = iota
	CheckTypeHTTP
)

// CheckerConfig defines settings for the health checking engine.
type CheckerConfig struct {
	Interval         time.Duration
	Timeout          time.Duration
	CheckType        CheckType
	HTTPPath         string // e.g. "/healthz"
	SuccessThreshold int32  // consecutive successes required to mark healthy
	FailureThreshold int32  // consecutive failures required to mark unhealthy
	OnStatusChange   func(b *backend.Backend, alive bool)
}

// DefaultCheckerConfig provides standard healthy checking defaults.
func DefaultCheckerConfig() CheckerConfig {
	return CheckerConfig{
		Interval:         5 * time.Second,
		Timeout:          2 * time.Second,
		CheckType:        CheckTypeTCP,
		HTTPPath:         "/healthz",
		SuccessThreshold: 1,
		FailureThreshold: 2,
	}
}

// HealthChecker actively monitors the health of servers in a ServerPool.
type HealthChecker struct {
	pool       *backend.ServerPool
	config     CheckerConfig
	httpClient *http.Client
}

// NewHealthChecker creates a new HealthChecker.
func NewHealthChecker(pool *backend.ServerPool, cfg CheckerConfig) *HealthChecker {
	if cfg.Interval <= 0 {
		cfg.Interval = 5 * time.Second
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 2 * time.Second
	}
	if cfg.SuccessThreshold <= 0 {
		cfg.SuccessThreshold = 1
	}
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 1
	}
	if cfg.HTTPPath == "" {
		cfg.HTTPPath = "/healthz"
	}

	return &HealthChecker{
		pool:   pool,
		config: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// Start runs periodic health checks until the context is canceled.
func (c *HealthChecker) Start(ctx context.Context) {
	log.Printf("[HealthChecker] Starting active health checks (Type: %v, Interval: %v, Timeout: %v)...",
		c.config.CheckType, c.config.Interval, c.config.Timeout)

	// Run initial check immediately
	c.CheckAll(ctx)

	ticker := time.NewTicker(c.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[HealthChecker] Stopping active health checks")
			return
		case <-ticker.C:
			c.CheckAll(ctx)
		}
	}
}

// CheckAll checks all backends in parallel.
func (c *HealthChecker) CheckAll(ctx context.Context) {
	backends := c.pool.GetBackends()
	if len(backends) == 0 {
		return
	}

	var wg sync.WaitGroup
	for _, b := range backends {
		wg.Add(1)
		go func(node *backend.Backend) {
			defer wg.Done()
			c.CheckBackend(ctx, node)
		}(b)
	}
	wg.Wait()
}

// CheckBackend executes a single probe against a backend and updates its state.
func (c *HealthChecker) CheckBackend(ctx context.Context, b *backend.Backend) {
	var err error
	if c.config.CheckType == CheckTypeHTTP {
		err = c.probeHTTP(ctx, b)
	} else {
		err = c.probeTCP(ctx, b)
	}

	wasAlive := b.IsAlive()

	if err != nil {
		fails := b.RecordFailure()
		if wasAlive && fails >= c.config.FailureThreshold {
			b.SetAlive(false)
			log.Printf("[HealthChecker] ❌ Backend %s is DOWN (probe error: %v, failures: %d)", b.Addr, err, fails)
			if c.config.OnStatusChange != nil {
				c.config.OnStatusChange(b, false)
			}
		}
	} else {
		successes := b.RecordSuccess()
		if !wasAlive && successes >= c.config.SuccessThreshold {
			b.SetAlive(true)
			log.Printf("[HealthChecker] ✅ Backend %s is BACK UP (successes: %d)", b.Addr, successes)
			if c.config.OnStatusChange != nil {
				c.config.OnStatusChange(b, true)
			}
		}
	}
}

func (c *HealthChecker) probeTCP(ctx context.Context, b *backend.Backend) error {
	dialer := net.Dialer{Timeout: c.config.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", b.Addr)
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}

func (c *HealthChecker) probeHTTP(ctx context.Context, b *backend.Backend) error {
	var targetURL string
	if b.URL != nil {
		targetURL = fmt.Sprintf("%s://%s%s", b.URL.Scheme, b.URL.Host, c.config.HTTPPath)
	} else {
		targetURL = fmt.Sprintf("http://%s%s", b.Addr, c.config.HTTPPath)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
