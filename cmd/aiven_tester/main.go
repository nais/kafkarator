package main

import (
	"fmt"
	"github.com/aiven/aiven-go-client"
	"github.com/nais/kafkarator/controllers"
	kafkarator_aiven "github.com/nais/kafkarator/pkg/aiven"
	"github.com/nais/kafkarator/pkg/aiven/serviceuser"
	"github.com/nais/kafkarator/pkg/certificate"
	kafka_nais_io_v1 "github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"path"
	"strings"
	"text/template"
	"time"

	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	// +kubebuilder:scaffold:imports
)

const (
	ExitOK = iota
	ExitConfig
	ExitClient
	ExitSync
	ExitSecretDir
	ExitCredStores
	ExitFileCreation
)

// Configuration options
const (
	AivenToken  = "aiven-token"
	LogFormat   = "log-format"
	Pool        = "pool"
	Team        = "team"
	Application = "application"
	TopicName   = "topic-name"
)

const (
	LogFormatJSON = "json"
	LogFormatText = "text"
)

const Script = `
#!/bin/bash

export KAFKA_BROKERS={{ .SyncResult.Brokers }}
export KAFKA_SCHEMA_REGISTRY={{ .SyncResult.Registry }}
export KAFKA_SCHEMA_REGISTRY_USER={{ .User.AivenUser.Username }}
export KAFKA_SCHEMA_REGISTRY_PASSWORD={{ .User.AivenUser.Password }}
export KAFKA_CERTIFICATE='{{ .User.AivenUser.AccessCert }}'
export KAFKA_PRIVATE_KEY='{{ .User.AivenUser.AccessKey }}'
export KAFKA_CA='{{ .SyncResult.CA }}'
export KAFKA_CREDSTORE_PASSWORD={{ .CredStorePass }}

export KAFKA_CERTIFICATE_PATH={{ index .Paths "service.crt" }}
export KAFKA_PRIVATE_KEY_PATH={{ index .Paths "service.key" }}
export KAFKA_CA_PATH={{ index .Paths "ca.crt" }}
export KAFKA_KEYSTORE_PATH={{ index .Paths "client.keystore.p12" }}
export KAFKA_TRUSTSTORE_PATH={{ index .Paths "client.truststore.jks" }}

echo Completed configuring your environment for Aiven testing
`

type TestConfig struct {
	Pool        string
	Service     string
	Team        string
	Application string
	TopicName   string
}

func init() {

	// Automatically read configuration options from environment variables.
	// i.e. --aiven-token will be configurable using KAFKARATOR_AIVEN_TOKEN.
	viper.SetEnvPrefix("AIVEN_TESTER")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))

	flag.String(AivenToken, "", "Administrator credentials for Aiven")
	flag.String(LogFormat, "text", "Log format, either 'text' or 'json'")
	flag.String(Team, "test-team", "Team that owns the test topic")
	flag.String(Application, "test-app", "Name of application that owns the test topic")
	flag.String(TopicName, "test-topic", "Name of test topic")
	flag.String(Pool, "nav-integration-test", "Pool to operate on")

	flag.Parse()

	err := viper.BindPFlags(flag.CommandLine)
	if err != nil {
		panic(err)
	}
}

func formatter(logFormat string) (log.Formatter, error) {
	switch logFormat {
	case LogFormatJSON:
		return &log.JSONFormatter{
			TimestampFormat:   time.RFC3339Nano,
			DisableHTMLEscape: true,
		}, nil
	case LogFormatText:
		return &log.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: time.RFC3339Nano,
		}, nil
	}
	return nil, fmt.Errorf("unsupported log format '%s'", logFormat)
}

