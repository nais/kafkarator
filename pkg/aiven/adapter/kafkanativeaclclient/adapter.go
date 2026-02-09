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

// his key groups native ACL entries into one logical permission
type nativeAclGroupKey struct {
	Host           string
	PermissionType kafka.KafkaAclPermissionType
	Principal      string
	ResourceType   kafka.ResourceType
	ResourceName   string
	PatternType    kafka.PatternType
}

// This is a group of native ACLs that together form one Aiven ACL.
type nativeAclGroup struct {
	operations     map[kafka.OperationType]struct{}
	idsByOperation map[kafka.OperationType]string

	username string
	topic    string
}

func (g *nativeAclGroup) addOperation(op kafka.OperationType, id string) {
	if g.operations == nil {
		g.operations = make(map[kafka.OperationType]struct{}, 8)
	}
	if g.idsByOperation == nil {
		g.idsByOperation = make(map[kafka.OperationType]string, 8)
	}

	g.operations[op] = struct{}{}

	if id != "" {
		if _, exists := g.idsByOperation[op]; !exists {
			g.idsByOperation[op] = id
		}
	}
}

func (g *nativeAclGroup) hasOperation(op kafka.OperationType) bool {
	_, ok := g.operations[op]
	return ok
}

func resolveAivenPermission(g *nativeAclGroup) *kafka.PermissionType {
	hasDescribe := g.hasOperation(kafka.OperationTypeDescribe)

	// The set of permissions that map to a "admin" role
	adminOps := []kafka.OperationType{
		kafka.OperationTypeRead,
		kafka.OperationTypeWrite,
		kafka.OperationTypeDescribe,
		kafka.OperationTypeDescribeConfigs,
		kafka.OperationTypeAlterConfigs,
		kafka.OperationTypeDelete,
	}

	// Check if admin
	isAdmin := true
	if hasDescribe {
		for _, op := range adminOps {
			if !g.hasOperation(op) {
				isAdmin = false
				break
			}
		}
	}

	var permission kafka.PermissionType
	if isAdmin {
		permission = kafka.PermissionTypeAdmin
		return &permission
	}

	hasRead := g.hasOperation(kafka.OperationTypeRead)
	hasWrite := g.hasOperation(kafka.OperationTypeWrite)

	switch {
	case hasDescribe && hasRead && hasWrite:
		permission = kafka.PermissionTypeReadwrite
	case hasDescribe && hasRead:
		permission = kafka.PermissionTypeRead
	case hasDescribe && hasWrite:
		permission = kafka.PermissionTypeWrite
	default:
		return nil
	}

	return &permission
}

type nativeAcl struct {
	Operation      kafka.OperationType
	PermissionType kafka.ServiceKafkaNativeAclPermissionType
}

func (c *AclClient) List(ctx context.Context, project, serviceName string) ([]*acl.Acl, error) {
	out, err := c.ServiceKafkaNativeAclList(ctx, project, serviceName)
	if err != nil {
		return nil, err
	}

	nativeAclGroups := make(map[nativeAclGroupKey]*nativeAclGroup)

	for _, nAcl := range out.KafkaAcl {
		groupKey := nativeAclGroupKey{
			Host:           nAcl.Host,
			PermissionType: nAcl.PermissionType,
			Principal:      nAcl.Principal,
			ResourceType:   nAcl.ResourceType,
			ResourceName:   nAcl.ResourceName,
			PatternType:    nAcl.PatternType,
		}

		group := nativeAclGroups[groupKey]
		if group == nil {
			group = &nativeAclGroup{
				username: strings.TrimPrefix(nAcl.Principal, "User:"),
				topic:    nAcl.ResourceName,
			}
			nativeAclGroups[groupKey] = group
		}

		group.addOperation(nAcl.Operation, nAcl.Id)
	}

	resolvedAivenAcls := make([]*acl.Acl, 0, len(nativeAclGroups))
	for groupKey, group := range nativeAclGroups {
		permission := resolveAivenPermission(group)
		if permission == nil {
			continue
		}

		nativeIDs := nativeIDsForPermission(group, *permission)
		permissionStr := string(*permission)

		resolvedAivenAcls = append(resolvedAivenAcls, &acl.Acl{
			Permission: permissionStr,
			Topic:      group.topic,
			Username:   group.username,
			IDs:        nativeIDs,
		})

		log.WithFields(log.Fields{
			"principal":       groupKey.Principal,
			"resource_type":   groupKey.ResourceType,
			"resource_name":   groupKey.ResourceName,
			"pattern_type":    groupKey.PatternType,
			"host":            groupKey.Host,
			"permission_type": groupKey.PermissionType,
			"aiven_perm":      permissionStr,
			"native_ids":      nativeIDs,
		}).Debug("Coalesced Kafka native ACL group into one Aiven ACL")
	}

	return resolvedAivenAcls, nil
}

