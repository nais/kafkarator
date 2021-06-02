package constants

// Annotations put onto the generated secret
const (
	PoolAnnotation        = "kafka.nais.io/pool"
	ApplicationAnnotation = "kafka.nais.io/application"

	// Temporary while migrating to Aivenator
	ServiceUserAnnotation        = "kafka.aiven.nais.io/serviceUser"
	AivenatorPoolAnnotation      = "kafka.aiven.nais.io/pool"
	AivenatorProtectedAnnotation = "aivenator.aiven.nais.io/protected"
)
