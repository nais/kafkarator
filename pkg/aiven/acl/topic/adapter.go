package topic

import (
	kafka_nais_io_v1 "github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
)

var _ Source = TopicAdapter{}

type TopicAdapter struct {
	*kafka_nais_io_v1.Topic
}

func (t TopicAdapter) TopicName() string {
	return t.FullName()
}

func (t TopicAdapter) Pool() string {
	return t.Spec.Pool
}

func (t TopicAdapter) ACLs() kafka_nais_io_v1.TopicACLs {
	return t.Spec.ACL
}

var _ Source = StreamAdapter{}

type StreamAdapter struct {
	*kafka_nais_io_v1.Stream
	Delete bool
}

func (s StreamAdapter) TopicName() string {
	return s.TopicWildcard()
}

func (s StreamAdapter) Pool() string {
	return s.Spec.Pool
}

func (s StreamAdapter) ACLs() kafka_nais_io_v1.TopicACLs {
	if s.Delete {
		return kafka_nais_io_v1.TopicACLs{}
	} else {
		return kafka_nais_io_v1.TopicACLs{s.ACL()}
	}
}
