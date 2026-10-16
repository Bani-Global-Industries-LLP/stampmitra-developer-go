// Command logs_pagination prints every request-log entry from the last 24 hours, following nextCursor lazily.
//
//	STAMPMITRA_API_KEY=sk_test_... go run ./logs_pagination
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
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	it := client.Logs.ListAll(&stampmitra.LogsListParams{
		From:  time.Now().Add(-24 * time.Hour),
		Limit: 100,
	})
	n := 0
	for it.Next(ctx) {
		e := it.Log()
		service := "-"
		if e.Service != nil {
			service = *e.Service
		}
		fmt.Println(e.ID, e.CreatedAt, service)
		n++
	}
	if err := it.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "iteration stopped:", err)
		os.Exit(1)
	}
	fmt.Println("entries:", n)
}
