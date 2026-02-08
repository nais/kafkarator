package acl

import "github.com/aiven/go-client-codegen/handler/kafka"

type normalizeKey struct {
	Username string
	Topic    string
}

type wantedAclAggregate struct {
	Username string
	Topic    string

	HasRead      bool
	HasWrite     bool
	HasReadWrite bool

	IDs []string
}

func NormalizeWanted(acls []Acl) []Acl {
	aggregates := make(map[normalizeKey]*wantedAclAggregate)

	for _, a := range acls {
		key := normalizeKey{Username: a.Username, Topic: a.Topic}

		agg, ok := aggregates[key]
		if !ok {
			agg = &wantedAclAggregate{Username: a.Username, Topic: a.Topic}
			aggregates[key] = agg
		}

		switch a.Permission {
		case string(kafka.PermissionTypeRead):
			agg.HasRead = true
		case string(kafka.PermissionTypeWrite):
			agg.HasWrite = true
		case string(kafka.PermissionTypeReadwrite):
			agg.HasReadWrite = true
		}

		agg.IDs = append(agg.IDs, a.IDs...)
	}

	result := make([]Acl, 0, len(aggregates))
	for _, agg := range aggregates {
		perm := ""
		switch {
		case agg.HasReadWrite || (agg.HasRead && agg.HasWrite):
			perm = string(kafka.PermissionTypeReadwrite)
		case agg.HasRead:
			perm = string(kafka.PermissionTypeRead)
		case agg.HasWrite:
			perm = string(kafka.PermissionTypeWrite)
		default:
			continue
		}

		result = append(result, Acl{
			Username:   agg.Username,
			Topic:      agg.Topic,
			Permission: perm,
			IDs:        agg.IDs,
		})
	}

	return result
}
