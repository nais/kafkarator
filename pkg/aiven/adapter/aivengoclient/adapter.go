package aivengoclient

import (
	"context"
	"github.com/aiven/aiven-go-client/v2"
)

type AclClient struct {
	*aiven.KafkaACLHandler
}

func (c *AclClient) List(ctx context.Context, project, serviceName string) ([]*aiven.KafkaACL, error) {
	return c.KafkaACLHandler.List(ctx, project, serviceName)
}

func (c *AclClient) Create(ctx context.Context, project, service string, req aiven.CreateKafkaACLRequest) (*aiven.KafkaACL, error) {
	return c.KafkaACLHandler.Create(ctx, project, service, req)
}

func (c *AclClient) Delete(ctx context.Context, project, service, aclID string) error {
	return c.KafkaACLHandler.Delete(ctx, project, service, aclID)
}
