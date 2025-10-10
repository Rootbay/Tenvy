package remotedesktop

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	defaultDialTimeout     = 10 * time.Second
	defaultDialKeepAlive   = 30 * time.Second
	defaultIdleConnTimeout = 90 * time.Second
	defaultTLSHandshake    = 10 * time.Second
	defaultExpectContinue  = 1 * time.Second
	minimumMaxIdleConns    = 64
	minimumMaxIdlePerHost  = 16
)

func secureHTTPClient(doer HTTPDoer) HTTPDoer {
	client, ok := doer.(*http.Client)
	if !ok {
		if doer != nil {
			return doer
		}
		client = &http.Client{}
	} else {
		client = cloneHTTPClient(client)
	}

	if client.Timeout <= 0 {
		client.Timeout = defaultFrameRequestTimeout
	}

	client.Transport = secureHTTPTransport(client.Transport)
	return client
}

func secureHTTPTransport(rt http.RoundTripper) http.RoundTripper {
	transport, ok := rt.(*http.Transport)
	switch {
	case ok:
		transport = transport.Clone()
	case rt == nil:
		defaultTransport, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			return rt
		}
		transport = defaultTransport.Clone()
	default:
		return rt
	}

	if transport.DialContext == nil {
		transport.DialContext = (&net.Dialer{
			Timeout:   defaultDialTimeout,
			KeepAlive: defaultDialKeepAlive,
		}).DialContext
	}

	if transport.MaxIdleConns < minimumMaxIdleConns {
		transport.MaxIdleConns = minimumMaxIdleConns
	}
	if transport.MaxIdleConnsPerHost < minimumMaxIdlePerHost {
		transport.MaxIdleConnsPerHost = minimumMaxIdlePerHost
	}
	if transport.IdleConnTimeout <= 0 {
		transport.IdleConnTimeout = defaultIdleConnTimeout
	}
	if transport.TLSHandshakeTimeout <= 0 {
		transport.TLSHandshakeTimeout = defaultTLSHandshake
	}
	if transport.ExpectContinueTimeout <= 0 {
		transport.ExpectContinueTimeout = defaultExpectContinue
	}

	transport.ForceAttemptHTTP2 = true

	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	} else {
		cfg := transport.TLSClientConfig.Clone()
		if cfg.MinVersion < tls.VersionTLS12 {
			cfg.MinVersion = tls.VersionTLS12
		}
		transport.TLSClientConfig = cfg
	}

	return transport
}

func buildAuthHeader(key string) string {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return ""
	}
	return fmt.Sprintf("Bearer %s", trimmed)
}

func cloneHTTPClient(base *http.Client) *http.Client {
	if base == nil {
		return &http.Client{}
	}
	clone := *base
	return &clone
}
