package stampmitra

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func page(ids []string, next string) string {
	items := ""
	for i, id := range ids {
		if i > 0 {
			items += ","
		}
		items += `{"id":"` + id + `","createdAt":"2026-10-15T09:41:07.000Z","actorType":"api_key","actorId":null,"service":"pan.verify","metadata":null}`
	}
	nc := "null"
	if next != "" {
		nc = `"` + next + `"`
	}
	return `{"items":[` + items + `],"nextCursor":` + nc + `}`
}

func TestLogsIteratorThreePages(t *testing.T) {
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("cursor") {
		case "":
			writeJSON(w, 200, page([]string{"a", "b"}, "c1"))
		case "c1":
			writeJSON(w, 200, page([]string{"c"}, "c2"))
		case "c2":
			writeJSON(w, 200, page([]string{"d", "e"}, ""))
		default:
			t.Errorf("unexpected cursor %q", r.URL.Query().Get("cursor"))
		}
	})
	it := env.c.Logs.ListAll(&LogsListParams{Limit: 2, Service: "pan.verify", Status: "success", From: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)})
	if it.Log() != nil {
		t.Fatal("Log() before Next must be nil")
	}
	var ids []string
	for it.Next(context.Background()) {
		ids = append(ids, it.Log().ID)
	}
	if it.Err() != nil {
		t.Fatal(it.Err())
	}
	if got := len(ids); got != 5 || ids[0] != "a" || ids[4] != "e" {
		t.Fatalf("ids = %v", ids)
	}
	if it.Next(context.Background()) || it.Log() != nil {
		t.Fatal("exhausted iterator must stay exhausted")
	}
	reqs := env.rec.all()
	if len(reqs) != 3 {
		t.Fatalf("pages fetched = %d", len(reqs))
	}
	q, _ := url.ParseQuery(reqs[1].Query)
	if q.Get("cursor") != "c1" || q.Get("limit") != "2" || q.Get("service") != "pan.verify" || q.Get("status") != "success" || q.Get("from") != "2026-10-01T00:00:00Z" {
		t.Errorf("query on page 2: %v", q)
	}
}

func TestLogsIteratorIsLazy(t *testing.T) {
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, page([]string{"a"}, "c1")) })
	it := env.c.Logs.ListAll(nil)
	if len(env.rec.all()) != 0 {
		t.Fatal("ListAll must not fetch")
	}
	if !it.Next(context.Background()) || len(env.rec.all()) != 1 {
		t.Fatal("first Next fetches exactly one page")
	}
}

func TestLogsIteratorSkipsEmptyPagesAndStartsFromCursor(t *testing.T) {
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("cursor") {
		case "start":
			writeJSON(w, 200, page(nil, "c2"))
		case "c2":
			writeJSON(w, 200, page([]string{"x"}, ""))
		}
	})
	it := env.c.Logs.ListAll(&LogsListParams{Cursor: "start"})
	if !it.Next(context.Background()) || it.Log().ID != "x" || it.Next(context.Background()) || it.Err() != nil {
		t.Fatalf("err=%v", it.Err())
	}
}

func TestLogsIteratorPageErrorStops(t *testing.T) {
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
		if n == 0 {
			writeJSON(w, 200, page([]string{"a"}, "c1"))
			return
		}
		writeErr(w, 401, "UNAUTHORIZED", "bad key")
	})
	it := env.c.Logs.ListAll(nil)
	var got int
	for it.Next(context.Background()) {
		got++
	}
	if got != 1 || !IsAuthentication(it.Err()) {
		t.Fatalf("got %d, err %v", got, it.Err())
	}
	if it.Next(context.Background()) {
		t.Fatal("a failed iterator must not continue")
	}
}

func TestLogsIteratorStuckCursor(t *testing.T) {
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, page([]string{"a"}, "same")) })
	it := env.c.Logs.ListAll(&LogsListParams{Cursor: "same"})
	var count int
	for it.Next(context.Background()) {
		count++
		if count > 5 {
			t.Fatal("iterator does not terminate")
		}
	}
	if !errors.Is(it.Err(), ErrDecode) {
		t.Fatalf("err = %v", it.Err())
	}
	if count != 1 {
		t.Fatalf("count = %d", count)
	}
}

func TestLogsIteratorContextCancel(t *testing.T) {
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, page([]string{"a"}, "c1")) })
	ctx, cancel := context.WithCancel(context.Background())
	it := env.c.Logs.ListAll(nil)
	if !it.Next(ctx) {
		t.Fatal(it.Err())
	}
	cancel()
	if it.Next(ctx) {
		t.Fatal("must stop after cancel")
	}
	if !errors.Is(it.Err(), context.Canceled) {
		t.Fatalf("err = %v", it.Err())
	}
}

func TestLogsListQueryEncoding(t *testing.T) {
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, page(nil, "")) })
	_, err := env.c.Logs.List(context.Background(), &LogsListParams{
		From: time.Date(2026, 10, 1, 5, 30, 0, 0, time.FixedZone("IST", 19800)), Service: "a&b=c", Limit: 200, Cursor: "x y",
	})
	if err != nil {
		t.Fatal(err)
	}
	q, _ := url.ParseQuery(env.rec.all()[0].Query)
	if q.Get("from") != "2026-10-01T00:00:00Z" || q.Get("service") != "a&b=c" || q.Get("cursor") != "x y" || q.Get("to") != "" || q.Get("status") != "" {
		t.Errorf("query = %v", q)
	}
}
