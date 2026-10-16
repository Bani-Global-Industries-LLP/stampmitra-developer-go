package main

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	stampmitra "github.com/Bani-Global-Industries-LLP/stampmitra-developer-go"
)

func TestHandler(t *testing.T) {
	secret := "whsec_test"
	body := `{"coreOrderId":"c1","coreEventType":"DOCUMENT_READY","occurredAt":"2026-09-25T09:05:00.000Z"}`
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	srv := httptest.NewServer(handler(secret))
	defer srv.Close()

	do := func(sig string) int {
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/webhooks/stampmitra", strings.NewReader(body))
		req.Header.Set("X-Webhook-Signature", sig)
		req.Header.Set("X-Webhook-Timestamp", ts)
		req.Header.Set("X-Webhook-Event-Id", "evt_1")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if got := do(stampmitra.SignPayload([]byte(body), ts, secret)); got != http.StatusNoContent {
		t.Fatalf("valid delivery: %d", got)
	}
	if got := do("deadbeef"); got != http.StatusBadRequest {
		t.Fatalf("bad signature: %d", got)
	}
}
