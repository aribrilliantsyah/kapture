package enricher

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ordinary/k8s-log-catcher/internal/kube"
)

func TestResolverFollowsOwners(t *testing.T) {
	objects := map[string]string{
		"/api/v1/namespaces/prod/pods/web-5c6d7f8b9-bbbbb":        `{"metadata":{"ownerReferences":[{"kind":"ReplicaSet","name":"web-5c6d7f8b9","controller":true}]}}`,
		"/apis/apps/v1/namespaces/prod/replicasets/web-5c6d7f8b9": `{"metadata":{"ownerReferences":[{"kind":"Deployment","name":"web","controller":true}]}}`,
		"/api/v1/namespaces/prod/pods/report-28456123-x7k2p":      `{"metadata":{"ownerReferences":[{"kind":"Job","name":"report-28456123","controller":true}]}}`,
		"/apis/batch/v1/namespaces/prod/jobs/report-28456123":     `{"metadata":{"ownerReferences":[{"kind":"CronJob","name":"report","controller":true}]}}`,
		"/api/v1/namespaces/kube-system/pods/kube-apiserver-cp1":  `{"metadata":{"ownerReferences":[{"kind":"Node","name":"cp1","controller":true}]}}`,
		"/api/v1/namespaces/prod/pods/debug":                      `{"metadata":{}}`,
	}
	calls := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if body, ok := objects[r.URL.Path]; ok {
			w.Write([]byte(body))
			return
		}
		http.NotFound(w, r)
	}))
	defer api.Close()
	res := NewResolver(kube.New(api.URL, "t", api.Client()))

	cases := []struct{ ns, pod, name, typ string }{
		{"prod", "web-5c6d7f8b9-bbbbb", "web", "deployment"},
		{"prod", "report-28456123-x7k2p", "report", "cronjob"},
		{"kube-system", "kube-apiserver-cp1", "kube-apiserver", "static"},
		{"prod", "debug", "debug", "pod"},
		{"prod", "gone-7f8b9c6d4-x2k1p", "gone", "deployment"}, // deleted pod: name pattern fallback
	}
	for _, c := range cases {
		if w := res.Resolve(c.ns, c.pod); w.Name != c.name || w.Type != c.typ {
			t.Errorf("%s/%s = %+v, want %s/%s", c.ns, c.pod, w, c.name, c.typ)
		}
	}
	before := calls
	res.Resolve("prod", "web-5c6d7f8b9-bbbbb")
	if calls != before {
		t.Fatal("resolved owners must be cached")
	}
}

func TestSafeHashDeployment(t *testing.T) {
	if w := ExtractWorkload("payments-bcdfghjkl-xz2k4"); w.Name != "payments" || w.Type != "deployment" {
		t.Fatalf("got %+v", w)
	}
}
