// Command estamp_order runs the e-Stamp order flow end to end: validate, create with an
// idempotency key, upload the payer's PAN card, list the stamps and fetch a certificate.
//
//	STAMPMITRA_API_KEY=sk_test_... go run ./estamp_order [pan-card.pdf]
//
// Use a sandbox key: a live key debits your wallets. An order can be placed without the PAN card
// (so the upload is skipped when no file is given), but a PAN card is mandatory to complete the stamp.
package main

import (
	"context"
	"errors"
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
		fail("setup", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	order := &stampmitra.EstampOrderParams{
		Reference: fmt.Sprintf("order-%d", time.Now().Unix()), // your own reference for the whole order
		Stamps: []stampmitra.EstampStampLine{{
			ClientReference: "line-001",
			State:           "MH",
			Category:        stampmitra.CategoryEStamp,
			ArticleCode:     "ART1",
			Denomination:    1000,
			// A party is a bare name or an object; use the object when the state wants an address.
			FirstParty:  stampmitra.PartyDetails(stampmitra.EstampParty{Name: "Jane Doe", Address: "12 MG Road, Pune"}),
			SecondParty: ptr(stampmitra.PartyName("John Smith")),
			Purpose:     "Rental Agreement",
		}},
	}

	// 1. Validate: nothing is saved or charged. Show the warnings and the quote to your user.
	v, err := client.Estamp.Orders.Validate(ctx, order)
	if err != nil {
		report("validate", err)
	}
	fmt.Println("validate:", v.Status, "valid:", v.Valid)
	if !v.Valid {
		os.Exit(1)
	}
	if v.Quote != nil {
		fmt.Printf("quote: duty %.2f + service fee %.2f + GST %.2f\n", v.Quote.FaceValueTotal, v.Quote.ServiceFeeTotal, v.Quote.GstOnFee)
	}

	// 2. Create: debits the Stamp duty wallet (duty) and the Service wallet (fees, before GST). The API requires an Idempotency-Key; derive it from your own
	//    reference so that re-running THIS program cannot create a second order.
	created, err := client.Estamp.Orders.Create(ctx, order, stampmitra.WithIdempotencyKey("create-"+order.Reference))
	if err != nil {
		report("create", err)
	}
	fmt.Println("created", created.ID, created.Status)

	// 3. Upload the payer's PAN card (not needed to place the order, mandatory to complete the stamp).
	if len(os.Args) > 1 {
		f, err := os.Open(os.Args[1])
		if err != nil {
			fail("open PAN card", err)
		}
		defer f.Close()
		doc, err := client.Estamp.Orders.UploadDocument(ctx, created.ID, &stampmitra.EstampDocumentUploadParams{
			File: f, FileName: os.Args[1], Type: stampmitra.DocumentTypePANCard,
		})
		if err != nil {
			report("upload", err)
		}
		fmt.Println("uploaded", doc.DocumentID, doc.Status)
	}

	// 4. List the stamps of the order, then 5. ask for each certificate.
	stamps, err := client.Estamp.Orders.ListStamps(ctx, created.ID)
	if err != nil {
		report("list stamps", err)
	}
	for _, s := range stamps.Items {
		fmt.Println("stamp", s.ID, s.Status)
		cert, err := client.Estamp.Stamps.GetCertificate(ctx, s.ID)
		if err != nil {
			report("certificate", err)
		}
		if cert.Available {
			fmt.Println("  certificate ready; the download link expires at", cert.ExpiresAt)
		} else {
			fmt.Println("  certificate not available yet:", cert.Status)
		}
	}
}

func ptr[T any](v T) *T { return &v }

// report prints the findings of a rejected order and which wallet is short, then exits.
func report(step string, err error) {
	var api *stampmitra.APIError
	if errors.As(err, &api) {
		for _, f := range api.FieldErrors {
			fmt.Fprintf(os.Stderr, "  %s: %s %s\n", f.Field, f.Code, f.Message)
		}
		if api.Wallet != "" {
			fmt.Fprintln(os.Stderr, "  wallet to top up:", api.Wallet)
		}
	}
	fail(step, err)
}

func fail(step string, err error) {
	fmt.Fprintln(os.Stderr, step+":", err)
	os.Exit(1)
}
