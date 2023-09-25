package controllers_test

import (
	"encoding/json"
	"github.com/nais/liberator/pkg/controller"
	"io/ioutil"
	"k8s.io/client-go/tools/record"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/nais/kafkarator/controllers"
	"github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
	log "github.com/sirupsen/logrus"
	"gotest.tools/assert"
)

func yamlSubTestNew(t *testing.T, path string) {
	fixture := fileReader(path)
	data, err := ioutil.ReadAll(fixture)
	if err != nil {
		t.Errorf("unable to read test data: %s", err)
		t.Fail()
		return
	}

	test := testCase{}
	err = yaml.Unmarshal(data, &test)
	if err != nil {
		t.Errorf("unable to unmarshal test data: %s", err)
		t.Fail()
		return
	}

	topic := &kafka_nais_io_v1.Topic{}
	err = json.Unmarshal(test.Topic, topic)
	if err != nil {
		t.Errorf("unable to parse topic: %s", err)
		t.Fail()
		return
	}
	if topic.Status == nil {
		topic.Status = &kafka_nais_io_v1.TopicStatus{}
	}
	err = topic.ApplyDefaults()
	if err != nil {
		t.Errorf("unable to apply defaults: %s", err)
		t.Fail()
		return
	}

	aivenMocks, assertMocks := aivenMockInterfaces(t, test)

	var reconciler controller.NaisReconciler[*kafka_nais_io_v1.Topic] = &controllers.NewTopicReconciler{
		Aiven:    aivenMocks,
		Projects: test.Config.Projects,
	}

	eventRecorder := record.NewFakeRecorder(100)

	if strings.Contains(path, "delete") {
		err = reconciler.Delete(topic, log.NewEntry(log.StandardLogger()), eventRecorder)
		assert.NilError(t, err)
	} else {
		result := reconciler.Process(topic, log.NewEntry(log.StandardLogger()), eventRecorder)
		assert.Equal(t, test.Output.Status.SynchronizationState, result.State)
		assert.Equal(t, test.Output.Requeue, result.Requeue)
	}

	assertMocks(t)
}

func TestGoldenFileNew(t *testing.T) {
	// TODO: Add back when deleting old code
	// kafkaratormetrics.Register(prometheus.DefaultRegisterer)

	files, err := ioutil.ReadDir(testDataDirectory)
	if err != nil {
		t.Error(err)
		t.Fail()
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := file.Name()
		if !strings.HasSuffix(name, ".yaml") {
			continue
		}
		path := filepath.Join(testDataDirectory, name)
		t.Run(name, func(t *testing.T) {
			yamlSubTestNew(t, path)
		})
	}
}
