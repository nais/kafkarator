package kafkanativeaclclient

import (
	"context"
	"fmt"
	"strings"

	nativekafkaclient "github.com/aiven/go-client-codegen"
	"github.com/aiven/go-client-codegen/handler/kafka"
	"github.com/nais/kafkarator/pkg/aiven/acl"
	log "github.com/sirupsen/logrus"
)

const (
	NativeAclAllow = "ALLOW"

	NativeAclAll      = "All"
	NativeAclDescribe = "Describe"
	NativeAclRead     = "Read"
	NativeAclWrite    = "Write"
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

	acls := make([]*acl.Acl, 0, len(out.KafkaAcl))
	for _, aclOut := range out.KafkaAcl {
		var idPtr *string
		if aclOut.Id != "" {
			idPtr = &aclOut.Id
		}

		permission, err := MapKafkaNativePermissionToAivenPermission(string(aclOut.Operation))
		if err != nil {
			return nil, err
		}

		converted := &kafka.AclOut{
			Id:         idPtr,
			Permission: kafka.PermissionType(*permission),
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

func (c *AclClient) Create(ctx context.Context, project, service string, req acl.CreateKafkaACLRequest) ([]*acl.Acl, error) {
	host := "*"

	kafkaNativeAcls := MapPermissionToKafkaNativePermission(req.Permission)
	aivenAcls := make([]kafka.ServiceKafkaNativeAclAddOut, 0, len(kafkaNativeAcls))
	for _, nativeAcl := range kafkaNativeAcls {
		in := &kafka.ServiceKafkaNativeAclAddIn{
			Host:           &host,
			Operation:      nativeAcl.Operation,
			PatternType:    kafka.PatternTypePrefixed,
			PermissionType: kafka.ServiceKafkaNativeAclPermissionType(nativeAcl.PermissionType),
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
		log.Debug("Creating Kafka NativeAclAddOut ", out)
		aivenAcls = append(aivenAcls, *out)
	}

	kafkaratorAcls := make([]*acl.Acl, 0, len(aivenAcls))
	for _, nativeAcl := range aivenAcls {
		log.Info("Creating Kafka NativeAclAddOut ", nativeAcl)

		permission, err := MapKafkaNativePermissionToAivenPermission(string(nativeAcl.Operation))
		if err != nil {
			return nil, err
		}

		aivenAcl := &acl.Acl{
			ID:         nativeAcl.Id,
			Permission: *permission,
			Topic:      nativeAcl.ResourceName,
			Username:   strings.TrimPrefix(nativeAcl.Principal, "User:"),
		}
		kafkaratorAcls = append(kafkaratorAcls, aivenAcl)
	}

	return kafkaratorAcls, nil
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
func MapPermissionToKafkaNativePermission(permission string) []nativeAcl {
	switch permission {
	case "write":
		return []nativeAcl{
			{
				Operation:      NativeAclWrite,
				PermissionType: NativeAclAllow,
			},
			{
				Operation:      NativeAclDescribe,
				PermissionType: NativeAclAllow,
			},
		}
	case "read":
		return []nativeAcl{
			{
				Operation:      NativeAclRead,
				PermissionType: NativeAclAllow,
			},
			{
				Operation:      NativeAclDescribe,
				PermissionType: NativeAclAllow,
			},
		}
	case "admin":
		return []nativeAcl{
			{
				Operation:      NativeAclAll,
				PermissionType: NativeAclAllow,
			},
		}
	case "readwrite":
		return []nativeAcl{
			{
				Operation:      NativeAclDescribe,
				PermissionType: NativeAclAllow,
			},
			{
				Operation:      NativeAclRead,
				PermissionType: NativeAclAllow,
			},
			{
				Operation:      NativeAclWrite,
				PermissionType: NativeAclAllow,
			},
		}
	default:
		return []nativeAcl{} // fallback
	}
}

// MapKafkaNativePermissionToAivenPermission maps Aiven API operation to custom permission string
func MapKafkaNativePermissionToAivenPermission(operation string) (*string, error) {
	var aivenAcl string

	switch operation {
	case NativeAclWrite:
		aivenAcl = "write"
		return &aivenAcl, nil
	case NativeAclRead:
		aivenAcl = "read"
		return &aivenAcl, nil
	case NativeAclAll:
		aivenAcl = "admin"
		return &aivenAcl, nil
	default:
		return nil, fmt.Errorf("Kafka Native ACL not mapped to Aiven ACL: %s", operation)
	}
}
