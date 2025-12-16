package netx

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/proxy"
)

type ClientConfig struct {
	Timeout       time.Duration
	TorSocks5Addr string // e.g. 127.0.0.1:9050 or 127.0.0.1:9150
	InsecureTLS   bool   // dev only
}

func NewHTTPClient(cfg ClientConfig) (*http.Client, error) {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}

	baseDialer := &net.Dialer{Timeout: 10 * time.Second}
	var dialer proxy.Dialer = baseDialer

	if cfg.TorSocks5Addr != "" {
		socks, err := proxy.SOCKS5("tcp", cfg.TorSocks5Addr, nil, baseDialer)
		if err != nil {
			return nil, err
		}
		dialer = socks
	}

	tr := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			_ = ctx // SOCKS5 dialer has no context support
			return dialer.Dial(network, addr)
		},
		TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.InsecureTLS}, // dev only
	}

	return &http.Client{Timeout: timeout, Transport: tr}, nil
}
