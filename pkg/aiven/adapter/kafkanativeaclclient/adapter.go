package kafkanativeaclclient

import (
	"context"
	"strings"

	nativekafkaclient "github.com/aiven/go-client-codegen"
	"github.com/aiven/go-client-codegen/handler/kafka"
	"github.com/nais/kafkarator/pkg/aiven/acl"
	log "github.com/sirupsen/logrus"
)

type AclClient struct {
	nativekafkaclient.Client
}

func (c *AclClient) List(ctx context.Context, project, serviceName string) ([]*acl.Acl, error) {
	out, err := c.ServiceKafkaNativeAclList(ctx, project, serviceName)
	if err != nil {
		return nil, err
	}

	acls := make([]*acl.Acl, 0, len(out.KafkaAcl))
	for _, aclOut := range out.KafkaAcl {
		var idPtr *string
		if aclOut.Id != "" {
			idPtr = &aclOut.Id
		}

		converted := &kafka.AclOut{
			Id:         idPtr,
			Permission: kafka.PermissionType(string(aclOut.Operation)),
			Topic:      aclOut.ResourceName,
			Username:   strings.TrimPrefix(aclOut.Principal, "User:"),
		}
		acls = append(acls, makeAcl(converted))
		if idPtr != nil {
			log.Info("Appending Kafka NativeAcl ", *idPtr)
		} else {
			log.Info("Appending Kafka NativeAcl with nil ID")
		}
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
	log.Info("Creating Kafka NativeAclAddIn ", in)
	out, err := c.ServiceKafkaNativeAclAdd(ctx, project, service, in)
	if err != nil {
		if strings.Contains(err.Error(), "409") && strings.Contains(err.Error(), "Identical ACL entry already exists") {
			log.Info("ACL already exists (string match fallback), skipping creation")
			return nil, nil
		}
		return nil, err
	}
	log.Info("Creating Kafka NativeAclAddOut ", out)
	return &acl.Acl{
		ID:         out.Id,
		Permission: MapKafkaNativePermissionToAivenPermission(string(out.Operation)),
		Topic:      out.ResourceName,
		Username:   strings.TrimPrefix(out.Principal, "User:"),
	}, nil
}

func (c *AclClient) Delete(ctx context.Context, project, service, aclID string) error {
	log.Info("Deleting Kafka NativeAcl with ID ", aclID, " from service ", service, " in project ", project)
	return c.ServiceKafkaNativeAclDelete(ctx, project, service, aclID)
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

// MapPermissionToKafkaNativePermission maps custom permission strings to Aiven API operation/permission_type (capitalized operation, uppercase permType)
func MapPermissionToKafkaNativePermission(permission string) (operation, permType string) {
	switch permission {
	case "write":
		return "Write", "ALLOW"
	case "read":
		return "Read", "ALLOW"
	case "admin":
		return "All", "ALLOW"
	case "readwrite":
		return "Alter", "ALLOW"
	default:
		return "Read", "ALLOW" // fallback
	}
}

// MapKafkaNativePermissionToAivenPermission maps Aiven API operation (capitalized) to custom permission string
func MapKafkaNativePermissionToAivenPermission(operation string) string {
	switch operation {
	case "Write":
		return "write"
	case "Read":
		return "read"
	case "All":
		return "admin"
	case "Alter":
		return "readwrite"
	default:
		return "read" // fallback
	}
}
