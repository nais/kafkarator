package kafkanativeaclclient

import (
	"context"
	"strings"

	nativekafkaclient "github.com/aiven/go-client-codegen"
	"github.com/aiven/go-client-codegen/handler/kafka"
	"github.com/nais/kafkarator/pkg/aiven/acl"
)

type AclClient struct {
	nativekafkaclient.Client
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

func (c *AclClient) Create(ctx context.Context, project, service string, req acl.CreateKafkaACLRequest) (*acl.Acl, error) {
	operation, permType := MapPermissionToKafkaNativePermission(req.Permission)
	host := "*"
	in := &kafka.ServiceKafkaNativeAclAddIn{
		Host:           &host,
		Operation:      kafka.OperationType(operation),
		PatternType:    kafka.PatternTypeLiteral,
		PermissionType: kafka.ServiceKafkaNativeAclPermissionType(permType),
		Principal:      "User:" + req.Username,
		ResourceName:   req.Topic,
		ResourceType:   kafka.ResourceTypeTopic,
	}
	out, err := c.ServiceKafkaNativeAclAdd(ctx, project, service, in)
	if err != nil {
		return nil, err
	}

	MapKafkaNativePermissionToAivenPermission(string(out.Operation))
	// Construct *acl.Acl from the out object fields directly
	return &acl.Acl{
		ID:         out.Id,
		Permission: MapKafkaNativePermissionToAivenPermission(string(out.Operation)),
		Topic:      out.ResourceName,
		Username:   strings.TrimPrefix(out.Principal, "User:"),
	}, nil
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

func makeAcl(aclOut *kafka.AclOut) *acl.Acl {
	return &acl.Acl{
		ID:         valueOrEmpty(aclOut.Id),
		Permission: string(aclOut.Permission),
		Topic:      aclOut.Topic,
		Username:   aclOut.Username,
	}
}

// MapPermissionToKafkaNativePermission maps custom permission strings to OperationType and PermissionType
func MapPermissionToKafkaNativePermission(permission string) (operation, permType string) {
	switch permission {
	case "write":
		return "WRITE", "ALLOW"
	case "read":
		return "READ", "ALLOW"
	case "admin":
		return "ALL", "ALLOW"
	case "readwrite":
		return "ALTER", "ALLOW"
	default:
		return "READ", "ALLOW" // fallback
	}
}

// MapKafkaNativePermissionToAivenPermission maps Kafka-native OperationType to custom Aiven permission string
func MapKafkaNativePermissionToAivenPermission(operation string) string {
	switch operation {
	case "WRITE":
		return "write"
	case "READ":
		return "read"
	case "ALL":
		return "admin"
	case "ALTER":
		return "readwrite"
	default:
		return "read" // fallback
	}
}
