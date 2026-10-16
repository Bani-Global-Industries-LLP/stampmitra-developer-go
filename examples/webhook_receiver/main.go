// Command webhook_receiver is a net/http server that verifies StampMitra webhook deliveries.
//
//	WEBHOOK_SECRET=whsec_... go run ./webhook_receiver   # listens on :8080 (PORT to change)
//
// The signature covers the raw body, so read it with io.ReadAll BEFORE decoding and hand the same
// bytes to ConstructEventFromHeaders. Respond with any 2xx to acknowledge; a non-2xx is retried.
package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	stampmitra "github.com/Bani-Global-Industries-LLP/stampmitra-developer-go"
)

const maxBody = 1 << 20 // 1 MiB

func handler(secret string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/webhooks/stampmitra", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
		if err != nil {
			http.Error(w, "cannot read body", http.StatusBadRequest)
			return
		}
		ev, err := stampmitra.ConstructEventFromHeaders(body, r.Header, secret)
		if err != nil {
			var sigErr *stampmitra.SignatureVerificationError
			if errors.As(err, &sigErr) {
				log.Printf("rejected webhook: %s", sigErr.Reason) // never log the secret or signature
			}
			http.Error(w, "invalid signature", http.StatusBadRequest)
			return
		}

		switch e := ev.(type) {
		case *stampmitra.OrderCreatedEvent:
			log.Printf("order created: %s", e.CoreOrderID)
		case *stampmitra.OrderPaymentConfirmedEvent:
			log.Printf("payment confirmed: %s", e.CoreOrderID)
		case *stampmitra.OrderDocumentReadyEvent:
			log.Printf("document ready: %s (event %s)", e.CoreOrderID, e.ID)
		case *stampmitra.OrderFailedEvent, *stampmitra.OrderCancelledEvent, *stampmitra.OrderRefundCompletedEvent,
			*stampmitra.OrderFulfilmentStartedEvent, *stampmitra.OrderFulfilmentCompletedEvent:
			log.Printf("%s: %s", ev.Type(), ev.Base().CoreOrderID)
		case *stampmitra.UnknownEvent:
			log.Printf("ignoring unrecognised event %q", e.RawType)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

func main() {
	secret := os.Getenv("WEBHOOK_SECRET")
	if secret == "" {
		log.Fatal("set WEBHOOK_SECRET to your webhook signing secret")
	}
	addr := ":" + envOr("PORT", "8080")
	fmt.Println("listening on", addr)
	log.Fatal(http.ListenAndServe(addr, handler(secret))) //nolint:gosec // example server
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
