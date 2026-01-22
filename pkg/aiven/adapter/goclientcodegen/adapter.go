package goclientcodegen

import (
	"context"
	"fmt"

	generatedclient "github.com/aiven/go-client-codegen"
	"github.com/aiven/go-client-codegen/handler/kafka"
	"github.com/nais/kafkarator/pkg/aiven/acl"
	log "github.com/sirupsen/logrus"
)

type AclClient struct {
	generatedclient.Client
}

func (c *AclClient) List(ctx context.Context, project, serviceName string) (acl.ExistingAcls, error) {
	out, err := c.ServiceKafkaNativeAclList(ctx, project, serviceName)
	if err != nil {
		return nil, err
	}

	acls := make(acl.ExistingAcls, 0, len(out.Acl))
	for _, aclOut := range out.Acl {
		acls = append(acls, acl.ExistingAcl{
			Acl: acl.Acl{
				ID:         valueOrEmpty(aclOut.Id),
				Permission: string(aclOut.Permission),
				Topic:      aclOut.Topic,
				Username:   aclOut.Username,
			},
		})
	}
	return acls, nil
}

func (c *AclClient) Create(ctx context.Context, project, service string, req acl.CreateKafkaACLRequest) ([]*acl.Acl, error) {
	in := &kafka.ServiceKafkaAclAddIn{
		Permission: kafka.PermissionType(req.Permission),
		Topic:      req.Topic,
		Username:   req.Username,
	}
	out, err := c.ServiceKafkaAclAdd(ctx, project, service, in)
	if err != nil {
		return nil, err
	}

	// The server doesn't return the ACL we created but list of all ACLs currently
	// defined. Need to find the correct one manually. There could be multiple ACLs
	// with same attributes. Assume the one that was created is the last one matching.
	var foundACL *kafka.AclOut
	for _, aclOut := range out {
		if aclOut.Permission == in.Permission && aclOut.Topic == in.Topic && aclOut.Username == in.Username {
			foundACL = &aclOut
		}
	}

	if foundACL == nil {
		return nil, fmt.Errorf("created ACL not found from response ACL list")
	}

	return []*acl.Acl{makeAcl(foundACL)}, nil
}

func (c *AclClient) Delete(ctx context.Context, project, service, aclID string) error {
	log.Info("Deleting Aiven Acl with ID (goclientcodegen)", aclID, " from service ", service, " in project ", project)
	_, err := c.ServiceKafkaAclDelete(ctx, project, service, aclID)
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
