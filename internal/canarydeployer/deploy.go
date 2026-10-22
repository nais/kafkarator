package canarydeployer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"text/template"
	"time"

	_ "embed"

	"github.com/nais/api/pkg/apiclient"
	"github.com/nais/api/pkg/apiclient/protoapi"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
	"sigs.k8s.io/yaml"
)

//go:embed canary.yaml.tmpl
var canaryManifestTmpl string

//go:embed topic.yaml.tmpl
var topicManifestTmpl string

type applyResponse struct {
	Results []struct {
		Resource        string `json:"resource"`
		EnvironmentName string `json:"environmentName"`
		Status          string `json:"status"`
		Error           string `json:"error,omitempty"`
	} `json:"results"`
}

// Deploy mints a short-lived token and applies the canary topics and
// application to every configured cluster.
func Deploy(ctx context.Context, cfg *Config) error {
	if err := cfg.validateDeploy(); err != nil {
		return err
	}

	configs, err := LoadDeployConfigs(cfg.CanaryDeployConfigsPath)
	if err != nil {
		return err
	}
	if len(configs) == 0 {
		log.Warn("no deploy configs found, nothing to do")
		return nil
	}

	token, err := mintToken(ctx, cfg)
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)

	var failed []string
	for _, dc := range configs {
		if err := deployTopics(ctx, cfg, token, dc); err != nil {
			log.Errorf("deploying topics for %q: %s", dc.CanaryCluster, err)
			failed = append(failed, dc.CanaryCluster+" (topics)")
			continue
		}

		if err := deployCanary(ctx, cfg, token, dc, now); err != nil {
			log.Errorf("deploying canary to %q: %s", dc.CanaryCluster, err)
			failed = append(failed, dc.CanaryCluster)
			continue
		}

		log.Infof("deployed canary and topics for %q", dc.CanaryCluster)
	}

	if len(failed) != 0 {
		return fmt.Errorf("canary failed to deploy to: %s", strings.Join(failed, ", "))
	}

	return nil
}

func deployTopics(ctx context.Context, cfg *Config, token string, dc DeployConfig) error {
	topics := []string{
		fmt.Sprintf("kafka-canary-%s", dc.CanaryCluster),
		fmt.Sprintf("kafka-canary-tx-%s", dc.CanaryCluster),
	}

	for _, topic := range topics {
		manifest, err := renderTopic(dc, topic)
		if err != nil {
			return fmt.Errorf("rendering topic %q: %w", topic, err)
		}

		if err := applyToEnvironment(ctx, cfg, token, dc.TopicCluster, manifest); err != nil {
			return fmt.Errorf("applying topic %q to %q: %w", topic, dc.TopicCluster, err)
		}
	}

	return nil
}

func deployCanary(ctx context.Context, cfg *Config, token string, dc DeployConfig, now string) error {
	manifest, err := renderCanary(cfg, dc, now)
	if err != nil {
		return fmt.Errorf("rendering canary manifest: %w", err)
	}

	return applyToEnvironment(ctx, cfg, token, dc.CanaryCluster, manifest)
}

func mintToken(ctx context.Context, cfg *Config) (string, error) {
	client, err := apiclient.New(cfg.GRPCTarget, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return "", fmt.Errorf("connecting to Nais api: %w", err)
	}
	defer func() { _ = client.Close() }()

	saResp, err := ensureServiceAccount(ctx, client)
	if err != nil {
		return "", err
	}

	tokenResp, err := client.ServiceAccounts().CreateToken(ctx, protoapi.CreateServiceAccountTokenRequest_builder{
		ServiceAccountId: saResp.GetId(),
		Name:             "kafka-kanary-" + strconv.FormatInt(time.Now().Unix(), 10),
		Description:      "Short-lived token for a kafka canary deploy run",
		ExpiresAt:        timestamppb.New(tokenExpiry(time.Now())),
	}.Build())
	if err != nil {
		return "", fmt.Errorf("minting token: %w", err)
	}

	return tokenResp.GetSecret(), nil
}

type topicVars struct {
	Team      string
	Pool      string
	TopicName string
}

func renderTopic(dc DeployConfig, topicName string) (map[string]any, error) {
	return renderManifest(topicManifestTmpl, topicVars{
		Team:      TeamSlug,
		Pool:      dc.Pool,
		TopicName: topicName,
	})
}

type canaryVars struct {
	Team                    string
	Image                   string
	Pool                    string
	Now                     string
	CanaryKafkaTopic        string
	CanaryKafkaTxTopic      string
	CanaryEnableTransaction string
	GroupID                 string
}

func renderCanary(cfg *Config, dc DeployConfig, now string) (map[string]any, error) {
	return renderManifest(canaryManifestTmpl, canaryVars{
		Team:               TeamSlug,
		Image:              cfg.CanaryImage,
		Pool:               dc.Pool,
		Now:                now,
		CanaryKafkaTopic:   fmt.Sprintf("%s.kafka-canary-%s", TeamSlug, dc.CanaryCluster),
		CanaryKafkaTxTopic: fmt.Sprintf("%s.kafka-canary-tx-%s", TeamSlug, dc.CanaryCluster),
		GroupID:            dc.CanaryCluster,
	})
}

func renderManifest(tmplText string, vars any) (map[string]any, error) {
	tmpl, err := template.New("manifest").Parse(tmplText)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return nil, err
	}

	var manifest map[string]any
	if err := yaml.Unmarshal(buf.Bytes(), &manifest); err != nil {
		return nil, err
	}
	return manifest, nil
}

func applyToEnvironment(ctx context.Context, cfg *Config, token, env string, manifest map[string]any) error {
	url := fmt.Sprintf("%s/api/v1/teams/%s/environments/%s/apply", cfg.APIBaseURL, TeamSlug, env)

	payload := struct {
		Resources []map[string]any `json:"resources"`
	}{
		Resources: []map[string]any{manifest},
	}

	body := &bytes.Buffer{}
	if err := json.NewEncoder(body).Encode(payload); err != nil {
		return fmt.Errorf("encoding apply request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return fmt.Errorf("building apply request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending apply request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("apply failed with HTTP status %d", resp.StatusCode)
	}

	var applyResp applyResponse
	if err := json.NewDecoder(resp.Body).Decode(&applyResp); err != nil {
		return fmt.Errorf("decoding apply response: %w", err)
	}

	for _, r := range applyResp.Results {
		if r.Error != "" {
			return fmt.Errorf("resource %q: %s", r.Resource, r.Error)
		}
	}

	return nil
}
