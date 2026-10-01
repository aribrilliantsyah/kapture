package fanout

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aribrilliantsyah/kapture/internal/aggregator/discovery"
	"github.com/aribrilliantsyah/kapture/internal/model"
)

func fakeAgent(t *testing.T, res model.QueryResult) string {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(res)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func logAt(sec int64, msg string) model.LogEntry {
	return model.LogEntry{Timestamp: time.Unix(sec, 0).UTC(), Message: msg}
}

func TestQueryLogsMergesWithinAgentBounds(t *testing.T) {
	// Agent A has more beyond t=9; agent B returned everything it has.
	a := fakeAgent(t, model.QueryResult{Entries: []model.LogEntry{logAt(10, "a10"), logAt(9, "a9")}, NextCursor: "9000000000"})
	b := fakeAgent(t, model.QueryResult{Entries: []model.LogEntry{logAt(12, "b12"), logAt(7, "b7")}})
	c := New(discovery.NewStatic([]string{a, b}), time.Second)

	res, err := c.QueryLogs(model.QueryRequest{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range res.Entries {
		got = append(got, e.Message)
	}
	// b7 is older than A's bound: it must wait for the next page, or A's
	// entries between 7 and 9 would be skipped.
	if len(got) != 3 || got[0] != "b12" || got[1] != "a10" || got[2] != "a9" {
		t.Fatalf("entries = %v, want [b12 a10 a9]", got)
	}
	if res.NextCursor != "9000000000" {
		t.Fatalf("next cursor = %q", res.NextCursor)
	}
}

func TestQueryLogsReportsUnreachableAgents(t *testing.T) {
	a := fakeAgent(t, model.QueryResult{Entries: []model.LogEntry{logAt(1, "a1")}})
	c := New(discovery.NewStatic([]string{a, "http://127.0.0.1:1"}), time.Second)
	res, err := c.QueryLogs(model.QueryRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Entries) != 1 || len(res.Errors) != 1 {
		t.Fatalf("entries=%d errors=%v", len(res.Entries), res.Errors)
	}

	res, _ = New(discovery.NewStatic(nil), time.Second).QueryLogs(model.QueryRequest{})
	if len(res.Errors) != 1 || res.Errors[0] != "no agents discovered" {
		t.Fatalf("errors = %v", res.Errors)
	}
}

func TestQueryLogsPassesAgentRejection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"invalid regex: missing )"}`))
	}))
	defer srv.Close()
	_, err := New(discovery.NewStatic([]string{srv.URL}), time.Second).QueryLogs(model.QueryRequest{Regex: "("})
	se, ok := err.(*StatusError)
	if !ok || se.Code != http.StatusBadRequest {
		t.Fatalf("err = %v, want 400 StatusError", err)
	}
}
