package discovery

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"
)

const (
	inClusterTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	inClusterCAPath    = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	inClusterAPIBase   = "https://kubernetes.default.svc"
)

// K8sOptions defines discovery parameters for Kubernetes.
type K8sOptions struct {
	Namespace       string
	LabelSelector   string
	HeadlessService string
	AgentPort       int
	RefreshInterval time.Duration
	StaticEndpoints []string
}

// KubernetesProvider dynamically discovers agent endpoints in Kubernetes.
type KubernetesProvider struct {
	opts       K8sOptions
	endpoints  []string
	httpClient *http.Client
	hasToken   bool
	token      string
	done       chan struct{}
	mu         sync.RWMutex
}

// NewKubernetes creates a new Kubernetes discovery provider.
func NewKubernetes(opts K8sOptions) *KubernetesProvider {
	if opts.Namespace == "" {
		opts.Namespace = "kapture"
	}
	if opts.LabelSelector == "" {
		opts.LabelSelector = "app=kapture,role=agent"
	}
	if opts.HeadlessService == "" {
		opts.HeadlessService = "kapture-agents"
	}
	if opts.AgentPort <= 0 {
		opts.AgentPort = 19489
	}
	if opts.RefreshInterval <= 0 {
		opts.RefreshInterval = 10 * time.Second
	}

	p := &KubernetesProvider{
		opts:      opts,
		endpoints: opts.StaticEndpoints,
		done:      make(chan struct{}),
	}

	// Try setting up in-cluster HTTP client for K8s API
	p.initInClusterClient()

	// Initial discovery pass
	p.refresh()

	// Start background refresh goroutine
	go p.refreshLoop()

	return p
}

// Endpoints returns the current list of agent URLs.
func (p *KubernetesProvider) Endpoints() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	res := make([]string, len(p.endpoints))
	copy(res, p.endpoints)
	return res
}

// Close stops background discovery.
func (p *KubernetesProvider) Close() {
	close(p.done)
}

func (p *KubernetesProvider) initInClusterClient() {
	tokenBytes, err := os.ReadFile(inClusterTokenPath)
	if err != nil {
		return // Not running inside cluster with token mounted
	}
	p.token = string(tokenBytes)

	caCert, err := os.ReadFile(inClusterCAPath)
	if err != nil {
		return
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return
	}

	p.httpClient = &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool,
			},
		},
	}
	p.hasToken = true
	slog.Info("in-cluster kubernetes credentials detected", "namespace", p.opts.Namespace)
}

func (p *KubernetesProvider) refreshLoop() {
	ticker := time.NewTicker(p.opts.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.done:
			return
		case <-ticker.C:
			p.refresh()
		}
	}
}

func (p *KubernetesProvider) refresh() {
	endpointSet := make(map[string]bool)

	// Include any manually configured static endpoints
	for _, ep := range p.opts.StaticEndpoints {
		if ep != "" {
			endpointSet[ep] = true
		}
	}

	// 1. Try In-Cluster Kubernetes API direct query
	if p.hasToken && p.httpClient != nil {
		eps, err := p.discoverViaAPI()
		if err == nil && len(eps) > 0 {
			for _, ep := range eps {
				endpointSet[ep] = true
			}
		} else if err != nil {
			slog.Debug("k8s api discovery attempt failed, falling back to DNS", "error", err)
		}
	}

	// 2. Try DNS lookup via Headless Service (Standard Kubernetes pattern)
	dnsEps, err := p.discoverViaDNS()
	if err == nil && len(dnsEps) > 0 {
		for _, ep := range dnsEps {
			endpointSet[ep] = true
		}
	}

	result := make([]string, 0, len(endpointSet))
	for ep := range endpointSet {
		result = append(result, ep)
	}
	sort.Strings(result)

	p.mu.Lock()
	changed := len(result) != len(p.endpoints)
	if !changed {
		for i := range result {
			if result[i] != p.endpoints[i] {
				changed = true
				break
			}
		}
	}
	p.endpoints = result
	p.mu.Unlock()

	if changed {
		slog.Info("kubernetes agent discovery updated", "count", len(result), "endpoints", result)
	}
}

// discoverViaAPI lists pods directly from K8s API server using in-cluster ServiceAccount credentials.
func (p *KubernetesProvider) discoverViaAPI() ([]string, error) {
	url := fmt.Sprintf("%s/api/v1/namespaces/%s/pods?labelSelector=%s",
		inClusterAPIBase, p.opts.Namespace, p.opts.LabelSelector)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.token)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("k8s api returned %d", resp.StatusCode)
	}

	var podList struct {
		Items []struct {
			Status struct {
				Phase  string `json:"phase"`
				PodIP  string `json:"podIP"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&podList); err != nil {
		return nil, err
	}

	var endpoints []string
	for _, item := range podList.Items {
		if item.Status.Phase == "Running" && item.Status.PodIP != "" {
			endpoints = append(endpoints, fmt.Sprintf("http://%s:%d", item.Status.PodIP, p.opts.AgentPort))
		}
	}
	return endpoints, nil
}

// discoverViaDNS resolves the headless service DNS name in CoreDNS.
func (p *KubernetesProvider) discoverViaDNS() ([]string, error) {
	domains := []string{
		fmt.Sprintf("%s.%s.svc.cluster.local", p.opts.HeadlessService, p.opts.Namespace),
		fmt.Sprintf("%s.%s", p.opts.HeadlessService, p.opts.Namespace),
		p.opts.HeadlessService,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()

	for _, domain := range domains {
		ips, err := net.DefaultResolver.LookupIP(ctx, "ip4", domain)
		if err == nil && len(ips) > 0 {
			var endpoints []string
			for _, ip := range ips {
				endpoints = append(endpoints, fmt.Sprintf("http://%s:%d", ip.String(), p.opts.AgentPort))
			}
			return endpoints, nil
		}
	}
	return nil, fmt.Errorf("no dns records resolved for headless service %s", p.opts.HeadlessService)
}
