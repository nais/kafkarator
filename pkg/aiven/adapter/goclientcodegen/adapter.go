package goclientcodegen

import (
	"context"

	generatedclient "github.com/aiven/go-client-codegen"
	"github.com/aiven/go-client-codegen/handler/kafka"
	"github.com/nais/kafkarator/pkg/aiven/acl"
	log "github.com/sirupsen/logrus"
)

type AclClient struct {
	generatedclient.Client
}

func (c *AclClient) List(ctx context.Context, project, serviceName string) ([]*acl.Acl, error) {
	out, err := c.ServiceKafkaNativeAclList(ctx, project, serviceName)
	if err != nil {
		return nil, err
	}

	acls := make([]*acl.Acl, 0, len(out.Acl))
	for _, aclOut := range out.Acl {
		acls = append(acls, makeAcl(&aclOut))
	}
	return acls, nil
}

func (c *AclClient) Create(ctx context.Context, project, service string, isStream bool, req acl.CreateKafkaACLRequest) error {
	in := &kafka.ServiceKafkaAclAddIn{
		Permission: kafka.PermissionType(req.Permission),
		Topic:      req.Topic,
		Username:   req.Username,
	}
	_, err := c.ServiceKafkaAclAdd(ctx, project, service, in)
	if err != nil {
		return err
	}
	return nil
}

func (c *AclClient) Delete(ctx context.Context, project, service string, acl acl.Acl) error {
	log.Info("Deleting Aiven Acl with ID (goclientcodegen)", acl.ID, " from service ", service, " in project ", project)
	_, err := c.ServiceKafkaAclDelete(ctx, project, service, acl.ID)
	return err
}

func valueOrEmpty(in *string) string {
	if in != nil {
		return *in
	}
	return ""
}

func makeAcl(aclOut *kafka.AclOut) *acl.Acl {
	return &acl.Acl{
		ID:         valueOrEmpty(aclOut.Id),
		Permission: string(aclOut.Permission),
		Topic:      aclOut.Topic,
		Username:   aclOut.Username,
	}
}
