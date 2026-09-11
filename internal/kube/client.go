// Package kube is a minimal read-only Kubernetes API client using the pod's
// service account (no client-go, keeps the binary small).
package kube

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	tokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	caPath    = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
)

// ErrNotFound is returned for HTTP 404.
var ErrNotFound = errors.New("not found")

// Client performs authenticated GETs against the API server.
type Client struct {
	base      string
	http      *http.Client
	tokenFile string

	mu      sync.Mutex
	token   string
	tokenAt time.Time
}

// InCluster builds a client from the mounted service account. It fails when
// not running inside a cluster.
func InCluster() (*Client, error) {
	ca, err := os.ReadFile(caPath)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(ca) {
		return nil, errors.New("invalid service account CA")
	}
	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
	base := "https://kubernetes.default.svc"
	if host != "" && port != "" {
		base = "https://" + net.JoinHostPort(host, port)
	}
	c := New(base, "", &http.Client{
		Timeout:   5 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}},
	})
	c.tokenFile = tokenPath
	if _, err := c.bearer(); err != nil {
		return nil, err
	}
	return c, nil
}

// New builds a client for an explicit endpoint (tests, out-of-cluster).
func New(base, token string, hc *http.Client) *Client {
	return &Client{base: strings.TrimSuffix(base, "/"), http: hc, token: token, tokenAt: time.Now()}
}

// bearer returns the service account token, re-read every few minutes because
// projected tokens are rotated by the kubelet.
func (c *Client) bearer() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.tokenFile != "" && (c.token == "" || time.Since(c.tokenAt) > 5*time.Minute) {
		b, err := os.ReadFile(c.tokenFile)
		if err != nil {
			return "", err
		}
		c.token, c.tokenAt = strings.TrimSpace(string(b)), time.Now()
	}
	return c.token, nil
}

// Get decodes the JSON object at path into out.
func (c *Client) Get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return err
	}
	if tok, err := c.bearer(); err == nil && tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return ErrNotFound
	case resp.StatusCode != http.StatusOK:
		return fmt.Errorf("kubernetes API %s: HTTP %d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
