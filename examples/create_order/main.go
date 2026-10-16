// Command create_order creates an e-Stamp order and reads it back.
//
//	STAMPMITRA_API_KEY=sk_test_... go run ./create_order
//
// Orders are addressed by YOUR externalOrderId: the create response returns it and every later
// call (status, payment, documents, download) takes it.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	stampmitra "github.com/Bani-Global-Industries-LLP/stampmitra-developer-go"
)

func main() {
	var opts []stampmitra.Option
	if u := os.Getenv("STAMPMITRA_BASE_URL"); u != "" {
		opts = append(opts, stampmitra.WithBaseURL(u))
	}
	client, err := stampmitra.NewClient("", opts...)
	if err != nil {
		fmt.Fprintln(os.Stderr, "setup:", err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Derive the idempotency key from your own order reference, so a crash-and-retry of THIS program
	// cannot create two orders. (Retries inside the SDK already reuse one key automatically.)
	externalID := fmt.Sprintf("ord_%d", time.Now().Unix())
	order, err := client.Orders.Create(ctx, &stampmitra.CreateOrderParams{
		ExternalOrderID:    externalID,
		ExternalCustomerID: "cust_ext_456",
		ServiceCode:        "estamp.instrument.rent_agreement",
		Details:            map[string]any{"state": "MH", "stampValue": 500},
	}, stampmitra.WithIdempotencyKey("create-"+externalID))
	if err != nil {
		fmt.Fprintln(os.Stderr, "create order:", err)
		os.Exit(1)
	}
	fmt.Println("created", order.ExternalOrderID, "at", order.CreatedAt)

	// Use externalOrderId (there is no "id" field on the create response).
	status, err := client.Orders.GetStatus(ctx, order.ExternalOrderID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "get status:", err)
		os.Exit(1)
	}
	fmt.Printf("status: %s\n", status.Status) // upstream-owned shape, kept as raw JSON

	docs, err := client.Orders.ListDocuments(ctx, order.ExternalOrderID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "list documents:", err)
		os.Exit(1)
	}
	fmt.Printf("documents: %s\n", docs.Documents)
}