func main() {
	logger := log.New()
	logfmt, err := formatter(viper.GetString(LogFormat))
	if err != nil {
		logger.Error(err)
		os.Exit(ExitConfig)
	}

	logger.SetFormatter(logfmt)

	aivenClient, err := aiven.NewTokenClient(viper.GetString(AivenToken), "")
	if err != nil {
		logger.Error(fmt.Errorf("unable to set up aiven client: %w", err))
		os.Exit(ExitClient)
	}

	tc, t := createTestObjects()

	interfaces := kafkarator_aiven.Interfaces{
		ACLs:         aivenClient.KafkaACLs,
		CA:           aivenClient.CA,
		ServiceUsers: aivenClient.ServiceUsers,
		Service:      aivenClient.Services,
		Topics:       aivenClient.KafkaTopics,
	}
	synchronizer := controllers.NewSynchronizer(interfaces, t, logger.WithField("topic", tc.TopicName))
	result, err := synchronizer.Synchronize(t)
	if err != nil {
		logger.Error(fmt.Errorf("failed to synchronize topic: %w", err))
		os.Exit(ExitSync)
	}

	// - Create Truststore and Keystore for JVM clients
	// - Save it all to directory, along with bash-script to set env-vars
	generator := certificate.NewExecGenerator(logger)
	// There is only one user, so just handle that
	user := result.Users[0]
	secretDir, err := ioutil.TempDir("", fmt.Sprintf("kafka-%s-%s-%s-", tc.Pool, tc.Team, tc.Application))
	if err != nil {
		logger.Error(fmt.Errorf("failed to create directory for secret: %w", err))
		os.Exit(ExitSecretDir)
	}
	credStore, err := generator.MakeCredStores(user.AivenUser.AccessKey, user.AivenUser.AccessCert, result.CA)
	if err != nil {
		logger.Error(fmt.Errorf("unable to generate truststore/keystore: %w", err))
		os.Exit(ExitCredStores)
	}

	paths, err := writeSecretFiles(user, result, credStore, secretDir)
	if err != nil {
		logger.Error(fmt.Errorf("unable to write file: %w", err))
		os.Exit(ExitFileCreation)
	}
	logger.Infof("Created files in %s", secretDir)

	type TemplateValue struct {
		Paths         map[string]string
		User          *serviceuser.UserMap
		SyncResult    *controllers.SyncResult
		CredStorePass string
	}

	value := TemplateValue{
		Paths:         paths,
		User:          user,
		SyncResult:    result,
		CredStorePass: credStore.Secret,
	}

	tmpl := template.Must(template.New("script").Parse(Script))
	filePath := path.Join(secretDir, "env.sh")
	f, err := os.Create(filePath)
	if err != nil {
		log.Error(fmt.Errorf("create script file: %w", err))
		os.Exit(ExitFileCreation)
	}
	err = tmpl.Execute(f, value)
	if err != nil {
		log.Error(fmt.Errorf("render template: %w", err))
		os.Exit(ExitFileCreation)
	}

	logger.Infof("Created env script at %s, source it to set up environment for Aiven testing", filePath)
	os.Exit(ExitOK)
}

func createTestObjects() (TestConfig, kafka_nais_io_v1.Topic) {
	tc := TestConfig{
		Pool:        viper.GetString(Pool),
		Service:     kafkarator_aiven.ServiceName(viper.GetString(Pool)),
		Team:        viper.GetString(Team),
		Application: viper.GetString(Application),
		TopicName:   viper.GetString(TopicName),
	}
	t := kafka_nais_io_v1.Topic{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tc.TopicName,
			Namespace: tc.Team,
		},
		Spec: kafka_nais_io_v1.TopicSpec{
			Pool: tc.Pool,
			ACL: []kafka_nais_io_v1.TopicACL{{
				Access:      "readwrite",
				Application: tc.Application,
				Team:        tc.Team,
			}},
		},
	}
	return tc, t
}

func writeSecretFiles(user *serviceuser.UserMap, result *controllers.SyncResult, credStore *certificate.CredStoreData, secretDir string) (map[string]string, error) {
	files := map[string][]byte{
		"service.crt":           []byte(user.AivenUser.AccessCert),
		"service.key":           []byte(user.AivenUser.AccessKey),
		"ca.crt":                []byte(result.CA),
		"client.keystore.p12":   credStore.Keystore,
		"client.truststore.jks": credStore.Truststore,
	}
	paths := map[string]string{}
	for fileName, data := range files {
		filePath := path.Join(secretDir, fileName)
		paths[fileName] = filePath
		err := ioutil.WriteFile(filePath, data, 0644)
		if err != nil {
			return nil, err
		}
	}
	return paths, nil
}
