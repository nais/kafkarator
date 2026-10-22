// Two entrypoints, driven by environment variables:
//
//   - provision: idempotently ensure the nais-verification team and the
//     kafka-kanary service account exist (via the open Nais API gRPC endpoint).
//   - deploy: mint a short-lived service account token and apply the canary
//     Topic and Application manifests to each configured cluster via the Nais
//     API REST apply endpoint.
package canarydeployer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	TeamSlug           = "nais-verification"
	ServiceAccountName = "kafka-kanary"
	RoleName           = "Team member" // RoleName grants k8s_resources:apply (among other team-member rights).

	teamPurpose               = "A place for Nais to run verification workloads"
	teamChannel               = "#nais"
	serviceAccountDescription = "Service account used by the kafka canary deployer"
)

type Config struct {
	GRPCTarget string
	APIBaseURL string

	CanaryImage             string
	CanaryDeployConfigsPath string
}

func ConfigFromEnv() (*Config, error) {
	return &Config{
		GRPCTarget:              os.Getenv("GRPC_TARGET"),
		APIBaseURL:              strings.TrimSuffix(os.Getenv("API_BASE_URL"), "/"),
		CanaryImage:             os.Getenv("CANARY_IMAGE"),
		CanaryDeployConfigsPath: os.Getenv("DEPLOY_CONFIGS_PATH"),
	}, nil
}

func (c *Config) validateProvision() error {
	if c.GRPCTarget == "" {
		return fmt.Errorf("GRPC_TARGET is required")
	}

	return nil
}

func (c *Config) validateDeploy() error {
	if c.GRPCTarget == "" {
		return fmt.Errorf("GRPC_TARGET is required")
	}
	if c.APIBaseURL == "" {
		return fmt.Errorf("API_BASE_URL is required")
	}
	if c.CanaryImage == "" {
		return fmt.Errorf("CANARY_IMAGE is required")
	}
	if c.CanaryDeployConfigsPath == "" {
		return fmt.Errorf("DEPLOY_CONFIGS_PATH is required")
	}

	return nil
}

// tokenExpiry returns tomorrow, the day-granular expiry for the short-lived
// token minted per deploy run.
func tokenExpiry(now time.Time) time.Time {
	return now.Add(24 * time.Hour)
}

// DeployConfig describes where to deploy the canary and its topics for a
// single cluster.
type DeployConfig struct {
	Pool          string
	CanaryCluster string
	TopicCluster  string
}

// deployConfigEntry mirrors an entry in deploy_configs.json.
type deployConfigEntry struct {
	Kind         string `json:"kind"`
	ClusterName  string `json:"cluster_name"`
	AivenProject string `json:"aiven_project"`
	Tenant       string `json:"tenant"`
}

// LoadDeployConfigs reads and interprets deploy_configs.json For the "nav"
// tenant, the topic cluster is looked up from entries that are themselves
// tenant-kind with an aiven project; for other tenants each entry maps its
// own aiven project to its cluster.
func LoadDeployConfigs(path string) ([]DeployConfig, error) {
	safePath := filepath.Clean(path)
	raw, err := os.ReadFile(safePath)
	if err != nil {
		return nil, fmt.Errorf("reading deploy configs: %w", err)
	}

	var entries []deployConfigEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("parsing deploy configs: %w", err)
	}
	if len(entries) == 0 {
		return nil, nil
	}

	lookup := topicClusterLookup(entries)

	var configs []DeployConfig
	for _, e := range entries {
		pool := e.AivenProject
		if pool == "" {
			continue
		}
		topicCluster, ok := lookup[pool]
		if !ok || topicCluster == "" {
			continue
		}
		configs = append(configs, DeployConfig{
			Pool:          pool,
			CanaryCluster: e.ClusterName,
			TopicCluster:  topicCluster,
		})
	}
	return configs, nil
}

// topicClusterLookup maps aiven project -> topic cluster. For "nav" only
// tenant-kind entries with an aiven project are eligible topic clusters.
func topicClusterLookup(entries []deployConfigEntry) map[string]string {
	lookup := map[string]string{}
	nav := entries[0].Tenant == "nav"
	for _, e := range entries {
		if e.AivenProject == "" {
			continue
		}
		if nav && e.Kind != "tenant" {
			continue
		}
		lookup[e.AivenProject] = e.ClusterName
	}

	return lookup
}
