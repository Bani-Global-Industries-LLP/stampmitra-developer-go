package stampmitra

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"strings"
	"testing"
)

func TestRedactBody(t *testing.T) {
	in := `{"pan":"ABCDE1234F","name":"Rahul Kumar","gstin":"27ABCDE1234F1Z5","accountNumber":"000123456789","ifsc":"ICIC0001234",` +
		`"directors":[{"din":"01234567","name":"A B"}],"referenceId":"SMV-1","valid":true,"dob":"1990-01-31","downloadUrl":"https://x/y?token=abc","phone":"9876543210","short":"x","pan2":null}`
	out := RedactBody([]byte(in))
	for _, secret := range []string{"ABCDE1234F", "Rahul", "27ABCDE1234F1Z5", "000123456789", "ICIC0001234", "01234567", "A B", "1990-01-31", "token=abc", "9876543210"} {
		if strings.Contains(out, secret) {
			t.Errorf("redacted body still contains %q: %s", secret, out)
		}
	}
	for _, keep := range []string{"SMV-1", `"valid":true`, "1234F"[1:]} {
		if !strings.Contains(out, keep) {
			t.Errorf("redacted body lost %q: %s", keep, out)
		}
	}
	if !strings.Contains(out, "******234F") {
		t.Errorf("identifier should keep last four characters: %s", out)
	}
	if got := RedactBody([]byte("<html>sk_test_x ABCDE1234F</html>")); strings.Contains(got, "ABCDE") || !strings.HasPrefix(got, "[non-JSON body") {
		t.Errorf("non-JSON body: %s", got)
	}
	if RedactBody(nil) != "" {
		t.Error("empty body")
	}
}

func TestDebugLoggingRedactsEverything(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
		if n == 0 {
			writeErr(w, 503, "X", "retry me")
			return
		}
		writeJSON(w, 200, `{"referenceId":"SMV-1","valid":true,"registeredName":"Rahul Kumar","pan":"ABCDE1234F"}`)
	}, WithLogger(logger), WithDefaultHeader("X-Trace", "visible-trace"))
	_, err := env.c.Verify.PAN(context.Background(), &PANVerifyParams{PAN: "ABCDE1234F", Name: "Rahul Kumar"}, WithIdempotencyKey("idem-1"))
	if err != nil {
		t.Fatal(err)
	}
	logs := buf.String()
	if !strings.Contains(logs, "stampmitra request") || !strings.Contains(logs, "stampmitra response") || !strings.Contains(logs, "retrying") {
		t.Fatalf("expected request/response/retry log lines:\n%s", logs)
	}
	for _, leak := range []string{testKey, "abcdefgh12345678", "Bearer", "ABCDE1234F", "Rahul"} {
		if strings.Contains(logs, leak) {
			t.Errorf("log output leaks %q:\n%s", leak, logs)
		}
	}
	if !strings.Contains(logs, "REDACTED") || !strings.Contains(logs, "visible-trace") {
		t.Errorf("headers should be redacted selectively:\n%s", logs)
	}
}

func TestNothingLoggedByDefault(t *testing.T) {
	// no logger configured: the call must simply work (nothing to capture); also an Info-level logger gets no debug lines
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, `[]`) }, WithLogger(logger))
	if _, err := env.c.Services.List(context.Background()); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Fatalf("unexpected output: %s", buf.String())
	}
}

func TestErrorsAndRawNeverContainKey(t *testing.T) {
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
		writeErr(w, 401, "UNAUTHORIZED", "Missing or invalid API key.")
	})
	var raw Response
	_, err := env.c.Usage.Summary(context.Background(), WithRawResponse(&raw))
	for _, s := range []string{err.Error(), strings.Join(flatten(raw.Header), ";")} {
		if strings.Contains(s, testKey) {
			t.Fatalf("leak: %s", s)
		}
	}
}

func flatten(h http.Header) []string {
	var out []string
	for k, v := range h {
		out = append(out, k+"="+strings.Join(v, ","))
	}
	return out
}