func (c *AclClient) Create(ctx context.Context, project, service string, isStream bool, req acl.CreateKafkaACLRequest) error {
	host := "*"

	// Topic pattern: literal for Topic, prefixed for Stream
	topicPattern := kafka.PatternTypeLiteral
	if isStream {
		topicPattern = kafka.PatternTypePrefixed
	}

	// Topic ACLs
	desiredTopicAcls := MapPermissionToKafkaNativePermission(kafka.PermissionType(req.Permission))
	for _, desired := range desiredTopicAcls {
		in := &kafka.ServiceKafkaNativeAclAddIn{
			Host:           &host,
			Operation:      desired.Operation,
			PatternType:    topicPattern,
			PermissionType: desired.PermissionType,
			Principal:      "User:" + req.Username,
			ResourceName:   req.Topic,
			ResourceType:   kafka.ResourceTypeTopic,
		}

		if _, err := c.ServiceKafkaNativeAclAdd(ctx, project, service, in); err != nil {
			if nativekafkaclient.IsAlreadyExists(err) {
				continue
			}
			return err
		}
	}

	// TransactionalId
	perm := kafka.PermissionType(req.Permission)
	txnPattern := kafka.PatternTypePrefixed
	txnName := transactionalIDName(req.Username)
	if needsTransactionalACL(perm) {

		// TransactionalId ACLs
		for _, desired := range transactionalNativeACLs() {
			in := &kafka.ServiceKafkaNativeAclAddIn{
				Host:           &host,
				Operation:      desired.Operation,
				PatternType:    txnPattern,
				PermissionType: desired.PermissionType,
				Principal:      "User:" + req.Username,
				ResourceName:   txnName,
				ResourceType:   kafka.ResourceTypeTransactionalId,
			}

			if _, err := c.ServiceKafkaNativeAclAdd(ctx, project, service, in); err != nil {
				if nativekafkaclient.IsAlreadyExists(err) {
					continue
				}
				return err
			}
		}

		log.WithFields(log.Fields{
			"username":   req.Username,
			"permission": req.Permission,
			"txn_prefix": txnName,
		}).Info("created Kafka native TransactionalId ACL entries")
	}

	// Group ACLs
	for _, desired := range groupNativeACLs() {
		in := &kafka.ServiceKafkaNativeAclAddIn{
			Host:           &host,
			Operation:      desired.Operation,
			PatternType:    txnPattern,
			PermissionType: desired.PermissionType,
			Principal:      "User:" + req.Username,
			ResourceName:   req.Topic,
			ResourceType:   kafka.ResourceTypeGroup,
		}

		if _, err := c.ServiceKafkaNativeAclAdd(ctx, project, service, in); err != nil {
			if nativekafkaclient.IsAlreadyExists(err) {
				continue
			}
			return err
		}

		log.WithFields(log.Fields{
			"username":     req.Username,
			"permission":   req.Permission,
			"group_prefix": req.Topic,
		}).Info("created Kafka native Group ACL entries")
	}

	log.WithFields(log.Fields{
		"username":   req.Username,
		"permission": req.Permission,
		"topic":      req.Topic,
	}).Info("created Kafka native ACL entries")
	return nil
}

func (c *AclClient) Delete(ctx context.Context, project, service string, acl acl.Acl) error {
	for _, id := range acl.IDs {
		log.WithFields(log.Fields{
			"username":   acl.Username,
			"topic":      acl.Topic,
			"permission": acl.Permission,
			"id":         id,
		}).Info("deleting Kafka native ACL entry")

		if err := c.ServiceKafkaNativeAclDelete(ctx, project, service, id); err != nil {
			return err
		}
	}
	return nil
}

