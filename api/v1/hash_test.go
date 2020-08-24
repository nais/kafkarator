package kafka_nais_io_v1_test

import (
	"testing"

	"github.com/nais/kafkarator/api/v1"
	"github.com/stretchr/testify/assert"
)

func TestHash(t *testing.T) {
	spec := kafka_nais_io_v1.TopicSpec{}
	hash, err := spec.Hash()
	assert.NoError(t, err)
	assert.Equal(t, "dd7b6d7c6d11a91f", hash)
}
