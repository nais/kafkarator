
all: kafkarator canary test

kafkarator:
	go build -o bin/kafkarator cmd/kafkarator/*.go

canary:
	go build -o bin/canary cmd/canary/*.go

mocks:
	cd pkg/aiven && go run github.com/vektra/mockery/v2 --inpackage --all --case snake

test:
	go test ./... -v -count=1
