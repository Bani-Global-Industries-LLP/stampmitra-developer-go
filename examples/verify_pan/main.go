// Command verify_pan verifies a PAN.
//
//	STAMPMITRA_API_KEY=sk_test_... go run ./verify_pan ABCDE1234F "Rahul Kumar"
//
// Set STAMPMITRA_BASE_URL (for example http://127.0.0.1:4010) to point it at a mock server.
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
	pan, name := "ABCDE1234F", ""
	if len(os.Args) > 1 {
		pan = os.Args[1]
	}
	if len(os.Args) > 2 {
		name = os.Args[2]
	}

	var opts []stampmitra.Option
	if u := os.Getenv("STAMPMITRA_BASE_URL"); u != "" {
		opts = append(opts, stampmitra.WithBaseURL(u))
	}
	client, err := stampmitra.NewClient("", opts...) // "" reads STAMPMITRA_API_KEY
	if err != nil {
		fmt.Fprintln(os.Stderr, "setup:", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	res, err := client.Verify.PAN(ctx, &stampmitra.PANVerifyParams{PAN: pan, Name: name})
	if err != nil {
		var apiErr *stampmitra.APIError
		switch {
		case stampmitra.IsValidation(err) && !errors.As(err, &apiErr):
			fmt.Fprintln(os.Stderr, "invalid input (nothing was sent or billed):", err)
		case errors.As(err, &apiErr):
			fmt.Fprintf(os.Stderr, "API error %d %s (request %s): %s\n", apiErr.Status, apiErr.Code, apiErr.RequestID, apiErr.Message)
		default:
			fmt.Fprintln(os.Stderr, "request failed:", err)
		}
		os.Exit(1)
	}
	// A negative result is still a successful (and billed) call: check Valid.
	fmt.Printf("valid=%v nameMatch=%s reference=%s\n", res.Valid, res.NameMatchResult, res.ReferenceID)
}
