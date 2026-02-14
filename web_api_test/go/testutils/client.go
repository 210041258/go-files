package testutils

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httputil"
	"time"
)

// Config holds settings for a customizable HTTP client.
// Zero values are replaced with sensible defaults.
type Config struct {
	// General client timeout (0 = default 30s)
	Timeout time.Duration

	// Dialer settings
	DialTimeout time.Duration
	KeepAlive   time.Duration

	// Transport settings
	MaxIdleConns          int
	IdleConnTimeout       time.Duration
	TLSHandshakeTimeout   time.Duration
	ExpectContinueTimeout time.Duration
	MaxIdleConnsPerHost   int
	MaxConnsPerHost       int
	DisableCompression    bool

	// TLS
	InsecureSkipVerify bool

	// Redirect Policy
	CheckRedirect func(req *http.Request, via []*http.Request) error

	// Middleware
	Logger       Logger // optional logging
	DumpRequests bool   // if true, dump full request/response
	MetricsHook  MetricsHook

	// State
	Jar http.CookieJar
}

// Logger defines the interface for logging HTTP traffic.
type Logger interface {
	Printf(format string, args ...interface{})
}

// MetricsHook allows custom metrics collection per request.
type MetricsHook func(req *http.Request, resp *http.Response, duration time.Duration, err error)

// NewClient creates an http.Client with sane defaults and optional middleware.
func NewClient(cfg *Config) *http.Client {
	if cfg == nil {
		cfg = &Config{}
	}

	// --- 1. Apply defaults ---
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.DialTimeout <= 0 {
		cfg.DialTimeout = 30 * time.Second
	}
	if cfg.KeepAlive <= 0 {
		cfg.KeepAlive = 30 * time.Second
	}
	if cfg.MaxIdleConns <= 0 {
		cfg.MaxIdleConns = 100
	}
	if cfg.IdleConnTimeout <= 0 {
		cfg.IdleConnTimeout = 90 * time.Second
	}
	if cfg.TLSHandshakeTimeout <= 0 {
		cfg.TLSHandshakeTimeout = 10 * time.Second
	}
	if cfg.ExpectContinueTimeout <= 0 {
		cfg.ExpectContinueTimeout = 1 * time.Second
	}
	if cfg.MaxIdleConnsPerHost <= 0 {
		cfg.MaxIdleConnsPerHost = 10
	}
	// MaxConnsPerHost defaults to 0 (unlimited) if not set

	// --- 2. Construct Transport ---
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   cfg.DialTimeout,
			KeepAlive: cfg.KeepAlive,
		}).DialContext,
		MaxIdleConns:          cfg.MaxIdleConns,
		IdleConnTimeout:       cfg.IdleConnTimeout,
		TLSHandshakeTimeout:   cfg.TLSHandshakeTimeout,
		ExpectContinueTimeout: cfg.ExpectContinueTimeout,
		MaxIdleConnsPerHost:   cfg.MaxIdleConnsPerHost,
		MaxConnsPerHost:       cfg.MaxConnsPerHost,
		DisableCompression:    cfg.DisableCompression,
		ForceAttemptHTTP2:     true,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.InsecureSkipVerify,
		},
	}

	// --- 3. Wrap Transport with Logging + Metrics ---
	var roundTripper http.RoundTripper = transport
	if cfg.Logger != nil || cfg.MetricsHook != nil || cfg.DumpRequests {
		roundTripper = &middlewareRoundTripper{
			next:         transport,
			logger:       cfg.Logger,
			metricsHook:  cfg.MetricsHook,
			dumpRequests: cfg.DumpRequests,
		}
	}

	// --- 4. Assemble Client ---
	return &http.Client{
		Timeout:       cfg.Timeout,
		Transport:     roundTripper,
		CheckRedirect: cfg.CheckRedirect,
		Jar:           cfg.Jar,
	}
}

// middlewareRoundTripper wraps a RoundTripper to add logging, dumps, and metrics.
type middlewareRoundTripper struct {
	next         http.RoundTripper
	logger       Logger
	metricsHook  MetricsHook
	dumpRequests bool
}

// RoundTrip implements http.RoundTripper.
func (mrt *middlewareRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()

	// Optional request dump
	if mrt.dumpRequests && mrt.logger != nil {
		if dump, err := httputil.DumpRequestOut(req, true); err == nil {
			mrt.logger.Printf("[HTTP Client] --> Request:\n%s", dump)
		} else {
			mrt.logger.Printf("[HTTP Client] --> Request dump error: %v", err)
		}
	}

	if mrt.logger != nil {
		mrt.logger.Printf("[HTTP Client] --> %s %s", req.Method, req.URL.String())
	}

	resp, err := mrt.next.RoundTrip(req)
	duration := time.Since(start)

	if mrt.logger != nil {
		if err != nil {
			mrt.logger.Printf("[HTTP Client] <-- ERROR: %v (took %s)", err, duration)
		} else {
			mrt.logger.Printf("[HTTP Client] <-- %s %s %d (took %s)", resp.Proto, resp.Request.URL.String(), resp.StatusCode, duration)
		}
	}

	// Call metrics hook if set
	if mrt.metricsHook != nil {
		mrt.metricsHook(req, resp, duration, err)
	}

	return resp, err
}
