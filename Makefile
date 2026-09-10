.PHONY: build run-agent run-aggregator test clean docker docker-push testdata

BINARY=logcatcher
MODULE=github.com/ordinary/k8s-log-catcher
VERSION ?= latest

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/$(BINARY) ./cmd/logcatcher

run-agent:
	LOG_CATCHER_MODE=agent \
	LOG_CATCHER_LOG_PATH=./testdata/containers \
	LOG_CATCHER_STORAGE_PATH=./testdata/db \
	go run ./cmd/logcatcher --mode=agent

run-aggregator:
	LOG_CATCHER_MODE=aggregator \
	LOG_CATCHER_DISCOVERY_METHOD=static \
	LOG_CATCHER_DISCOVERY_ENDPOINTS=http://localhost:19489 \
	go run ./cmd/logcatcher --mode=aggregator

test:
	go test ./... -v -race

clean:
	rm -rf bin/ testdata/db/

docker:
	docker build -t kapture:latest .

docker-push:
	@./scripts/build-and-push.sh $(VERSION)

# Generate test log data
testdata:
	mkdir -p testdata/containers
	@echo "Generating test log files..."
	@./scripts/generate-testdata.sh
