package utils_test

import (
	"testing"

	"github.com/nais/kafkarator/pkg/utils"
	"github.com/stretchr/testify/assert"
)

type secretTest struct {
	basename string
	maxlen   int
	output   string
}

var tests = []secretTest{
	{"kafka-app-pool", 63, "kafka-app-pool-74b5fc16"},
	{"kafka-extremely-long-application-name-very-long-pool-name", 63, "kafka-extremely-long-application-name-very-long-pool-n-9370c1a7"},
	{"foobarbaz", 10, "f-1a7827aa"},
}

func TestSecretName(t *testing.T) {
	for _, test := range tests {
		output, err := utils.ShortName(test.basename, test.maxlen)
		assert.NoError(t, err)
		assert.Equal(t, test.output, output)
		assert.True(t, len(test.output) <= test.maxlen)
	}
}
