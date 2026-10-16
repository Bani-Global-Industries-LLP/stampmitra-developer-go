package stampmitra

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	mrand "math/rand"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	maxResponseBytes = 16 << 20 // 16 MiB
	maxErrorBody     = 64 << 10
	maxRetryAfter    = 60 * time.Second
)

// pathEscape escapes one path segment.
func pathEscape(s string) string { return url.PathEscape(s) }

// newUUIDv4 returns a random (version 4) UUID from the operating system's CSPRNG. It never falls
// back to a weak generator: if the system cannot provide randomness the call fails.
func newUUIDv4() (string, error) {
	var b [16]byte
	if _, err := io.ReadFull(rand.Reader, b[:]); err != nil {
		return "", fmt.Errorf("stampmitra: cannot generate idempotency key (no secure random source): %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	var out [36]byte
	hex.Encode(out[0:8], b[0:4])
	out[8] = '-'
	hex.Encode(out[9:13], b[4:6])
	out[13] = '-'
	hex.Encode(out[14:18], b[6:8])
	out[18] = '-'
	hex.Encode(out[19:23], b[8:10])
	out[23] = '-'
	hex.Encode(out[24:], b[10:])
	return string(out[:]), nil
}

func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// randomJitter returns a uniform duration in [0, max]. Jitter does not need a secure RNG.
func randomJitter(max time.Duration) time.Duration {
	if max <= 0 {
		return 0
	}
	return time.Duration(mrand.Int63n(int64(max) + 1)) //nolint:gosec // jitter only
}

func checkHeader(k, v string) error {
	if k == "" || strings.ContainsAny(k, " \t\r\n:") {
		return errors.New("invalid header name")
	}
	switch strings.ToLower(k) {
	case "authorization", "x-api-key", "host", "content-length", "proxy-authorization":
		return fmt.Errorf("header %q is managed by the SDK and cannot be set", k)
	}
	if strings.ContainsAny(v, "\r\n\x00") {
		return fmt.Errorf("invalid value for header %q", k)
	}
	return nil
}

// call performs one API call and decodes the JSON body into a new T.
func call[T any](ctx context.Context, c *Client, method, path string, query url.Values, body any, autoKey bool, opts []RequestOption) (*T, error) {
	var out T
	if err := c.do(ctx, method, path, query, body, &out, autoKey, opts); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out any, autoKey bool, opts []RequestOption) error {
	if ctx == nil {
		return &ValidationError{Message: "context must not be nil"}
	}
	var rc requestConfig
	for _, o := range opts {
		if o != nil {
			o(&rc)
		}
	}
	if rc.raw != nil {
		*rc.raw = Response{}
	}
	for k := range rc.headers {
		if err := checkHeader(k, rc.headers.Get(k)); err != nil {
			return &ValidationError{Field: "header", Message: err.Error()}
		}
	}
	timeout := c.timeout
	if rc.timeout != nil {
		timeout = *rc.timeout
	}
	if timeout <= 0 {
		return &ValidationError{Field: "timeout", Message: "must be positive"}
	}
	maxRetries := c.maxRetries
	if rc.maxRetries != nil {
		maxRetries = *rc.maxRetries
	}
	if maxRetries < 0 {
		maxRetries = 0
	}

	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return &ValidationError{Message: "request body cannot be encoded as JSON: " + err.Error()}
		}
	}

	key := rc.idemKey
	if key != "" && strings.ContainsAny(key, "\r\n\x00") {
		return &ValidationError{Field: "idempotencyKey", Message: "contains invalid characters"}
	}
	if key == "" && method == http.MethodPost && autoKey {
		var err error
		if key, err = newUUIDv4(); err != nil {
			return err
		}
	}
	// GET is always retry-safe; a POST is retried only when it carries an Idempotency-Key, which
	// the server honours by replaying the original result instead of repeating (or billing) the work.
	canRetry := method == http.MethodGet || key != ""

	target := c.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	for attempt := 0; ; attempt++ {
		status, hdr, respBody, err := c.attempt(ctx, method, target, payload, key, &rc, timeout, attempt+1)
		var retryAfter time.Duration
		if err == nil {
			if status >= 200 && status < 300 {
				c.captureRaw(&rc, status, hdr, respBody, attempt+1, hdr.Get("X-Request-Id"))
				if out == nil {
					return nil
				}
				if len(bytes.TrimSpace(respBody)) == 0 {
					return &DecodeError{Status: status, Err: errors.New("empty response body")}
				}
				if derr := json.Unmarshal(respBody, out); derr != nil {
					return &DecodeError{Status: status, Err: derr}
				}
				return nil
			}
			apiErr := newAPIError(status, hdr, respBody, attempt+1)
			retryAfter = apiErr.RetryAfter
			err = apiErr
			c.captureRaw(&rc, status, hdr, respBody, attempt+1, apiErr.RequestID)
			if !isRetryableStatus(status) {
				return err
			}
		} else {
			var ce *ConnectionError
			if !errors.As(err, &ce) {
				return err // context cancelled by the caller, or other non-retryable failure
			}
			ce.Attempts = attempt + 1
			c.captureRaw(&rc, 0, nil, nil, attempt+1, "")
		}
		if !canRetry || attempt >= maxRetries {
			return err
		}
		delay := c.retryDelay(attempt, retryAfter)
		c.logf(ctx, slog.LevelDebug, "stampmitra: retrying", "method", method, "path", path, "attempt", attempt+1, "delay", delay.String())
		if serr := c.sleep(ctx, delay); serr != nil {
			return serr
		}
	}
}

