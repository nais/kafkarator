module github.com/nais/kafkarator

go 1.15

require (
	github.com/Shopify/sarama v1.27.2
	github.com/aiven/aiven-go-client v1.5.7
	github.com/ghodss/yaml v1.0.0
	github.com/gogo/protobuf v1.3.1 // indirect
	github.com/nais/liberator v0.0.0-20210421120235-5f3edcf81f86
	github.com/prometheus/client_golang v1.0.0
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.3.2
	github.com/stretchr/testify v1.6.1
	k8s.io/api v0.17.16
	k8s.io/apimachinery v0.17.16
	k8s.io/client-go v0.17.16
	sigs.k8s.io/controller-runtime v0.5.0
	sigs.k8s.io/controller-tools v0.2.5 // indirect
)
