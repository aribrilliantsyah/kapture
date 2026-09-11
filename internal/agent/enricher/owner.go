package enricher

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/ordinary/k8s-log-catcher/internal/kube"
)

const (
	lookupTimeout = 3 * time.Second
	fallbackTTL   = 2 * time.Minute // retry the API for pods resolved by name only
	apiBackoff    = time.Minute
	maxCachedPods = 20000
)

// Resolver finds the workload that owns a pod by following ownerReferences
// (Pod -> ReplicaSet -> Deployment, Pod -> Job -> CronJob, ...). Rollouts give
// pods new names, the owner stays the same, so logs of every generation of a
// deployment are grouped together. Without API access it falls back to
// ExtractWorkload (name patterns).
type Resolver struct {
	kube *kube.Client

	mu        sync.Mutex
	cache     map[string]cached
	downUntil time.Time
}

type cached struct {
	w       Workload
	expires time.Time // zero = permanent (resolved through the API)
}

// NewResolver creates a resolver; a nil client means name patterns only.
func NewResolver(c *kube.Client) *Resolver {
	return &Resolver{kube: c, cache: map[string]cached{}}
}

type objectMeta struct {
	Metadata struct {
		OwnerReferences []struct {
			Kind       string `json:"kind"`
			Name       string `json:"name"`
			Controller *bool  `json:"controller"`
		} `json:"ownerReferences"`
	} `json:"metadata"`
}

// Resolve returns the workload of a pod.
func (r *Resolver) Resolve(namespace, pod string) Workload {
	if r == nil {
		return ExtractWorkload(pod)
	}
	key := namespace + "/" + pod
	r.mu.Lock()
	if c, ok := r.cache[key]; ok && (c.expires.IsZero() || time.Now().Before(c.expires)) {
		r.mu.Unlock()
		return c.w
	}
	useAPI := r.kube != nil && time.Now().After(r.downUntil)
	r.mu.Unlock()

	w, err := Workload{}, errors.New("no api")
	if useAPI {
		w, err = r.lookup(namespace, pod)
	}
	entry := cached{w: w}
	if err != nil {
		if useAPI && !errors.Is(err, kube.ErrNotFound) {
			slog.Warn("cannot read pod owners from the Kubernetes API, grouping by pod name for now", "pod", key, "error", err)
			r.mu.Lock()
			r.downUntil = time.Now().Add(apiBackoff)
			r.mu.Unlock()
		}
		entry = cached{w: ExtractWorkload(pod), expires: time.Now().Add(fallbackTTL)}
	}
	r.mu.Lock()
	if len(r.cache) > maxCachedPods {
		clear(r.cache)
	}
	r.cache[key] = entry
	r.mu.Unlock()
	return entry.w
}

func (r *Resolver) lookup(ns, pod string) (Workload, error) {
	ctx, cancel := context.WithTimeout(context.Background(), lookupTimeout)
	defer cancel()

	var p objectMeta
	if err := r.kube.Get(ctx, "/api/v1/namespaces/"+ns+"/pods/"+pod, &p); err != nil {
		return Workload{}, err
	}
	kind, name, ok := controller(p)
	if !ok {
		return Workload{Name: pod, Type: "pod"}, nil
	}
	switch kind {
	case "ReplicaSet":
		var rs objectMeta
		if err := r.kube.Get(ctx, "/apis/apps/v1/namespaces/"+ns+"/replicasets/"+name, &rs); err == nil {
			if k, n, ok := controller(rs); ok && k == "Deployment" {
				return Workload{Name: n, Type: "deployment"}, nil
			}
		}
		return Workload{Name: name, Type: "replicaset"}, nil
	case "Job":
		var job objectMeta
		if err := r.kube.Get(ctx, "/apis/batch/v1/namespaces/"+ns+"/jobs/"+name, &job); err == nil {
			if k, n, ok := controller(job); ok && k == "CronJob" {
				return Workload{Name: n, Type: "cronjob"}, nil
			}
		}
		return Workload{Name: name, Type: "job"}, nil
	case "Node":
		// Static pods (kube-apiserver-<node>) are owned by their node.
		return Workload{Name: strings.TrimSuffix(pod, "-"+name), Type: "static"}, nil
	default:
		return Workload{Name: name, Type: strings.ToLower(kind)}, nil
	}
}

func controller(o objectMeta) (kind, name string, ok bool) {
	refs := o.Metadata.OwnerReferences
	for _, ref := range refs {
		if ref.Controller != nil && *ref.Controller {
			return ref.Kind, ref.Name, true
		}
	}
	if len(refs) > 0 {
		return refs[0].Kind, refs[0].Name, true
	}
	return "", "", false
}
