package enricher

import "regexp"

// Workload contains the extracted workload name and type.
type Workload struct {
	Name string
	Type string // deployment, statefulset, daemonset, job, cronjob, replicaset, pod
}

var (
	// Deployment: <name>-<rs-hash 7-10 chars>-<pod-hash 5 chars>
	// The rs-hash contains at least one digit (ReplicaSet hashes do)
	deploymentRe = regexp.MustCompile(`^(.+)-([a-z0-9]{7,10})-([a-z0-9]{5})$`)

	// StatefulSet: <name>-<ordinal>
	statefulSetRe = regexp.MustCompile(`^(.+)-(\d+)$`)

	// CronJob: <name>-<timestamp 8-10 digits>-<hash 5-8 chars>
	cronJobRe = regexp.MustCompile(`^(.+)-(\d{8,10})-([a-z0-9]{5,8})$`)

	// DaemonSet / Job / bare ReplicaSet: <name>-<hash 5 chars>
	daemonSetRe = regexp.MustCompile(`^(.+)-([a-z0-9]{5})$`)
)

// hasDigit checks if a string contains at least one digit.
func hasDigit(s string) bool {
	for _, c := range s {
		if c >= '0' && c <= '9' {
			return true
		}
	}
	return false
}

// ExtractWorkload parses a pod name and returns the workload name and type.
// It uses regex pattern matching on Kubernetes pod naming conventions.
func ExtractWorkload(podName string) Workload {
	// 1. CronJob: name-<timestamp>-<hash> (check early — timestamp is all digits)
	if m := cronJobRe.FindStringSubmatch(podName); m != nil {
		return Workload{Name: m[1], Type: "cronjob"}
	}

	// 2. Deployment: name-<rs-hash>-<pod-hash>
	//    rs-hash must contain at least one digit to distinguish from name-parts
	if m := deploymentRe.FindStringSubmatch(podName); m != nil {
		rsHash := m[2]
		if hasDigit(rsHash) {
			return Workload{Name: m[1], Type: "deployment"}
		}
	}

	// 3. StatefulSet: name-<ordinal>
	if m := statefulSetRe.FindStringSubmatch(podName); m != nil {
		return Workload{Name: m[1], Type: "statefulset"}
	}

	// 4. DaemonSet / Job / bare ReplicaSet: name-<hash>
	if m := daemonSetRe.FindStringSubmatch(podName); m != nil {
		return Workload{Name: m[1], Type: "daemonset"}
	}

	// 5. Fallback: standalone pod
	return Workload{Name: podName, Type: "pod"}
}
