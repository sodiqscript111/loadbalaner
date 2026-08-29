package config

import (
	"flag"
	"fmt"
	"strings"
)

type Config struct {
	Mode     string
	Port     string
	Backends []string
	TLSCert  string
	TLSKey   string
}

func ParseFlags() (*Config, error) {
	var (
		mode     = flag.String("mode", "l4", "Run mode: 'l4' or 'l7'")
		port     = flag.String("port", ":8080", "Listen port (e.g., :8080)")
		backends = flag.String("backends", "127.0.0.1:8081", "Comma-separated list of backend addresses")
		tlsCert  = flag.String("tls-cert", "", "Path to TLS certificate file")
		tlsKey   = flag.String("tls-key", "", "Path to TLS private key file")
	)

	flag.Parse()

	m := strings.ToLower(strings.TrimSpace(*mode))
	if m != "l4" && m != "l7" {
		return nil, fmt.Errorf("invalid mode: must be 'l4' or 'l7'")
	}

	rawBackends := strings.Split(*backends, ",")
	var parsedBackends []string
	for _, b := range rawBackends {
		trimmed := strings.TrimSpace(b)
		if trimmed != "" {
			parsedBackends = append(parsedBackends, trimmed)
		}
	}

	if len(parsedBackends) == 0 {
		return nil, fmt.Errorf("at least one backend must be provided")
	}

	return &Config{
		Mode:     m,
		Port:     *port,
		Backends: parsedBackends,
		TLSCert:  *tlsCert,
		TLSKey:   *tlsKey,
	}, nil
}
