package enricher

import "testing"

func TestParseFilename(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		want   FileMeta
		wantOK bool
	}{
		{
			name: "standard deployment pod",
			path: "/var/log/containers/api-server-7f8b9c6d4-x2k1p_production_app-a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2.log",
			want: FileMeta{
				Pod:       "api-server-7f8b9c6d4-x2k1p",
				Namespace: "production",
				Container: "app",
			},
			wantOK: true,
		},
		{
			name: "statefulset pod",
			path: "/var/log/containers/postgres-0_database_postgres-a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2.log",
			want: FileMeta{
				Pod:       "postgres-0",
				Namespace: "database",
				Container: "postgres",
			},
			wantOK: true,
		},
		{
			name:   "invalid filename",
			path:   "/var/log/containers/invalid.log",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseFilename(tt.path)
			if ok != tt.wantOK {
				t.Fatalf("ParseFilename() ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if got.Pod != tt.want.Pod {
				t.Errorf("pod = %q, want %q", got.Pod, tt.want.Pod)
			}
			if got.Namespace != tt.want.Namespace {
				t.Errorf("namespace = %q, want %q", got.Namespace, tt.want.Namespace)
			}
			if got.Container != tt.want.Container {
				t.Errorf("container = %q, want %q", got.Container, tt.want.Container)
			}
		})
	}
}

func TestExtractWorkload(t *testing.T) {
	tests := []struct {
		podName  string
		wantName string
		wantType string
	}{
		// Deployment: name-<rs-hash>-<pod-hash>
		{"api-server-7f8b9c6d4-x2k1p", "api-server", "deployment"},
		{"worker-batch-5a6b7c8d9-abc12", "worker-batch", "deployment"},
		{"my-app-6db9f4f9b7-kx5ht", "my-app", "deployment"},

		// StatefulSet: name-<ordinal>
		{"postgres-0", "postgres", "statefulset"},
		{"postgres-1", "postgres", "statefulset"},
		{"kafka-broker-2", "kafka-broker", "statefulset"},
		{"redis-cluster-10", "redis-cluster", "statefulset"},

		// CronJob: name-<timestamp>-<hash>
		{"backup-28456123-abc12", "backup", "cronjob"},
		{"cleanup-1736942400-x1y2z", "cleanup", "cronjob"},

		// DaemonSet/Job: name-<hash>
		{"node-exporter-abc12", "node-exporter", "daemonset"},
		{"fluentd-x1y2z", "fluentd", "daemonset"},

		// Standalone pod
		{"debug-pod", "debug-pod", "pod"},
		{"my-migration", "my-migration", "pod"},
	}

	for _, tt := range tests {
		t.Run(tt.podName, func(t *testing.T) {
			wl := ExtractWorkload(tt.podName)
			if wl.Name != tt.wantName {
				t.Errorf("name = %q, want %q", wl.Name, tt.wantName)
			}
			if wl.Type != tt.wantType {
				t.Errorf("type = %q, want %q", wl.Type, tt.wantType)
			}
		})
	}
}
