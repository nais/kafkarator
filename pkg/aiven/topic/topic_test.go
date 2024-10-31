package topic_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/aiven/aiven-go-client/v2"
	"github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/nais/kafkarator/pkg/aiven/topic"
	"github.com/nais/kafkarator/pkg/utils"
)

func intp(i int) *int {
	return &i
}

func int64p(i int64) *int64 {
	return &i
}

func stringp(s string) *string {
	return &s
}

type topicTest struct {
	name     string
	topic    kafka_nais_io_v1.Topic         // input
	project  string                         // expected kafka project
	service  string                         // expected kafka service
	existing *aiven.KafkaTopic              // expected return from getting existing topic
	create   *aiven.CreateKafkaTopicRequest // expected create request, if any
	update   *aiven.UpdateKafkaTopicRequest // expected request, if any
	error    map[string]bool                // should any sub-function return an error? create - update - get
}

var tests = []topicTest{
	{
		name: "create a new topic",
		topic: kafka_nais_io_v1.Topic{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytopic",
				Namespace: "myteam",
			},
			Spec: kafka_nais_io_v1.TopicSpec{
				Pool: "mypool",
				Config: &kafka_nais_io_v1.Config{
					Partitions:  intp(2),
					Replication: intp(3),
				},
			},
		},
		project: "someproject",
		service: "mypool-kafka",
		create: &aiven.CreateKafkaTopicRequest{
			Partitions:  intp(2),
			Replication: intp(3),
			TopicName:   "myteam.mytopic",
			Tags: []aiven.KafkaTopicTag{
				{Key: "created-by", Value: "Kafkarator"},
			},
		},
	},

	{
		name: "update an existing topic",
		topic: kafka_nais_io_v1.Topic{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytopic",
				Namespace: "myteam",
			},
			Spec: kafka_nais_io_v1.TopicSpec{
				Pool: "mypool",
				Config: &kafka_nais_io_v1.Config{
					Replication: intp(3),
				},
			},
		},
		project: "someproject",
		service: "mypool-kafka",
		existing: &aiven.KafkaTopic{
			Replication: 2,
		},
		update: &aiven.UpdateKafkaTopicRequest{
			Replication: intp(3),
			Tags: []aiven.KafkaTopicTag{
				{Key: "created-by", Value: "Kafkarator"},
			},
		},
	},

	{
		name: "skip updating a topic without changes",
		topic: kafka_nais_io_v1.Topic{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytopic",
				Namespace: "myteam",
			},
			Spec: kafka_nais_io_v1.TopicSpec{
				Pool: "mypool",
				Config: &kafka_nais_io_v1.Config{
					CleanupPolicy:         stringp("compact"),
					MaxMessageBytes:       intp(2048),
					MinimumInSyncReplicas: intp(3),
					Partitions:            intp(2),
					Replication:           intp(3),
					RetentionBytes:        intp(1024),
					RetentionHours:        intp(36),
					SegmentHours:          intp(24),
				},
			},
		},
		project: "someproject",
		service: "mypool-kafka",
		existing: &aiven.KafkaTopic{
			Partitions:  []*aiven.Partition{{}, {}},
			Replication: 3,
			Config: aiven.KafkaTopicConfigResponse{
				CleanupPolicy: &aiven.KafkaTopicConfigResponseString{
					Value: "compact",
				},
				MaxMessageBytes: &aiven.KafkaTopicConfigResponseInt{
					Value: 2048,
				},
				MinInsyncReplicas: &aiven.KafkaTopicConfigResponseInt{
					Value: 3,
				},
				RetentionBytes: &aiven.KafkaTopicConfigResponseInt{
					Value: 1024,
				},
				RetentionMs: &aiven.KafkaTopicConfigResponseInt{
					Value: (time.Duration(36) * time.Hour).Milliseconds(),
				},
				SegmentMs: &aiven.KafkaTopicConfigResponseInt{
					Value: (time.Duration(24) * time.Hour).Milliseconds(),
				},
			},
			Tags: []aiven.KafkaTopicTag{
				{Key: "created-by", Value: "Kafkarator"},
			},
		},
	},

	{
		name: "skip updating a topic with nil config",
		topic: kafka_nais_io_v1.Topic{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytopic",
				Namespace: "myteam",
			},
			Spec: kafka_nais_io_v1.TopicSpec{
				Pool: "mypool",
			},
		},
		project: "someproject",
		service: "mypool-kafka",
		existing: &aiven.KafkaTopic{
			Partitions:  []*aiven.Partition{{}, {}},
			Replication: 3,
			Config: aiven.KafkaTopicConfigResponse{
				CleanupPolicy: &aiven.KafkaTopicConfigResponseString{
					Value: "compact",
				},
				MaxMessageBytes: &aiven.KafkaTopicConfigResponseInt{
					Value: 2048,
				},
				MinInsyncReplicas: &aiven.KafkaTopicConfigResponseInt{
					Value: 3,
				},
				RetentionBytes: &aiven.KafkaTopicConfigResponseInt{
					Value: 1024,
				},
				RetentionMs: &aiven.KafkaTopicConfigResponseInt{
					Value: (time.Duration(36) * time.Hour).Milliseconds(),
				},
				SegmentMs: &aiven.KafkaTopicConfigResponseInt{
					Value: (time.Duration(24) * time.Hour).Milliseconds(),
				},
			},
			Tags: []aiven.KafkaTopicTag{
				{Key: "created-by", Value: "Kafkarator"},
			},
		},
	},

	{
		name: "Ignore unset fields in config",
		topic: kafka_nais_io_v1.Topic{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytopic",
				Namespace: "myteam",
			},
			Spec: kafka_nais_io_v1.TopicSpec{
				Pool: "mypool",
				Config: &kafka_nais_io_v1.Config{
					CleanupPolicy:         stringp("compact"),
					MaxMessageBytes:       nil,
					MinimumInSyncReplicas: intp(3),
					Partitions:            intp(2),
					Replication:           intp(3),
					RetentionBytes:        intp(1024),
					RetentionHours:        nil,
					SegmentHours:          nil,
				},
			},
		},
		project: "someproject",
		service: "mypool-kafka",
		existing: &aiven.KafkaTopic{
			Partitions:  []*aiven.Partition{{}, {}},
			Replication: 3,
			Config: aiven.KafkaTopicConfigResponse{
				CleanupPolicy: &aiven.KafkaTopicConfigResponseString{
					Value: "compact",
				},
				MinInsyncReplicas: &aiven.KafkaTopicConfigResponseInt{
					Value: 3,
				},
				RetentionBytes: &aiven.KafkaTopicConfigResponseInt{
					Value: 1024,
				},
				RetentionMs: &aiven.KafkaTopicConfigResponseInt{
					Value: (time.Duration(36) * time.Hour).Milliseconds(),
				},
			},
			Tags: []aiven.KafkaTopicTag{
				{Key: "created-by", Value: "Kafkarator"},
			},
		},
	},

	{
		name: "Negative retention is always -1",
		topic: kafka_nais_io_v1.Topic{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytopic",
				Namespace: "myteam",
			},
			Spec: kafka_nais_io_v1.TopicSpec{
				Pool: "mypool",
				Config: &kafka_nais_io_v1.Config{
					Replication:    intp(3),
					RetentionHours: intp(-1),
				},
			},
		},
		project: "someproject",
		service: "mypool-kafka",
		existing: &aiven.KafkaTopic{
			Replication: 2,
			Config: aiven.KafkaTopicConfigResponse{
				RetentionMs: &aiven.KafkaTopicConfigResponseInt{
					Source: "topic_config",
					Value:  -1,
				},
			},
		},
		update: &aiven.UpdateKafkaTopicRequest{
			Replication: intp(3),
			Config: aiven.KafkaTopicConfig{
				RetentionMs: int64p(-1),
			},
			Tags: []aiven.KafkaTopicTag{
				{Key: "created-by", Value: "Kafkarator"},
			},
		},
	},

	{
		name: "SegmentHours below minimum value is always 1 hour",
		topic: kafka_nais_io_v1.Topic{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytopic",
				Namespace: "myteam",
			},
			Spec: kafka_nais_io_v1.TopicSpec{
				Pool: "mypool",
				Config: &kafka_nais_io_v1.Config{
					SegmentHours: intp(0),
				},
			},
		},
		project: "someproject",
		service: "mypool-kafka",
		existing: &aiven.KafkaTopic{
			Config: aiven.KafkaTopicConfigResponse{
				SegmentMs: &aiven.KafkaTopicConfigResponseInt{
					Source: "topic_config",
					Value:  (time.Duration(168) * time.Hour).Milliseconds(),
				},
			},
		},
		update: &aiven.UpdateKafkaTopicRequest{
			Config: aiven.KafkaTopicConfig{
				SegmentMs: int64p((time.Duration(1) * time.Hour).Milliseconds()),
			},
			Tags: []aiven.KafkaTopicTag{
				{Key: "created-by", Value: "Kafkarator"},
			},
		},
	},

	{
		name: "unexpected error when getting existing topic",
		topic: kafka_nais_io_v1.Topic{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytopic",
				Namespace: "myteam",
			},
			Spec: kafka_nais_io_v1.TopicSpec{
				Pool: "mypool",
			},
		},
		project: "someproject",
		service: "mypool-kafka",
		create: &aiven.CreateKafkaTopicRequest{
			TopicName: "myteam.mytopic",
			Tags: []aiven.KafkaTopicTag{
				{Key: "created-by", Value: "Kafkarator"},
			},
		},
		error: map[string]bool{
			"get": true,
		},
	},

	{
		name: "unexpected error when creating topic",
		topic: kafka_nais_io_v1.Topic{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytopic",
				Namespace: "myteam",
			},
			Spec: kafka_nais_io_v1.TopicSpec{
				Pool: "mypool",
			},
		},
		project: "someproject",
		service: "mypool-kafka",
		create: &aiven.CreateKafkaTopicRequest{
			TopicName: "myteam.mytopic",
			Tags: []aiven.KafkaTopicTag{
				{Key: "created-by", Value: "Kafkarator"},
			},
		},
		error: map[string]bool{
			"create": true,
		},
	},

	{
		name: "unexpected error when updating topic",
		topic: kafka_nais_io_v1.Topic{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mytopic",
				Namespace: "myteam",
			},
			Spec: kafka_nais_io_v1.TopicSpec{
				Pool: "mypool",
				Config: &kafka_nais_io_v1.Config{
					RetentionBytes: intp(3),
				},
			},
		},
		project: "someproject",
		service: "mypool-kafka",
		existing: &aiven.KafkaTopic{
			Config: aiven.KafkaTopicConfigResponse{
				RetentionBytes: &aiven.KafkaTopicConfigResponseInt{
					Value: 2,
				},
			},
		},
		update: &aiven.UpdateKafkaTopicRequest{
			Config: aiven.KafkaTopicConfig{
				RetentionBytes: int64p(3),
			},
			Tags: []aiven.KafkaTopicTag{
				{Key: "created-by", Value: "Kafkarator"},
			},
		},
		error: map[string]bool{
			"update": true,
		},
	},
}

