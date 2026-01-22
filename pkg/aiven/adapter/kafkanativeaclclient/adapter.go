package kafkanativeaclclient

import (
	"context"
	"fmt"
	"sort"
	"strings"

	nativekafkaclient "github.com/aiven/go-client-codegen"
	"github.com/aiven/go-client-codegen/handler/kafka"
	"github.com/nais/kafkarator/pkg/aiven/acl"
	log "github.com/sirupsen/logrus"
)

type AclClient struct {
	nativekafkaclient.Client
}

func (c *AclClient) List(ctx context.Context, project, serviceName string) (acl.ExistingAcls, error) {
	response, err := c.ServiceKafkaNativeAclList(ctx, project, serviceName)
	if err != nil {
		return nil, err
	}

	type aclGroupKey struct {
		username       string
		topic          string
		patternType    kafka.PatternType
		permissionType kafka.KafkaAclPermissionType
		host           string
	}

	type aclGroupState struct {
		operations     map[kafka.OperationType]struct{}
		idsByOperation map[kafka.OperationType][]string
	}

	groupedACLs := map[aclGroupKey]*aclGroupState{}

	for _, nativeACL := range response.KafkaAcl {
		username := strings.TrimPrefix(nativeACL.Principal, "User:")

		host := ""
		if nativeACL.Host != "" {
			host = nativeACL.Host
		}

		groupKey := aclGroupKey{
			username:       username,
			topic:          nativeACL.ResourceName,
			patternType:    nativeACL.PatternType,
			permissionType: nativeACL.PermissionType,
			host:           host,
		}

		groupState, exists := groupedACLs[groupKey]
		if !exists {
			groupState = &aclGroupState{
				operations:     map[kafka.OperationType]struct{}{},
				idsByOperation: map[kafka.OperationType][]string{},
			}
			groupedACLs[groupKey] = groupState
		}

		groupState.operations[nativeACL.Operation] = struct{}{}

		if nativeACL.Id != "" {
			groupState.idsByOperation[nativeACL.Operation] =
				append(groupState.idsByOperation[nativeACL.Operation], nativeACL.Id)
		}
	}

	existingACLs := make(acl.ExistingAcls, 0, len(groupedACLs))

	for groupKey, groupState := range groupedACLs {
		inferredPermission := inferPermissionFromOps(groupState.operations)
		if inferredPermission == nil {
			// e.g. describe-only / unknown combos
			continue
		}

		requiredOperations := permissionToOps[*inferredPermission]

		nativeIDs := make([]string, 0, 8)
		for operation := range requiredOperations {
			nativeIDs = append(
				nativeIDs,
				groupState.idsByOperation[operation]...,
			)
		}

		sort.Strings(nativeIDs)

		existingACLs = append(existingACLs, acl.ExistingAcl{
			Acl: acl.Acl{
				Permission:      string(*inferredPermission),
				Topic:           groupKey.topic,
				Username:        groupKey.username,
				ResourcePattern: toResourcePatternString(groupKey.patternType),
			},
			IDs: nativeIDs,
		})
	}

	return existingACLs, nil
}

func (c *AclClient) Create(ctx context.Context, project, service string, req acl.CreateKafkaACLRequest) ([]*acl.Acl, error) {
	host := "*"

	perm := kafka.PermissionType(req.Permission)
	ops, ok := permissionToOps[perm]
	if !ok {
		return nil, fmt.Errorf("unknown permission type: %s", req.Permission)
	}

	patternType, err := parseResourcePattern(req.ResourcePattern)
	if err != nil {
		return nil, err
	}

	opList := make([]kafka.OperationType, 0, len(ops))
	for op := range ops {
		opList = append(opList, op)
	}
	sort.Slice(opList, func(i, j int) bool { return string(opList[i]) < string(opList[j]) })

	for _, op := range opList {
		in := &kafka.ServiceKafkaNativeAclAddIn{
			Host:           &host,
			Operation:      op,
			PatternType:    patternType,
			PermissionType: kafka.ServiceKafkaNativeAclPermissionTypeAllow,
			Principal:      "User:" + req.Username,
			ResourceName:   req.Topic,
			ResourceType:   kafka.ResourceTypeTopic,
		}

		log.WithFields(log.Fields{
			"username": req.Username,
			"topic":    req.Topic,
			"op":       op,
			"pattern":  patternType,
		}).Info("Creating Kafka native ACL")

		_, err := c.ServiceKafkaNativeAclAdd(ctx, project, service, in)
		if err != nil {
			if isAclAlreadyExistsError(err) {
				log.Debug("ACL already exists (string match fallback), skipping creation")
				continue
			}
			return nil, err
		}
	}

	return []*acl.Acl{}, nil
}

func (c *AclClient) Delete(ctx context.Context, project, service, aclID string) error {
	log.Info("Deleting Kafka NativeAcl with ID ", aclID, " from service ", service, " in project ", project)
	return c.ServiceKafkaNativeAclDelete(ctx, project, service, aclID)
}

func isAclAlreadyExistsError(err error) bool {
	return err != nil &&
		strings.Contains(err.Error(), "409") &&
		strings.Contains(err.Error(), "Identical ACL entry already exists")
}

// ---------- permission inference + mapping ----------

// Single source of truth: Create expands using this; List infers by checking containment.
var permissionToOps = map[kafka.PermissionType]map[kafka.OperationType]struct{}{
	kafka.PermissionTypeAdmin: {
		kafka.OperationTypeRead:            {},
		kafka.OperationTypeDescribe:        {},
		kafka.OperationTypeWrite:           {},
		kafka.OperationTypeAlterConfigs:    {},
		kafka.OperationTypeDescribeConfigs: {},
		kafka.OperationTypeDelete:          {},
	},
	kafka.PermissionTypeReadwrite: {
		kafka.OperationTypeRead:     {},
		kafka.OperationTypeDescribe: {},
		kafka.OperationTypeWrite:    {},
	},
	kafka.PermissionTypeWrite: {
		kafka.OperationTypeWrite:    {},
		kafka.OperationTypeDescribe: {},
	},
	kafka.PermissionTypeRead: {
		kafka.OperationTypeRead:     {},
		kafka.OperationTypeDescribe: {},
	},
}

func inferPermissionFromOps(have map[kafka.OperationType]struct{}) *kafka.PermissionType {
	order := []kafka.PermissionType{
		kafka.PermissionTypeAdmin,
		kafka.PermissionTypeReadwrite,
		kafka.PermissionTypeWrite,
		kafka.PermissionTypeRead,
	}
	for _, p := range order {
		if containsAll(have, permissionToOps[p]) {
			pp := p
			return &pp
		}
	}
	return nil
}

func containsAll(have map[kafka.OperationType]struct{}, want map[kafka.OperationType]struct{}) bool {
	for op := range want {
		if _, ok := have[op]; !ok {
			return false
		}
	}
	return true
}

func parseResourcePattern(p string) (kafka.PatternType, error) {
	switch strings.ToUpper(p) {
	case "", "LITERAL":
		return kafka.PatternTypeLiteral, nil
	case "PREFIXED":
		return kafka.PatternTypePrefixed, nil
	default:
		return "", fmt.Errorf("unknown resource pattern: %s", p)
	}
}

func toResourcePatternString(pt kafka.PatternType) string {
	switch pt {
	case kafka.PatternTypeLiteral:
		return "LITERAL"
	case kafka.PatternTypePrefixed:
		return "PREFIXED"
	default:
		return string(pt)
	}
}
