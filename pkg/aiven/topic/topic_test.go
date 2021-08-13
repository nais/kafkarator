package topic_test

import (
	"net/http"
	"testing"

	"github.com/aiven/aiven-go-client"
	"github.com/nais/kafkarator/pkg/aiven/topic"
	"github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func intp(i int) *int {
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
					MinimumInSyncReplicas: intp(3),
					Partitions:            intp(2),
					Replication:           intp(3),
					RetentionBytes:        intp(1024),
					RetentionHours:        intp(36),
				},
			},
		},
		project: "someproject",
		service: "mypool-kafka",
		existing: &aiven.KafkaTopic{
			CleanupPolicy:         "compact",
			MinimumInSyncReplicas: 3,
			Partitions:            []*aiven.Partition{{}, {}},
			Replication:           3,
			RetentionBytes:        1024,
			RetentionHours:        intp(36),
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
			CleanupPolicy:         "compact",
			MinimumInSyncReplicas: 3,
			Partitions:            []*aiven.Partition{{}, {}},
			Replication:           3,
			RetentionBytes:        1024,
			RetentionHours:        intp(36),
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
					MinimumInSyncReplicas: intp(3),
					Partitions:            intp(2),
					Replication:           intp(3),
					RetentionBytes:        intp(1024),
					RetentionHours:        nil,
				},
			},
		},
		project: "someproject",
		service: "mypool-kafka",
		existing: &aiven.KafkaTopic{
			CleanupPolicy:         "compact",
			MinimumInSyncReplicas: 3,
			Partitions:            []*aiven.Partition{{}, {}},
			Replication:           3,
			RetentionBytes:        1024,
			RetentionHours:        intp(34),
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
			RetentionBytes: 2,
		},
		update: &aiven.UpdateKafkaTopicRequest{
			RetentionBytes: intp(3),
		},
		error: map[string]bool{
			"update": true,
		},
	},
}

func TestManager_Synchronize(t *testing.T) {
	for i, test := range tests {
		t.Logf("Running test %d: %s", i+1, test.name)

		if test.error == nil {
			test.error = map[string]bool{}
		}

		m := &topic.MockInterface{}

		if test.error["get"] {
			m.On("Get", test.project, test.service, test.topic.FullName()).Return(nil, aiven.Error{
				Status: http.StatusInternalServerError,
			})
		} else if test.existing == nil {
			m.On("Get", test.project, test.service, test.topic.FullName()).Return(nil, aiven.Error{
				Status: http.StatusNotFound,
			})
		} else {
			m.On("Get", test.project, test.service, test.topic.FullName()).Return(test.existing, nil)
		}

		if test.create != nil && !test.error["get"] {
			if test.error["create"] {
				m.On("Create", test.project, test.service, *test.create).Return(aiven.Error{
					Message: "failed create",
					Status:  http.StatusInternalServerError,
				})
			} else {
				m.On("Create", test.project, test.service, *test.create).Return(nil)
			}
		}

		if test.update != nil && !test.error["get"] {
			if test.error["update"] {
				m.On("Update", test.project, test.service, test.topic.FullName(), *test.update).Return(aiven.Error{
					Message: "failed create",
					Status:  http.StatusInternalServerError,
				})
			} else {
				m.On("Update", test.project, test.service, test.topic.FullName(), *test.update).Return(nil)
			}
		}

		manager := topic.Manager{
			AivenTopics: m,
			Topic:       test.topic,
			Project:     test.project,
			Service:     test.service,
			Logger:      log.NewEntry(log.StandardLogger()),
		}

		err := manager.Synchronize()

		if len(test.error) > 0 {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}

		m.AssertExpectations(t)
	}
}
