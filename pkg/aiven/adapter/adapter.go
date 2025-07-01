package adapter

import (
	"context"
	"fmt"
	"github.com/aiven/aiven-go-client/v2"
	generated_client "github.com/aiven/go-client-codegen"
	"github.com/aiven/go-client-codegen/handler/kafka"
)

type AclClient struct {
	generated_client.Client
}

func (c *AclClient) List(ctx context.Context, project, serviceName string) ([]*aiven.KafkaACL, error) {
	out, err := c.ServiceKafkaNativeAclList(ctx, project, serviceName)
	if err != nil {
		return nil, err
	}

	acls := make([]*aiven.KafkaACL, len(out.Acl))
	for _, aclOut := range out.Acl {
		acls = append(acls, makeKafkaACL(&aclOut))
	}
	return acls, nil
}

func (c *AclClient) Create(ctx context.Context, project, service string, req aiven.CreateKafkaACLRequest) (*aiven.KafkaACL, error) {
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
	for _, acl := range out {
		if acl.Permission == in.Permission && acl.Topic == in.Topic && acl.Username == in.Username {
			foundACL = &acl
		}
	}

	if foundACL == nil {
		return nil, fmt.Errorf("created ACL not found from response ACL list")
	}

	return makeKafkaACL(foundACL), nil
}

func (c *AclClient) Delete(ctx context.Context, project, service, aclID string) error {
	_, err := c.ServiceKafkaAclDelete(ctx, project, service, aclID)
	return err
}

func valueOrEmpty(in *string) string {
	if in != nil {
		return *in
	}
	return ""
}

func makeKafkaACL(aclOut *kafka.AclOut) *aiven.KafkaACL {
	return &aiven.KafkaACL{
		ID:         valueOrEmpty(aclOut.Id),
		Permission: string(aclOut.Permission),
		Topic:      aclOut.Topic,
		Username:   aclOut.Username,
	}
}
