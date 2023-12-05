kafkarator:
	go build -o bin/kafkarator cmd/kafkarator/*.go

canary:
	go build -o bin/canary cmd/canary/*.go

mocks:
	cd pkg/aiven && go run github.com/vektra/mockery/v2 --inpackage --all --case snake

test:
	go test ./... -count=1

integration_test:
	echo "*** Make sure to set the environment AIVEN_TOKEN to a valid token ***"
	go test ./pkg/aiven/... -tags=integration -v -count=1
