package remotedesktopengine

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
	client.CheckRedirect = secureRedirectPolicy(client.CheckRedirect)
	return client
}

func secureRedirectPolicy(next func(*http.Request, []*http.Request) error) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if err := validateRedirectTarget(req); err != nil {
			return err
		}
		if next != nil {
			return next(req, via)
		}
		return nil
	}
}

func validateRedirectTarget(req *http.Request) error {
	if req == nil || req.URL == nil {
		return nil
	}

	scheme := strings.ToLower(req.URL.Scheme)
	switch scheme {
	case "", "https", "wss":
		return nil
	default:
		return fmt.Errorf("redirect to insecure scheme blocked: %s", scheme)
	}
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

	if transport.TLSClientConfig != nil {
		transport.TLSClientConfig.InsecureSkipVerify = false
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
