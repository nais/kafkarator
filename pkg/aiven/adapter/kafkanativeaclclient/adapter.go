package kafkanativeaclclient

import (
	"context"
	"strings"

	nativekafkaclient "github.com/aiven/go-client-codegen"
	"github.com/aiven/go-client-codegen/handler/kafka"
	"github.com/nais/kafkarator/pkg/aiven/acl"
	log "github.com/sirupsen/logrus"
)

const (
	NATIVE_ACL_ALLOW = "ALLOW"

	NATIVE_ACL_READ  = "Read"
	NATIVE_ACL_WRITE = "Write"
	NATIVE_ACL_ALL   = "All"
)

type AclClient struct {
	nativekafkaclient.Client
}

type nativeAcl struct {
	Operation      kafka.OperationType
	PermissionType kafka.PermissionType
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

func (c *AclClient) Create(ctx context.Context, project, service string, req acl.CreateKafkaACLRequest) ([]*acl.Acl, error) {
	host := "*"

	kafkaNativeAcls := MapPermissionToKafkaNativePermission(req.Permission)
	aivenAcls := make([]kafka.ServiceKafkaNativeAclAddOut, 0, len(kafkaNativeAcls))
	for _, nativeAcl := range kafkaNativeAcls {
		in := &kafka.ServiceKafkaNativeAclAddIn{
			Host:           &host,
			Operation:      kafka.OperationType(nativeAcl.Operation),
			PatternType:    kafka.PatternTypeLiteral,
			PermissionType: kafka.ServiceKafkaNativeAclPermissionType(nativeAcl.PermissionType),
			Principal:      "User:" + req.Username,
			ResourceName:   req.Topic,
			ResourceType:   kafka.ResourceTypeTopic,
		}
		log.Debug("Creating Kafka NativeAclAddIn ", in)
		out, err := c.ServiceKafkaNativeAclAdd(ctx, project, service, in)
		if err != nil {
			return nil, err
		}
		log.Debug("Creating Kafka NativeAclAddOut ", out)
		aivenAcls = append(aivenAcls, *out)
	}

	kafkaratorAcls := make([]*acl.Acl, 0, len(aivenAcls))
	for _, nativeAcl := range aivenAcls {
		aivenAcl := &acl.Acl{
			ID:         nativeAcl.Id,
			Permission: MapKafkaNativePermissionToAivenPermission(string(nativeAcl.Operation)),
			Topic:      nativeAcl.ResourceName,
			Username:   strings.TrimPrefix(nativeAcl.Principal, "User:"),
		}
		kafkaratorAcls = append(kafkaratorAcls, aivenAcl)
	}
	return kafkaratorAcls, nil
}

func (c *AclClient) Delete(ctx context.Context, project, service, aclID string) error {
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
func MapPermissionToKafkaNativePermission(permission string) []nativeAcl {
	switch permission {
	case "write":
		return []nativeAcl{{
			Operation:      NATIVE_ACL_WRITE,
			PermissionType: NATIVE_ACL_ALLOW,
		}}
	case "read":
		return []nativeAcl{{
			Operation:      NATIVE_ACL_READ,
			PermissionType: NATIVE_ACL_ALLOW,
		}}
	case "admin":
		return []nativeAcl{{
			Operation:      NATIVE_ACL_ALL,
			PermissionType: NATIVE_ACL_ALLOW,
		}}
	case "readwrite":
		return []nativeAcl{
			{
				Operation:      NATIVE_ACL_READ,
				PermissionType: NATIVE_ACL_ALLOW,
			},
			{
				Operation:      NATIVE_ACL_WRITE,
				PermissionType: NATIVE_ACL_ALLOW,
			},
		}
	default:
		return []nativeAcl{} // fallback
	}
}

// MapKafkaNativePermissionToAivenPermission maps Aiven API operation (capitalized) to custom permission string
func MapKafkaNativePermissionToAivenPermission(operation string) string {
	switch operation {
	case NATIVE_ACL_WRITE:
		return "write"
	case NATIVE_ACL_READ:
		return "read"
	case NATIVE_ACL_ALL:
		return "admin"
	// case "Alter":	// TODO
	// 	return "readwrite"
	default:
		return "read" // fallback
	}
}
