.PHONY: build run-agent run-aggregator test clean docker docker-push testdata

BINARY=kapture
MODULE=github.com/ordinary/k8s-log-catcher
VERSION ?= latest

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/$(BINARY) ./cmd/kapture

run-agent:
	KAPTURE_MODE=agent \
	KAPTURE_LOG_PATH=./testdata/containers \
	KAPTURE_STORAGE_PATH=./testdata/db \
	go run ./cmd/kapture --mode=agent

run-aggregator:
	KAPTURE_MODE=aggregator \
	KAPTURE_DISCOVERY_METHOD=static \
	KAPTURE_DISCOVERY_ENDPOINTS=http://localhost:19489 \
	go run ./cmd/kapture --mode=aggregator

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
