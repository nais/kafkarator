package kafka

import (
	"fmt"
	"time"
)

type Message struct {
	Offset    int64
	TimeStamp time.Time
	Partition int32
}

func (c *Message) String() string {
	return fmt.Sprintf("offset=%d, partition=%d, timestamp=%s", c.Offset, c.Partition, c.TimeStamp.Format(time.RFC3339Nano))
}
