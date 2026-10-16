package stampmitra

import (
	"net/http"
	"time"

	"log/slog"
)

// Option configures a Client. Pass options to NewClient.
type Option func(*clientConfig)

type clientConfig struct {
	baseURL        string
	httpClient     *http.Client
	timeout        time.Duration
	maxRetries     int
	defaultHeaders http.Header
	appInfo        *AppInfo
	logger         *slog.Logger
	validateInput  bool
	allowCustomKey bool
	backoffBase    time.Duration
	backoffCap     time.Duration
}

// AppInfo identifies your application in the User-Agent header.
type AppInfo struct {
	Name    string
	Version string
	URL     string
}

// WithBaseURL overrides the API base URL (default https://api-b2b.stampmitra.in). It must use
// https (plain http is accepted only for localhost, 127.0.0.1 and ::1), carry no credentials,
// and have no query or fragment.
func WithBaseURL(u string) Option { return func(c *clientConfig) { c.baseURL = u } }

// WithHTTPClient supplies the HTTP client. The SDK works on a shallow copy whose redirect policy
// never follows redirects (so credentials cannot be forwarded to another host); your client's
// Transport, Jar and Timeout are kept. The per-attempt timeout set by WithTimeout is applied
// through the request context in addition to any Timeout on this client.
func WithHTTPClient(h *http.Client) Option { return func(c *clientConfig) { c.httpClient = h } }

// WithTimeout sets the per-attempt timeout (default 30s). Use WithRequestTimeout to change it for one call.
func WithTimeout(d time.Duration) Option { return func(c *clientConfig) { c.timeout = d } }

// WithMaxRetries sets the default number of automatic retries (default 2, so at most 3 attempts).
// Zero disables retries.
func WithMaxRetries(n int) Option { return func(c *clientConfig) { c.maxRetries = n } }

// WithBackoff overrides the retry backoff: delays are drawn uniformly from [0, min(max, base*2^attempt)]
// ("full jitter"). Defaults are 500ms and 8s.
func WithBackoff(base, max time.Duration) Option {
	return func(c *clientConfig) { c.backoffBase, c.backoffCap = base, max }
}

// WithDefaultHeader adds a header sent on every request. Authorization and X-API-Key cannot be set this way.
func WithDefaultHeader(key, value string) Option {
	return func(c *clientConfig) {
		if c.defaultHeaders == nil {
			c.defaultHeaders = http.Header{}
		}
		c.defaultHeaders.Add(key, value)
	}
}

// WithAppInfo appends your application name, version and URL to the User-Agent.
func WithAppInfo(a AppInfo) Option { return func(c *clientConfig) { c.appInfo = &a } }

// WithLogger enables debug logging of requests and responses to l (at slog.LevelDebug).
// Authorization and X-API-Key are redacted and identifiers and personal data in bodies are masked.
// Nothing is logged by default.
func WithLogger(l *slog.Logger) Option { return func(c *clientConfig) { c.logger = l } }

// WithValidateInput turns client-side input validation (and normalization) on or off. It is on by default.
func WithValidateInput(on bool) Option { return func(c *clientConfig) { c.validateInput = on } }

// WithAllowCustomKey accepts API keys that do not match ^sk_(test|live)_[A-Za-z0-9_-]{8,}$.
func WithAllowCustomKey() Option { return func(c *clientConfig) { c.allowCustomKey = true } }

// RequestOption customizes a single call. Pass it as the trailing arguments of any method.
type RequestOption func(*requestConfig)

type requestConfig struct {
	timeout    *time.Duration
	maxRetries *int
	idemKey    string
	headers    http.Header
	raw        *Response
}

// WithRequestTimeout overrides the per-attempt timeout for this call.
func WithRequestTimeout(d time.Duration) RequestOption {
	return func(r *requestConfig) { r.timeout = &d }
}

// WithRequestMaxRetries overrides the retry count for this call.
func WithRequestMaxRetries(n int) RequestOption { return func(r *requestConfig) { r.maxRetries = &n } }

// WithIdempotencyKey sets the Idempotency-Key for a POST. The same key is reused on every automatic
// retry. Without it the SDK generates a random UUIDv4 for every POST except webhook replay.
func WithIdempotencyKey(key string) RequestOption { return func(r *requestConfig) { r.idemKey = key } }

// WithRequestHeader adds a header to this call only. Authorization and X-API-Key cannot be set this way.
func WithRequestHeader(key, value string) RequestOption {
	return func(r *requestConfig) {
		if r.headers == nil {
			r.headers = http.Header{}
		}
		r.headers.Add(key, value)
	}
}

// WithRawResponse captures HTTP metadata (status, headers, request id, attempts, raw body) of the
// final response into out, for successful and failed calls alike.
func WithRawResponse(out *Response) RequestOption { return func(r *requestConfig) { r.raw = out } }

// Response is the raw HTTP outcome of a call, captured with WithRawResponse.
type Response struct {
	// StatusCode is the final HTTP status; 0 when no response was received.
	StatusCode int
	// Header holds the response headers.
	Header http.Header
	// RequestID is the X-Request-Id header or, for errors, the requestId from the error body.
	RequestID string
	// Attempts is how many HTTP attempts were made.
	Attempts int
	// Body is the raw response body.
	Body []byte
}
