package search

import "testing"

func TestMatch(t *testing.T) {
	cases := []struct {
		query string
		msg   string
		want  bool
	}{
		{"", "anything", true},
		{"timeout", "Connection TIMEOUT after 30s", true},
		{"timeout database", "database timeout", true},
		{"timeout database", "timeout only", false},
		{"error OR warning", "a warning here", true},
		{"error OR warning", "all good", false},
		{"error -healthcheck", "error in healthcheck", false},
		{"error -healthcheck", "error in handler", true},
		{`"connection refused"`, "dial: connection refused", true},
		{`"connection refused"`, "refused connection", false},
		{`/failed.*after \d+ retries/`, "Failed to connect after 3 retries", true},
		{`/failed.*after \d+ retries/`, "failed after many retries", false},
		{"level:error pod:api-0 boom", "boom", true},
	}
	for _, c := range cases {
		q, err := Parse(c.query)
		if err != nil {
			t.Fatalf("Parse(%q): %v", c.query, err)
		}
		if got := q.Match(c.msg); got != c.want {
			t.Errorf("%q on %q = %v, want %v", c.query, c.msg, got, c.want)
		}
	}
}

func TestFields(t *testing.T) {
	q, err := Parse(`ns:production workload:api-server level:error pod:api-0 c:app type:Deployment timeout`)
	if err != nil {
		t.Fatal(err)
	}
	f := q.Fields
	if f.Namespace != "production" || f.Workload != "api-server" || f.Level != "ERROR" ||
		f.Pod != "api-0" || f.Container != "app" || f.WorkloadType != "deployment" {
		t.Fatalf("fields = %+v", f)
	}
	if !q.HasText() || !q.Match("request timeout") {
		t.Fatal("free text term lost")
	}
	if q, _ := Parse("level:warn"); q.HasText() {
		t.Fatal("field-only query should not need the message")
	}
}

func TestInvalidRegex(t *testing.T) {
	if _, err := Parse("/([/"); err == nil {
		t.Fatal("expected error")
	}
}
