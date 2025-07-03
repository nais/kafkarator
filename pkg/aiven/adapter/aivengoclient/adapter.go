package aivengoclient

import (
	"context"
	"github.com/aiven/aiven-go-client/v2"
	"github.com/nais/kafkarator/pkg/aiven/acl"
	"k8s.io/utils/ptr"
)

type AclClient struct {
	*aiven.KafkaACLHandler
}

func (c *AclClient) List(ctx context.Context, project, serviceName string) ([]*acl.Acl, error) {
	out, err := c.KafkaACLHandler.List(ctx, project, serviceName)
	if err != nil {
		return nil, err
	}

	acls := make([]*acl.Acl, 0, len(out))
	for _, aclOut := range out {
		acls = append(acls, ptr.To(acl.FromKafkaACL(aclOut)))
	}
	return acls, nil
}

func (c *AclClient) Create(ctx context.Context, project, service string, req acl.CreateKafkaACLRequest) (*acl.Acl, error) {
	in := aiven.CreateKafkaACLRequest{
		Permission: req.Permission,
		Topic:      req.Topic,
		Username:   req.Username,
	}
	out, err := c.KafkaACLHandler.Create(ctx, project, service, in)
	if err != nil {
		return nil, err
	}

	return ptr.To(acl.FromKafkaACL(out)), nil
}

func (c *AclClient) Delete(ctx context.Context, project, service, aclID string) error {
	return c.KafkaACLHandler.Delete(ctx, project, service, aclID)
}
