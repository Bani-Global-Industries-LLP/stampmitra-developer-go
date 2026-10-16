package stampmitra

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// Sentinel errors for use with errors.Is. Every error returned by this package matches exactly one
// of them (or is a plain context error when the caller cancelled the context).
var (
	// ErrValidation matches local input validation failures and HTTP 400 responses.
	ErrValidation = errors.New("stampmitra: validation error")
	// ErrAuthentication matches HTTP 401 responses.
	ErrAuthentication = errors.New("stampmitra: authentication error")
	// ErrPaymentRequired matches HTTP 402 responses.
	ErrPaymentRequired = errors.New("stampmitra: payment required")
	// ErrPermission matches HTTP 403 responses.
	ErrPermission = errors.New("stampmitra: permission denied")
	// ErrNotFound matches HTTP 404 responses.
	ErrNotFound = errors.New("stampmitra: not found")
	// ErrConflict matches HTTP 409 responses.
	ErrConflict = errors.New("stampmitra: conflict")
	// ErrRateLimit matches HTTP 429 responses.
	ErrRateLimit = errors.New("stampmitra: rate limited")
	// ErrUpstream matches HTTP 5xx responses.
	ErrUpstream = errors.New("stampmitra: upstream error")
	// ErrConnection matches network failures and timeouts (no usable response was received).
	ErrConnection = errors.New("stampmitra: connection error")
	// ErrTimeout matches request timeouts. A *ConnectionError that timed out matches both
	// ErrTimeout and ErrConnection.
	ErrTimeout = errors.New("stampmitra: timeout")
	// ErrConfig matches invalid client configuration (bad API key format, base URL, ...).
	ErrConfig = errors.New("stampmitra: invalid configuration")
	// ErrSignature matches webhook signature verification failures.
	ErrSignature = errors.New("stampmitra: webhook signature verification failed")
	// ErrDecode matches a 2xx response whose body could not be decoded.
	ErrDecode = errors.New("stampmitra: cannot decode response")
)

// APIError is returned for every non-2xx HTTP response. Use errors.As to obtain it and errors.Is
// with the Err* sentinels (or the Is* helpers) to classify it.
//
// The API key is never part of an APIError.
type APIError struct {
	// Status is the HTTP status code.
	Status int
	// Code is the machine-readable error code from the body (for example "NOT_FOUND"); empty when
	// the body was not the documented error envelope.
	Code string
	// Message is the human-readable message from the body, verbatim (a 403 about an IP allowlist
	// is surfaced exactly as the server sent it). For non-JSON bodies it is "HTTP <status>".
	Message string
	// RequestID is the requestId from the error body (or the X-Request-Id header when present).
	RequestID string
	// Field names the offending input for validation errors, when the server says so.
	Field string
	// RetryAfter is the parsed Retry-After header (429 and 503); zero when absent.
	RetryAfter time.Duration
	// Body is the raw response body, truncated to 64 KiB. It may contain personal data: do not log it blindly.
	Body []byte
	// Attempts is how many HTTP attempts were made before giving up.
	Attempts int
}

// Error implements error.
func (e *APIError) Error() string {
	s := "stampmitra: HTTP " + strconv.Itoa(e.Status)
	if e.Code != "" {
		s += " " + e.Code
	}
	if e.Message != "" && e.Message != "HTTP "+strconv.Itoa(e.Status) {
		s += ": " + e.Message
	}
	if e.Field != "" {
		s += " (field " + e.Field + ")"
	}
	if e.RequestID != "" {
		s += " [request_id=" + e.RequestID + "]"
	}
	return s
}

// Unwrap returns the sentinel matching the status so errors.Is(err, ErrNotFound) works.
func (e *APIError) Unwrap() error { return sentinelForStatus(e.Status) }

func sentinelForStatus(status int) error {
	switch {
	case status == http.StatusBadRequest:
		return ErrValidation
	case status == http.StatusUnauthorized:
		return ErrAuthentication
	case status == http.StatusPaymentRequired:
		return ErrPaymentRequired
	case status == http.StatusForbidden:
		return ErrPermission
	case status == http.StatusNotFound:
		return ErrNotFound
	case status == http.StatusConflict:
		return ErrConflict
	case status == http.StatusTooManyRequests:
		return ErrRateLimit
	case status >= 500:
		return ErrUpstream
	}
	return nil
}

