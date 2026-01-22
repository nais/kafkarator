package aivengoclient

import (
	"context"

	"github.com/aiven/aiven-go-client/v2"
	"github.com/nais/kafkarator/pkg/aiven/acl"
	log "github.com/sirupsen/logrus"
	"k8s.io/utils/ptr"
)

type AclClient struct {
	*aiven.KafkaACLHandler
}

func (c *AclClient) List(ctx context.Context, project, serviceName string) (acl.ExistingAcls, error) {
	out, err := c.KafkaACLHandler.List(ctx, project, serviceName)
	if err != nil {
		return nil, err
	}

	acls := make(acl.ExistingAcls, 0, len(out))
	for _, aclOut := range out {
		acls = append(acls, acl.ExistingAcl{
			Acl: acl.Acl{
				ID:         aclOut.ID,
				Permission: aclOut.Permission,
				Topic:      aclOut.Topic,
				Username:   aclOut.Username,
			},
		})
	}
	return acls, nil
}

func (c *AclClient) Create(ctx context.Context, project, service string, req acl.CreateKafkaACLRequest) ([]*acl.Acl, error) {
	in := aiven.CreateKafkaACLRequest{
		Permission: req.Permission,
		Topic:      req.Topic,
		Username:   req.Username,
	}
	out, err := c.KafkaACLHandler.Create(ctx, project, service, in)
	if err != nil {
		return nil, err
	}

	return []*acl.Acl{ptr.To(acl.FromKafkaACL(out))}, nil
}

func (c *AclClient) Delete(ctx context.Context, project, service, aclID string) error {
	log.Info("Deleting Aiven Acl with ID ", aclID, " from service ", service, " in project ", project)
	return c.KafkaACLHandler.Delete(ctx, project, service, aclID)
}