func TestManager_Synchronize(t *testing.T) {
	ctx := context.Background()
	for _, test := range tests {
		t.Run(test.name, func(tt *testing.T) {
			subTest(ctx, tt, test)
		})
	}
}

func subTest(ctx context.Context, t *testing.T, test topicTest) {
	if test.error == nil {
		test.error = map[string]bool{}
	}

	m := &topic.MockInterface{}
	m.Test(t)

	if test.error["get"] {
		m.On("Get", ctx, test.project, test.service, test.topic.FullName()).Return(nil, aiven.Error{
			Status: http.StatusInternalServerError,
		})
	} else if test.existing == nil {
		m.On("Get", ctx, test.project, test.service, test.topic.FullName()).Return(nil, aiven.Error{
			Status: http.StatusNotFound,
		})
	} else {
		m.On("Get", ctx, test.project, test.service, test.topic.FullName()).Return(test.existing, nil)
	}

	if test.create != nil && !test.error["get"] {
		if test.error["create"] {
			m.On("Create", ctx, test.project, test.service, mock.MatchedBy(utils.TopicCreateReqComp(*test.create))).Return(aiven.Error{
				Message: "failed create",
				Status:  http.StatusInternalServerError,
			})
		} else {
			m.On("Create", ctx, test.project, test.service, mock.MatchedBy(utils.TopicCreateReqComp(*test.create))).Return(nil)
		}
	}

	if test.update != nil && !test.error["get"] {
		if test.error["update"] {
			m.On("Update", ctx, test.project, test.service, test.topic.FullName(), mock.MatchedBy(utils.TopicUpdateReqComp(*test.update))).Return(aiven.Error{
				Message: "failed create",
				Status:  http.StatusInternalServerError,
			})
		} else {
			m.On("Update", ctx, test.project, test.service, test.topic.FullName(), mock.MatchedBy(utils.TopicUpdateReqComp(*test.update))).Return(nil)
		}
	}

	manager := topic.Manager{
		AivenTopics: m,
		Topic:       test.topic,
		Project:     test.project,
		Service:     test.service,
		Logger:      log.NewEntry(log.StandardLogger()),
	}

	err := manager.Synchronize(ctx)

	if len(test.error) > 0 {
		assert.Error(t, err)
	} else {
		assert.NoError(t, err)
	}

	m.AssertExpectations(t)
}
