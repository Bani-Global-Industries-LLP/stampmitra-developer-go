package stampmitra

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// DefaultBaseURL is the production and sandbox API host; the key prefix selects the environment.
const DefaultBaseURL = "https://api-b2b.stampmitra.in"

// EnvAPIKey is the environment variable NewClient reads when apiKey is empty.
const EnvAPIKey = "STAMPMITRA_API_KEY" //nolint:gosec // the name of an environment variable, not a credential

var keyPattern = regexp.MustCompile(`^sk_(test|live)_[A-Za-z0-9_-]{8,}$`)

// Client is the StampMitra API client. It is safe for concurrent use. Create it with NewClient and
// reuse it. Printing a Client (any fmt verb) or logging it never reveals the API key.
type Client struct {
	// Environment is "test" for sk_test_ keys, "live" for sk_live_ keys and "custom" otherwise.
	Environment string

	Verify   *VerifyService
	Services *ServicesService
	Estamp   *EstampService
	Orders   *OrdersService
	Payments *PaymentsService
	Usage    *UsageService
	Logs     *LogsService
	Webhooks *WebhooksService

	apiKey     secret
	baseURL    string
	http       *http.Client
	timeout    time.Duration
	maxRetries int
	headers    http.Header
	userAgent  string
	logger     *slog.Logger
	validate   bool
	backoff    [2]time.Duration

	// test seams
	sleep  func(ctx context.Context, d time.Duration) error
	jitter func(max time.Duration) time.Duration
	now    func() time.Time
}

// NewClient creates a client. If apiKey is empty the STAMPMITRA_API_KEY environment variable is used.
// Keys must look like sk_test_... or sk_live_... unless WithAllowCustomKey is given.
//
// The API key is for server-side use only: never ship sk_ keys in mobile apps, browser code or public repositories.
func NewClient(apiKey string, opts ...Option) (*Client, error) {
	cfg := clientConfig{
		baseURL:       DefaultBaseURL,
		timeout:       30 * time.Second,
		maxRetries:    2,
		validateInput: true,
		backoffBase:   500 * time.Millisecond,
		backoffCap:    8 * time.Second,
	}
	for _, o := range opts {
		if o != nil {
			o(&cfg)
		}
	}
	if apiKey == "" {
		apiKey = os.Getenv(EnvAPIKey)
	}
	if apiKey == "" {
		return nil, &ConfigError{Message: "no API key: pass one to NewClient or set " + EnvAPIKey}
	}
	if strings.ContainsAny(apiKey, "\r\n\x00 \t") {
		return nil, &ConfigError{Message: "API key contains whitespace or control characters"}
	}
	env := "custom"
	switch {
	case keyPattern.MatchString(apiKey):
		env = strings.SplitN(apiKey, "_", 3)[1]
	case !cfg.allowCustomKey:
		return nil, &ConfigError{Message: "API key must match ^sk_(test|live)_[A-Za-z0-9_-]{8,}$ (use WithAllowCustomKey to accept other formats)"}
	}
	base, err := validateBaseURL(cfg.baseURL)
	if err != nil {
		return nil, err
	}
	if cfg.maxRetries < 0 {
		return nil, &ConfigError{Message: "max retries must not be negative"}
	}
	if cfg.timeout <= 0 {
		return nil, &ConfigError{Message: "timeout must be positive"}
	}
	for k := range cfg.defaultHeaders {
		if err := checkHeader(k, cfg.defaultHeaders.Get(k)); err != nil {
			return nil, &ConfigError{Message: err.Error()}
		}
	}

	hc := &http.Client{}
	if cfg.httpClient != nil {
		cp := *cfg.httpClient
		hc = &cp
	}
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	c := &Client{
		Environment: env,
		apiKey:      secret(apiKey),
		baseURL:     base,
		http:        hc,
		timeout:     cfg.timeout,
		maxRetries:  cfg.maxRetries,
		headers:     cfg.defaultHeaders,
		userAgent:   buildUserAgent(cfg.appInfo),
		validate:    cfg.validateInput,
		backoff:     [2]time.Duration{cfg.backoffBase, cfg.backoffCap},
		sleep:       sleepContext,
		jitter:      randomJitter,
		now:         time.Now,
	}
	c.logger = cfg.logger
	c.Verify = &VerifyService{c}
	c.Services = &ServicesService{c}
	c.Estamp = &EstampService{c}
	c.Orders = &OrdersService{c}
	c.Payments = &PaymentsService{Intents: &PaymentIntentsService{c}}
	c.Usage = &UsageService{c}
	c.Logs = &LogsService{c}
	c.Webhooks = &WebhooksService{Events: &WebhookEventsService{c}}
	return c, nil
}

func validateBaseURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.Hostname() == "" {
		return "", &ConfigError{Message: "base URL is not a valid absolute URL"}
	}
	if u.User != nil {
		return "", &ConfigError{Message: "base URL must not contain credentials"}
	}
	if u.RawQuery != "" || u.Fragment != "" || strings.Contains(raw, "?") || strings.Contains(raw, "#") {
		return "", &ConfigError{Message: "base URL must not contain a query or fragment"}
	}
	switch u.Scheme {
	case "https":
	case "http":
		switch u.Hostname() {
		case "localhost", "127.0.0.1", "::1":
		default:
			return "", &ConfigError{Message: "base URL must use https (http is allowed only for localhost, 127.0.0.1 and ::1)"}
		}
	default:
		return "", &ConfigError{Message: "base URL must use https"}
	}
	return strings.TrimRight(u.String(), "/"), nil
}

func buildUserAgent(a *AppInfo) string {
	ua := fmt.Sprintf("stampmitra-go/%s (%s; %s/%s)", Version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
	if a != nil && a.Name != "" {
		app := sanitizeUA(a.Name)
		if a.Version != "" {
			app += "/" + sanitizeUA(a.Version)
		}
		if a.URL != "" {
			app += " (" + sanitizeUA(a.URL) + ")"
		}
		ua = app + " " + ua
	}
	return ua
}

func sanitizeUA(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || r > 0x7e {
			return -1
		}
		return r
	}, s)
}

// secret is a string that refuses to print itself.
type secret string

func (secret) String() string   { return redacted }
func (secret) GoString() string { return `"` + redacted + `"` }

// Format implements fmt.Formatter so no verb reveals the value.
func (secret) Format(f fmt.State, _ rune) { _, _ = f.Write([]byte(redacted)) }

// MarshalText keeps the value out of JSON and text encodings.
func (secret) MarshalText() ([]byte, error) { return []byte(redacted), nil }

// String describes the client without the API key.
func (c Client) String() string {
	return fmt.Sprintf("stampmitra.Client{environment: %s, baseURL: %s, apiKey: %s}", c.Environment, c.baseURL, redacted)
}

// GoString implements fmt.GoStringer.
func (c Client) GoString() string { return c.String() }

// Format implements fmt.Formatter so that %v, %+v, %#v and %s all print the redacted description.
func (c Client) Format(f fmt.State, _ rune) { _, _ = f.Write([]byte(c.String())) }
