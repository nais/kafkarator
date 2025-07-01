package main

import (
	"fmt"
	"github.com/kelseyhightower/envconfig"
	log "github.com/sirupsen/logrus"
	"reflect"
)

type FeatureFlags struct {
	GeneratedClient bool `split_words:"true"`
}

func GetFeatureFlags() (*FeatureFlags, error) {
	flags := &FeatureFlags{}
	err := envconfig.Process("FEATURE", flags)
	if err != nil {
		return nil, fmt.Errorf("unable to parse feature flags config: %w", err)
	}

	return flags, nil
}

func (f *FeatureFlags) Log(logger log.FieldLogger) {
	val := reflect.ValueOf(*f)
	typeOfStruct := val.Type()

	for i := 0; i < val.NumField(); i++ {
		logger.Infof("%s: %v", typeOfStruct.Field(i).Name, val.Field(i).Interface())
	}
}
