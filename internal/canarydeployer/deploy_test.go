package canarydeployer

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfigs(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "deploy_configs.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadDeployConfigsNav(t *testing.T) {
	// For Nav, only tenant-kind entries are eligible topic clusters.
	path := writeConfigs(t, `[
		{"kind":"tenant","cluster_name":"prod-gcp","aiven_project":"nav-prod","tenant":"nav"},
		{"kind":"onprem","cluster_name":"prod-fss","aiven_project":"nav-prod","tenant":"nav"}
	]`)

	configs, err := LoadDeployConfigs(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 2 {
		t.Fatalf("expected 2 configs, got %d", len(configs))
	}
	// Both entries share pool nav-prod; topic cluster must be the tenant-kind one.
	for _, c := range configs {
		if c.TopicCluster != "prod-gcp" {
			t.Errorf("topic cluster = %q, want prod-gcp", c.TopicCluster)
		}
	}
}

func TestLoadDeployConfigsOtherTenant(t *testing.T) {
	path := writeConfigs(t, `[
		{"kind":"tenant","cluster_name":"dev","aiven_project":"acme-dev","tenant":"acme"}
	]`)

	configs, err := LoadDeployConfigs(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}
	c := configs[0]
	if c.Pool != "acme-dev" || c.CanaryCluster != "dev" || c.TopicCluster != "dev" {
		t.Errorf("unexpected config: %+v", c)
	}
}

func TestRenderCanary(t *testing.T) {
	cfg := &Config{Image: "example.com/canary:1"}
	dc := DeployConfig{Pool: "nav-dev", CanaryCluster: "dev-gcp", TopicCluster: "dev-gcp"}
	m, err := renderCanary(cfg, dc, "2026-10-21T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if m["kind"] != "Application" {
		t.Errorf("kind = %v", m["kind"])
	}
	meta := m["metadata"].(map[string]any)
	if meta["namespace"] != TeamSlug {
		t.Errorf("namespace = %v, want %s", meta["namespace"], TeamSlug)
	}
	spec := m["spec"].(map[string]any)
	if spec["image"] != "example.com/canary:1" {
		t.Errorf("image = %v", spec["image"])
	}
}

func TestRenderTopic(t *testing.T) {
	dc := DeployConfig{Pool: "nav-dev", CanaryCluster: "dev-gcp", TopicCluster: "dev-gcp"}
	m, err := renderTopic(dc, "kafka-canary-dev-gcp")
	if err != nil {
		t.Fatal(err)
	}
	if m["kind"] != "Topic" {
		t.Errorf("kind = %v", m["kind"])
	}
	meta := m["metadata"].(map[string]any)
	if meta["name"] != "kafka-canary-dev-gcp" {
		t.Errorf("name = %v", meta["name"])
	}
}
