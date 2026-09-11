.PHONY: build run-agent run-aggregator test clean docker docker-push testdata

BINARY=kapture
MODULE=github.com/ordinary/k8s-log-catcher
VERSION ?= latest
TIMEZONE ?= Asia/Jakarta
APP_VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null)
LDFLAGS = -s -w -X $(MODULE)/internal/version.Version=$(APP_VERSION) -X $(MODULE)/internal/version.Commit=$(COMMIT)

build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o bin/$(BINARY) ./cmd/kapture

run-agent:
	KAPTURE_MODE=agent \
	KAPTURE_TIMEZONE=$(TIMEZONE) \
	KAPTURE_LOG_PATH=./testdata/containers \
	KAPTURE_STORAGE_PATH=./testdata/db \
	go run ./cmd/kapture --mode=agent

run-aggregator:
	KAPTURE_MODE=aggregator \
	KAPTURE_TIMEZONE=$(TIMEZONE) \
	KAPTURE_DISCOVERY_METHOD=static \
	KAPTURE_DISCOVERY_ENDPOINTS=http://localhost:19489 \
	go run ./cmd/kapture --mode=aggregator

test:
	go test ./... -v -race

clean:
	rm -rf bin/ testdata/db/

docker:
	docker build --build-arg VERSION=$(APP_VERSION) --build-arg COMMIT=$(COMMIT) -t kapture:latest .

docker-push:
	@./scripts/build-and-push.sh $(VERSION)

# Generate test log data
testdata:
	mkdir -p testdata/containers
	@echo "Generating test log files..."
	@./scripts/generate-testdata.sh
