package canarydeployer

import (
	"context"
	"fmt"

	"github.com/nais/api/pkg/apiclient"
	"github.com/nais/api/pkg/apiclient/protoapi"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Provision idempotently ensures the canary team and service account exist.
func Provision(ctx context.Context, cfg *Config) error {
	if err := cfg.validateProvision(); err != nil {
		return err
	}

	client, err := apiclient.New(cfg.GRPCTarget, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("connecting to Nais api: %w", err)
	}
	defer func() { _ = client.Close() }()

	teamResp, err := client.Teams().Create(ctx, &protoapi.CreateTeamRequest{
		Slug:         TeamSlug,
		Purpose:      teamPurpose,
		SlackChannel: teamChannel,
	})
	if err != nil {
		return fmt.Errorf("ensuring team %q: %w", TeamSlug, err)
	}
	log.Infof("team %q ensured", teamResp.GetTeam().GetSlug())

	saResp, err := ensureServiceAccount(ctx, client)
	if err != nil {
		return err
	}
	log.Infof("service account %q ensured (id=%s created=%t)", ServiceAccountName, saResp.GetId(), saResp.GetCreated())

	return nil
}

func ensureServiceAccount(ctx context.Context, client *apiclient.APIClient) (*protoapi.CreateServiceAccountResponse, error) {
	resp, err := client.ServiceAccounts().Create(ctx, protoapi.CreateServiceAccountRequest_builder{
		TeamSlug:    TeamSlug,
		Name:        ServiceAccountName,
		Description: serviceAccountDescription,
		RoleName:    RoleName,
	}.Build())
	if err != nil {
		return nil, fmt.Errorf("ensuring service account %q: %w", ServiceAccountName, err)
	}

	return resp, nil
}
