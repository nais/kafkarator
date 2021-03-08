kafkarator:
	go build -o bin/kafkarator cmd/kafkarator/*.go

canary:
	go build -o bin/canary cmd/canary/*.go

aiven_tester:
	go build -o bin/aiven_tester cmd/aiven_tester/*.go

mocks:
	cd pkg/aiven && mockery -inpkg -all -case snake

test:
	go test ./... -count=1

integration_test:
	echo "*** Make sure to set the environment AIVEN_TOKEN to a valid token ***"
	go test ./pkg/aiven/... ./pkg/certificate/... -tags=integration -v -count=1
