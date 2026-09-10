package discovery

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestKubernetesProviderStaticFallback(t *testing.T) {
	opts := K8sOptions{
		Namespace:       "test-ns",
		HeadlessService: "non-existent-service",
		StaticEndpoints: []string{"http://10.0.0.1:19489", "http://10.0.0.2:19489"},
		RefreshInterval: 50 * time.Millisecond,
	}

	p := NewKubernetes(opts)
	defer p.Close()

	eps := p.Endpoints()
	if len(eps) != 2 {
		t.Fatalf("expected 2 static endpoints, got %d: %v", len(eps), eps)
	}
	if eps[0] != "http://10.0.0.1:19489" || eps[1] != "http://10.0.0.2:19489" {
		t.Fatalf("unexpected endpoints: %v", eps)
	}
}

func TestKubernetesAPIDiscoveryMock(t *testing.T) {
	// Mock K8s API server
	mockAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/namespaces/log-catcher/pods" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, `{
				"items": [
					{"status": {"phase": "Running", "podIP": "10.244.1.15"}},
					{"status": {"phase": "Running", "podIP": "10.244.2.20"}},
					{"status": {"phase": "Pending", "podIP": ""}}
				]
			}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer mockAPI.Close()

	opts := K8sOptions{
		Namespace:       "log-catcher",
		AgentPort:       19489,
		RefreshInterval: 50 * time.Millisecond,
	}

	p := &KubernetesProvider{
		opts:       opts,
		httpClient: mockAPI.Client(),
		hasToken:   true,
		token:      "mock-token",
		done:       make(chan struct{}),
	}

	url := fmt.Sprintf("%s/api/v1/namespaces/%s/pods?labelSelector=%s",
		mockAPI.URL, p.opts.Namespace, p.opts.LabelSelector)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var podList struct {
		Items []struct {
			Status struct {
				Phase string `json:"phase"`
				PodIP string `json:"podIP"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&podList); err != nil {
		t.Fatal(err)
	}

	var eps []string
	for _, item := range podList.Items {
		if item.Status.Phase == "Running" && item.Status.PodIP != "" {
			eps = append(eps, fmt.Sprintf("http://%s:%d", item.Status.PodIP, p.opts.AgentPort))
		}
	}

	if len(eps) != 2 {
		t.Fatalf("expected 2 running agent pods, got %d: %v", len(eps), eps)
	}
	if eps[0] != "http://10.244.1.15:19489" || eps[1] != "http://10.244.2.20:19489" {
		t.Fatalf("unexpected endpoints: %v", eps)
	}
}
