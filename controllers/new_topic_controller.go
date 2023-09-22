package controllers

import (
	"errors"
	"fmt"
	"github.com/aiven/aiven-go-client"
	kafkarator_aiven "github.com/nais/kafkarator/pkg/aiven"
	kafka_nais_io_v1 "github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
	"github.com/nais/liberator/pkg/controller"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"time"
)

type ConditionType string

const (
	AivenFailure      ConditionType = "AivenFailure"
	KafkaratorFailure ConditionType = "KafkaratorFailure"
)

type NewTopicReconciler struct {
	Aiven    kafkarator_aiven.Interfaces
	Projects []string
}

func (r *NewTopicReconciler) New() *kafka_nais_io_v1.Topic {
	return &kafka_nais_io_v1.Topic{}
}

func (r *NewTopicReconciler) Name() string {
	return "kafkarator"
}

func (r *NewTopicReconciler) LogDetail(topic *kafka_nais_io_v1.Topic) log.Fields {
	return log.Fields{
		"team":          topic.Labels["team"],
		"aiven_project": topic.Spec.Pool,
		"aiven_topic":   topic.FullName(),
	}
}

func (r *NewTopicReconciler) Process(topic *kafka_nais_io_v1.Topic, logger log.FieldLogger, eventRecorder record.EventRecorder) controller.ReconcileResult {
	var err error
	status := topic.Status
	status.FullyQualifiedName = topic.FullName()

	conditions := r.makeConditions()

	fail := func(err error, reason string, retry bool) controller.ReconcileResult {
		var conditionType ConditionType
		var aivenError aiven.Error
		ok := errors.As(err, &aivenError)
		var message string
		if !ok {
			conditionType = KafkaratorFailure
			message = err.Error()
		} else {
			conditionType = AivenFailure
			message = fmt.Sprintf("%s: %s", aivenError.Message, aivenError.MoreInfo)
			if !retry {
				status.LatestAivenSyncFailure = time.Now().Format(time.RFC3339)
			}
		}

		condition := conditions[conditionType]
		condition.Status = metav1.ConditionTrue
		condition.Reason = reason
		condition.Message = message
		logger.Errorf("%s: %s - %s", conditionType, reason, message)
		eventRecorder.Event(topic, "Warning", reason, message)

		return controller.ReconcileResult{
			Requeue:    retry,
			Conditions: r.getConditions(conditions),
			State:      controller.SynchronizationStateFailed,
		}
	}

	projectName := topic.Spec.Pool
	if !r.projectWhitelisted(projectName) {
		return fail(fmt.Errorf("pool '%s' cannot be used in this cluster", projectName), "InvalidPool", false)
	}

	synchronizer, err := NewSynchronizer(r.Aiven, *topic, logger)
	if err != nil {
		return fail(fmt.Errorf("failed creating synchronizer: %w", err), "InternalError", true)
	}

	err = synchronizer.Synchronize()
	if err != nil {
		return fail(fmt.Errorf("failed synchronizing: %w", err), "SynchronizationError", true)
	}

	status.LatestAivenSyncFailure = ""
	return controller.ReconcileResult{
		Requeue:    false,
		Conditions: r.getConditions(conditions),
		State:      controller.SynchronizationStateSuccessful,
	}
}

func (r *NewTopicReconciler) Delete(topic *kafka_nais_io_v1.Topic, logger log.FieldLogger, eventRecorder record.EventRecorder) bool {
	//TODO implement me
	panic("implement me")
}

func (r *NewTopicReconciler) NeedsProcessing(topic *kafka_nais_io_v1.Topic, logger log.FieldLogger, eventRecorder record.EventRecorder) bool {
	return topic.NeedsSynchronization(topic.Status.SynchronizationHash) // TODO: Hash is already handled earlier in the process
}

func (r *NewTopicReconciler) projectWhitelisted(project string) bool {
	for _, p := range r.Projects {
		if p == project {
			return true
		}
	}
	return false
}

func (r *NewTopicReconciler) getConditions(conditions map[ConditionType]*metav1.Condition) []metav1.Condition {
	var result []metav1.Condition
	for _, condition := range conditions {
		result = append(result, *condition)
	}
	return result
}

func (r *NewTopicReconciler) makeConditions() map[ConditionType]*metav1.Condition {
	return map[ConditionType]*metav1.Condition{
		AivenFailure: {
			Type: string(AivenFailure),
		},
		KafkaratorFailure: {
			Type: string(KafkaratorFailure),
		},
	}
}
