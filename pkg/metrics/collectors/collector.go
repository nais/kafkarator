package collectors

import (
	"context"
	"github.com/sirupsen/logrus"
)

type Collector interface {
	Report(ctx context.Context) error
	Description() string
	Logger() logrus.FieldLogger
}
