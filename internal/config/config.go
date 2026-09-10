package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the root configuration.
type Config struct {
	Mode       string          `json:"mode"` // agent | aggregator
	Agent      AgentConfig     `json:"agent"`
	Aggregator AggregatorConfig `json:"aggregator"`
	LogLevel   string          `json:"log_level"`
	NodeName   string          `json:"node_name"`
}

// AgentConfig holds agent-specific settings.
type AgentConfig struct {
	LogPath   string          `json:"log_path"`
	Storage   StorageConfig   `json:"storage"`
	Collector CollectorConfig `json:"collector"`
	Exclude   ExcludeConfig   `json:"exclude"`
	API       APIConfig       `json:"api"`
}

// StorageConfig defines storage parameters.
type StorageConfig struct {
	Path        string        `json:"path"`
	Retention   time.Duration `json:"retention"`
	MaxDisk     int64         `json:"max_disk"` // bytes
	Compression string        `json:"compression"`
	GCInterval  time.Duration `json:"gc_interval"`
}

// CollectorConfig defines collection tuning.
type CollectorConfig struct {
	BatchSize     int           `json:"batch_size"`
	BatchInterval time.Duration `json:"batch_interval"`
	MaxGoroutines int           `json:"max_goroutines"`
	RateLimit     int           `json:"rate_limit"`
	BufferSize    int           `json:"buffer_size"`
}

// ExcludeConfig defines what to skip.
type ExcludeConfig struct {
	Namespaces []string          `json:"namespaces"`
	Labels     map[string]string `json:"labels"`
}

// APIConfig for agent internal API.
type APIConfig struct {
	Port     int `json:"port"`
	GRPCPort int `json:"grpc_port"`
}

// AggregatorConfig holds aggregator settings.
type AggregatorConfig struct {
	Dashboard DashboardConfig  `json:"dashboard"`
	Discovery DiscoveryConfig  `json:"discovery"`
	Query     QueryConfig      `json:"query"`
}

// DashboardConfig for the web UI.
type DashboardConfig struct {
	Port int      `json:"port"`
	Auth AuthConfig `json:"auth"`
}