func nativeOpsForPermission(permission kafka.PermissionType) []kafka.OperationType {
	defs := MapPermissionToKafkaNativePermission(permission)
	ops := make([]kafka.OperationType, 0, len(defs))
	for _, d := range defs {
		ops = append(ops, d.Operation)
	}
	return ops
}

func nativeIDsForPermission(group *nativeAclGroup, permission kafka.PermissionType) []string {
	ops := nativeOpsForPermission(permission)
	ids := make([]string, 0, len(ops))
	for _, op := range ops {
		if group.idsByOperation == nil {
			continue
		}
		if id := group.idsByOperation[op]; id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

// MapPermissionToKafkaNativePermission maps custom permission strings to Aiven API operation/permission_type (capitalized operation, uppercase permType)
func MapPermissionToKafkaNativePermission(permission kafka.PermissionType) []nativeAcl {
	var nativeAclList []nativeAcl
	// Describe is required for all permissions
	nativeAclList = append(nativeAclList, nativeAcl{
		Operation:      kafka.OperationTypeDescribe,
		PermissionType: kafka.ServiceKafkaNativeAclPermissionTypeAllow,
	})
	switch permission {
	case kafka.PermissionTypeWrite:
		nativeAclList = append(nativeAclList, nativeAcl{
			Operation:      kafka.OperationTypeWrite,
			PermissionType: kafka.ServiceKafkaNativeAclPermissionTypeAllow,
		})
	case kafka.PermissionTypeRead:
		nativeAclList = append(nativeAclList, nativeAcl{
			Operation:      kafka.OperationTypeRead,
			PermissionType: kafka.ServiceKafkaNativeAclPermissionTypeAllow,
		})
	case kafka.PermissionTypeAdmin:
		nativeAclList = append(nativeAclList,
			nativeAcl{
				Operation:      kafka.OperationTypeRead,
				PermissionType: kafka.ServiceKafkaNativeAclPermissionTypeAllow,
			},
			nativeAcl{
				Operation:      kafka.OperationTypeWrite,
				PermissionType: kafka.ServiceKafkaNativeAclPermissionTypeAllow,
			},
			nativeAcl{
				Operation:      kafka.OperationTypeDescribeConfigs,
				PermissionType: kafka.ServiceKafkaNativeAclPermissionTypeAllow,
			},
			nativeAcl{
				Operation:      kafka.OperationTypeAlterConfigs,
				PermissionType: kafka.ServiceKafkaNativeAclPermissionTypeAllow,
			},
			nativeAcl{
				Operation:      kafka.OperationTypeDelete,
				PermissionType: kafka.ServiceKafkaNativeAclPermissionTypeAllow,
			},
		)
	case kafka.PermissionTypeReadwrite:
		nativeAclList = append(nativeAclList,
			nativeAcl{
				Operation:      kafka.OperationTypeRead,
				PermissionType: kafka.ServiceKafkaNativeAclPermissionTypeAllow,
			},
			nativeAcl{
				Operation:      kafka.OperationTypeWrite,
				PermissionType: kafka.ServiceKafkaNativeAclPermissionTypeAllow,
			},
		)
	default:
		return []nativeAcl{}
	}

	return nativeAclList
}

func needsTransactionalACL(permission kafka.PermissionType) bool {
	switch permission {
	case kafka.PermissionTypeWrite, kafka.PermissionTypeReadwrite:
		return true
	default:
		return false
	}
}

func transactionalNativeACLs() []nativeAcl {
	return []nativeAcl{
		{
			Operation:      kafka.OperationTypeDescribe,
			PermissionType: kafka.ServiceKafkaNativeAclPermissionTypeAllow,
		},
		{
			Operation:      kafka.OperationTypeWrite,
			PermissionType: kafka.ServiceKafkaNativeAclPermissionTypeAllow,
		},
	}
}

func groupNativeACLs() []nativeAcl {
	return []nativeAcl{
		{
			Operation:      kafka.OperationTypeRead,
			PermissionType: kafka.ServiceKafkaNativeAclPermissionTypeAllow,
		},
		{
			Operation:      kafka.OperationTypeDescribe,
			PermissionType: kafka.ServiceKafkaNativeAclPermissionTypeAllow,
		},
	}
}

func transactionalIDName(s string) string {
	if i := strings.IndexByte(s, '_'); i > 0 {
		return s[:i]
	}
	return s
}
