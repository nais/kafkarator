// Package canarydeployer deploys the kafka canary (topics + application) to
// each configured cluster via the Nais API, in the "v3" world where the old
// hookd deploy API key is replaced by a short-lived service account token.
//
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
	"strings"
	"time"
)

const (
	// TeamSlug is the team the canary runs as.
	TeamSlug = "nais-verification"

	// ServiceAccountName is the canary's service account within the Nais team.
	ServiceAccountName = "kafka-kanary"

	// RoleName grants k8s_resources:apply (among other team-member rights).
	RoleName = "Team member"

	teamPurpose               = "A place for Nais to run verification workloads"
	teamChannel               = "#nais"
	serviceAccountDescription = "Service account used by the kafka canary deployer"
)

// Config is the environment-driven configuration shared by both entrypoints.
type Config struct {
	GRPCTarget string
	APIBaseURL string

	// Image is the canary application image to deploy.
	Image string

	// DeployConfigsPath points to the JSON file describing per-cluster deploy
	// configurations.
	DeployConfigsPath string
}

func ConfigFromEnv() (*Config, error) {
	return &Config{
		GRPCTarget:        os.Getenv("GRPC_TARGET"),
		APIBaseURL:        strings.TrimSuffix(os.Getenv("API_BASE_URL"), "/"),
		Image:             os.Getenv("IMAGE"),
		DeployConfigsPath: os.Getenv("DEPLOY_CONFIGS_PATH"),
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
	if c.Image == "" {
		return fmt.Errorf("IMAGE is required")
	}
	if c.DeployConfigsPath == "" {
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

// LoadDeployConfigs reads and interprets deploy_configs.json, mirroring the
// Python deployer's load_deploy_configs. For the "nav" tenant, the topic
// cluster is looked up from entries that are themselves tenant-kind with an
// aiven project; for other tenants each entry maps its own aiven project to
// its cluster.
func LoadDeployConfigs(path string) ([]DeployConfig, error) {
	raw, err := os.ReadFile(path)
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