// AuthConfig for dashboard security and 2FA.
type AuthConfig struct {
	Enabled  bool   `json:"enabled"`
	FilePath string `json:"file_path"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// DiscoveryConfig for agent discovery.
type DiscoveryConfig struct {
	Method        string   `json:"method"` // kubernetes | static
	LabelSelector string   `json:"label_selector"`
	Namespace     string   `json:"namespace"`
	Endpoints     []string `json:"endpoints"` // for static
}

// QueryConfig for aggregator query settings.
type QueryConfig struct {
	Timeout    time.Duration `json:"timeout"`
	MaxResults int           `json:"max_results"`
}

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Mode:     "agent",
		LogLevel: "info",
		NodeName: hostname(),
		Agent: AgentConfig{
			LogPath: "/var/log/containers",
			Storage: StorageConfig{
				Path:        "/data/kapture",
				Retention:   7 * 24 * time.Hour, // 168h
				MaxDisk:     5 * 1024 * 1024 * 1024, // 5GB
				Compression: "snappy",
				GCInterval:  5 * time.Minute,
			},
			Collector: CollectorConfig{
				BatchSize:     100,
				BatchInterval: 100 * time.Millisecond,
				MaxGoroutines: 50,
				RateLimit:     5000,
				BufferSize:    4096,
			},
			Exclude: ExcludeConfig{
				Namespaces: []string{},
				Labels:     map[string]string{},
			},
			API: APIConfig{
				Port:     19489,
				GRPCPort: 19490,
			},
		},
		Aggregator: AggregatorConfig{
			Dashboard: DashboardConfig{
				Port: 19488,
				Auth: AuthConfig{
					Enabled:  true,
					FilePath: "/data/kapture/auth.json",
					Username: "admin",
					Password: "",
				},
			},
			Discovery: DiscoveryConfig{
				Method:        "static",
				LabelSelector: "app=kapture,role=agent",
				Namespace:     "kapture",
			},
			Query: QueryConfig{
				Timeout:    30 * time.Second,
				MaxResults: 10000,
			},
		},
	}
}

// LoadFromEnv overrides config values from environment variables (supports KAPTURE_* and LOG_CATCHER_*).
func (c *Config) LoadFromEnv() {
	if v := getEnv("KAPTURE_MODE", "LOG_CATCHER_MODE"); v != "" {
		c.Mode = v
	}
	if v := getEnv("KAPTURE_LOG_LEVEL", "LOG_CATCHER_LOG_LEVEL"); v != "" {
		c.LogLevel = v
	}
	if v := getEnv("KAPTURE_NODE_NAME", "LOG_CATCHER_NODE_NAME"); v != "" {
		c.NodeName = v
	}
	if v := getEnv("KAPTURE_LOG_PATH", "LOG_CATCHER_LOG_PATH"); v != "" {
		c.Agent.LogPath = v
	}
	if v := getEnv("KAPTURE_STORAGE_PATH", "LOG_CATCHER_STORAGE_PATH"); v != "" {
		c.Agent.Storage.Path = v
	}
	if v := getEnv("KAPTURE_STORAGE_RETENTION", "LOG_CATCHER_STORAGE_RETENTION"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.Agent.Storage.Retention = d
		}
	}
	if v := getEnv("KAPTURE_STORAGE_MAX_DISK", "LOG_CATCHER_STORAGE_MAX_DISK"); v != "" {
		c.Agent.Storage.MaxDisk = parseBytes(v)
	}
	if v := getEnv("KAPTURE_AGENT_PORT", "LOG_CATCHER_AGENT_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			c.Agent.API.Port = p
		}
	}
	if v := getEnv("KAPTURE_AGENT_GRPC_PORT", "LOG_CATCHER_AGENT_GRPC_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			c.Agent.API.GRPCPort = p
		}
	}
	if v := getEnv("KAPTURE_DASHBOARD_PORT", "LOG_CATCHER_DASHBOARD_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			c.Aggregator.Dashboard.Port = p
		}
	}
	if v := getEnv("KAPTURE_AUTH_ENABLED", "LOG_CATCHER_AUTH_ENABLED"); v != "" {
		c.Aggregator.Dashboard.Auth.Enabled = (v == "true" || v == "1" || v == "yes")
	}
	if v := getEnv("KAPTURE_AUTH_FILE", "LOG_CATCHER_AUTH_FILE"); v != "" {
		c.Aggregator.Dashboard.Auth.FilePath = v
	}
	if v := getEnv("KAPTURE_USERNAME", "LOG_CATCHER_USERNAME"); v != "" {
		c.Aggregator.Dashboard.Auth.Username = v
	}
	if v := getEnv("KAPTURE_PASSWORD", "LOG_CATCHER_PASSWORD"); v != "" {
		c.Aggregator.Dashboard.Auth.Enabled = true
		c.Aggregator.Dashboard.Auth.Password = v
		if c.Aggregator.Dashboard.Auth.Username == "" {
			c.Aggregator.Dashboard.Auth.Username = "admin"
		}
	}
	if v := getEnv("KAPTURE_DISCOVERY_METHOD", "LOG_CATCHER_DISCOVERY_METHOD"); v != "" {
		c.Aggregator.Discovery.Method = v
	}
	if v := getEnv("KAPTURE_DISCOVERY_NAMESPACE", "LOG_CATCHER_DISCOVERY_NAMESPACE"); v != "" {
		c.Aggregator.Discovery.Namespace = v
	}
	if v := getEnv("KAPTURE_DISCOVERY_LABEL_SELECTOR", "LOG_CATCHER_DISCOVERY_LABEL_SELECTOR"); v != "" {
		c.Aggregator.Discovery.LabelSelector = v
	}
	if v := getEnv("KAPTURE_DISCOVERY_ENDPOINTS", "LOG_CATCHER_DISCOVERY_ENDPOINTS"); v != "" {
		c.Aggregator.Discovery.Endpoints = strings.Split(v, ",")
	}
}

func getEnv(primary, secondary string) string {
	if v := os.Getenv(primary); v != "" {
		return v
	}
	return os.Getenv(secondary)
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

func parseBytes(s string) int64 {
	s = strings.TrimSpace(strings.ToUpper(s))
	multiplier := int64(1)
	if strings.HasSuffix(s, "GB") {
		multiplier = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "GB")
	} else if strings.HasSuffix(s, "MB") {
		multiplier = 1024 * 1024
		s = strings.TrimSuffix(s, "MB")
	} else if strings.HasSuffix(s, "KB") {
		multiplier = 1024
		s = strings.TrimSuffix(s, "KB")
	}
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 5 * 1024 * 1024 * 1024 // default 5GB
	}
	return n * multiplier
}