// ValidationError is returned before any network call when input fails client-side validation.
// It matches ErrValidation. Its message never contains the rejected value.
type ValidationError struct {
	Field   string
	Message string
}

// Error implements error.
func (e *ValidationError) Error() string {
	if e.Field == "" {
		return "stampmitra: invalid input: " + e.Message
	}
	return "stampmitra: invalid " + e.Field + ": " + e.Message
}

// Is makes errors.Is(err, ErrValidation) true.
func (e *ValidationError) Is(target error) bool { return target == ErrValidation }

// ConnectionError wraps a transport failure: DNS, refused or dropped connections, TLS failures and
// timeouts. It matches ErrConnection, and ErrTimeout when Timeout is true.
type ConnectionError struct {
	// Timeout reports that the per-attempt timeout elapsed.
	Timeout bool
	// Err is the underlying error.
	Err error
	// Attempts is how many HTTP attempts were made.
	Attempts int
}

// Error implements error.
func (e *ConnectionError) Error() string {
	kind := "connection error"
	if e.Timeout {
		kind = "timeout"
	}
	return fmt.Sprintf("stampmitra: %s: %v", kind, e.Err)
}

// Unwrap exposes the underlying error.
func (e *ConnectionError) Unwrap() error { return e.Err }

// Is matches ErrConnection and, for timeouts, ErrTimeout.
func (e *ConnectionError) Is(target error) bool {
	return target == ErrConnection || (e.Timeout && target == ErrTimeout)
}

// ConfigError reports invalid client configuration. It matches ErrConfig. Messages never contain the API key.
type ConfigError struct{ Message string }

// Error implements error.
func (e *ConfigError) Error() string { return "stampmitra: invalid configuration: " + e.Message }

// Is makes errors.Is(err, ErrConfig) true.
func (e *ConfigError) Is(target error) bool { return target == ErrConfig }

// DecodeError reports a 2xx response whose body was not valid JSON for the expected type.
type DecodeError struct {
	Status int
	Err    error
}

// Error implements error.
func (e *DecodeError) Error() string {
	return fmt.Sprintf("stampmitra: cannot decode HTTP %d response: %v", e.Status, e.Err)
}

// Unwrap exposes the underlying error.
func (e *DecodeError) Unwrap() error { return e.Err }

// Is makes errors.Is(err, ErrDecode) true.
func (e *DecodeError) Is(target error) bool { return target == ErrDecode }

// IsValidation reports whether err is a validation error (local or HTTP 400).
func IsValidation(err error) bool { return errors.Is(err, ErrValidation) }

// IsAuthentication reports whether err is an HTTP 401.
func IsAuthentication(err error) bool { return errors.Is(err, ErrAuthentication) }

// IsPaymentRequired reports whether err is an HTTP 402.
func IsPaymentRequired(err error) bool { return errors.Is(err, ErrPaymentRequired) }

// IsPermission reports whether err is an HTTP 403.
func IsPermission(err error) bool { return errors.Is(err, ErrPermission) }

// IsNotFound reports whether err is an HTTP 404.
func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }

// IsConflict reports whether err is an HTTP 409.
func IsConflict(err error) bool { return errors.Is(err, ErrConflict) }

// IsRateLimit reports whether err is an HTTP 429.
func IsRateLimit(err error) bool { return errors.Is(err, ErrRateLimit) }

// IsUpstream reports whether err is an HTTP 5xx.
func IsUpstream(err error) bool { return errors.Is(err, ErrUpstream) }

// IsConnection reports whether err is a network failure or timeout.
func IsConnection(err error) bool { return errors.Is(err, ErrConnection) }

// IsTimeout reports whether err is a request timeout.
func IsTimeout(err error) bool { return errors.Is(err, ErrTimeout) }

// RequestID returns the request id carried by err when it is (or wraps) an *APIError, else "".
func RequestID(err error) string {
	var api *APIError
	if errors.As(err, &api) {
		return api.RequestID
	}
	return ""
}