func (c *Client) captureRaw(rc *requestConfig, status int, hdr http.Header, body []byte, attempts int, requestID string) {
	if rc.raw == nil {
		return
	}
	*rc.raw = Response{StatusCode: status, Header: hdr.Clone(), Attempts: attempts, RequestID: requestID, Body: append([]byte(nil), body...)}
}

func isRetryableStatus(s int) bool {
	return s == http.StatusTooManyRequests || s == http.StatusBadGateway || s == http.StatusServiceUnavailable || s == http.StatusGatewayTimeout
}

func (c *Client) retryDelay(attempt int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		if retryAfter > maxRetryAfter {
			return maxRetryAfter
		}
		return retryAfter
	}
	base, capD := c.backoff[0], c.backoff[1]
	ceil := capD
	if attempt < 30 {
		if v := time.Duration(float64(base) * math.Pow(2, float64(attempt))); v > 0 && v < capD {
			ceil = v
		}
	}
	return c.jitter(ceil)
}

func parseRetryAfter(v string, now time.Time) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if secs, err := strconv.ParseFloat(v, 64); err == nil {
		if secs <= 0 || math.IsNaN(secs) || math.IsInf(secs, 0) {
			return 0
		}
		if secs > maxRetryAfter.Seconds() {
			return maxRetryAfter
		}
		return time.Duration(secs * float64(time.Second))
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := t.Sub(now); d > 0 {
			if d > maxRetryAfter {
				return maxRetryAfter
			}
			return d
		}
	}
	return 0
}

// attempt performs one HTTP round trip. It returns a *ConnectionError for transport failures and
// the context's error when the caller's context ended.
func (c *Client) attempt(ctx context.Context, method, target string, payload []byte, key string, rc *requestConfig, timeout time.Duration, n int) (int, http.Header, []byte, error) {
	actx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var rdr io.Reader
	if payload != nil {
		rdr = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(actx, method, target, rdr)
	if err != nil {
		return 0, nil, nil, &ValidationError{Message: "cannot build request"}
	}
	for k, vs := range c.headers {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	for k, vs := range rc.headers {
		req.Header.Del(k)
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	req.Header.Set("Authorization", "Bearer "+string(c.apiKey))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	start := c.now()
	if c.logger != nil && c.logger.Enabled(ctx, slog.LevelDebug) {
		c.logger.LogAttrs(ctx, slog.LevelDebug, "stampmitra request", slog.String("method", method), slog.String("url", target),
			slog.Int("attempt", n), slog.Any("headers", redactHeaders(req.Header)), slog.String("body", RedactBody(payload)))
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, nil, c.transportError(ctx, actx, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return 0, nil, nil, c.transportError(ctx, actx, err)
	}
	if len(body) > maxResponseBytes {
		return 0, nil, nil, &DecodeError{Status: resp.StatusCode, Err: errors.New("response body exceeds 16 MiB")}
	}
	if c.logger != nil && c.logger.Enabled(ctx, slog.LevelDebug) {
		c.logger.LogAttrs(ctx, slog.LevelDebug, "stampmitra response", slog.Int("status", resp.StatusCode), slog.Int("attempt", n),
			slog.Duration("elapsed", c.now().Sub(start)), slog.Any("headers", redactHeaders(resp.Header)), slog.String("body", RedactBody(body)))
	}
	return resp.StatusCode, resp.Header, body, nil
}

// transportError classifies a failed round trip. A caller-cancelled context is returned as is.
func (c *Client) transportError(parent, attemptCtx context.Context, err error) error {
	if perr := parent.Err(); perr != nil {
		return perr
	}
	timedOut := errors.Is(attemptCtx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded)
	var ne net.Error
	if !timedOut && errors.As(err, &ne) && ne.Timeout() {
		timedOut = true
	}
	return &ConnectionError{Timeout: timedOut, Err: sanitizeTransportErr(err)}
}

// sanitizeTransportErr keeps the cause but drops the request URL's query, which can carry filters.
func sanitizeTransportErr(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return fmt.Errorf("%s: %w", ue.Op, ue.Err)
	}
	return err
}

func newAPIError(status int, hdr http.Header, body []byte, attempts int) *APIError {
	e := &APIError{Status: status, Attempts: attempts, Message: "HTTP " + strconv.Itoa(status), RequestID: hdr.Get("X-Request-Id")}
	e.RetryAfter = parseRetryAfter(hdr.Get("Retry-After"), time.Now())
	if len(body) > maxErrorBody {
		body = body[:maxErrorBody]
	}
	e.Body = append([]byte(nil), body...)
	var env struct {
		Error *struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"requestId"`
			Field     string `json:"field"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &env) == nil && env.Error != nil {
		e.Code = env.Error.Code
		if env.Error.Message != "" {
			e.Message = env.Error.Message
		}
		if env.Error.RequestID != "" {
			e.RequestID = env.Error.RequestID
		}
		e.Field = env.Error.Field
	}
	return e
}

func (c *Client) logf(ctx context.Context, lvl slog.Level, msg string, args ...any) {
	if c.logger != nil && c.logger.Enabled(ctx, lvl) {
		c.logger.Log(ctx, lvl, msg, args...)
	}
}
