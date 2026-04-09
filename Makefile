.PHONY: build build-a2a test vet lint check coverage

build:
	GOWORK=off go build -o process-mcp ./cmd/process-mcp

build-a2a:
	GOWORK=off go build -o process-a2a ./cmd/process-a2a

test:
	GOWORK=off go test ./... -count=1

vet:
	GOWORK=off go vet ./...

lint:
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || \
	(command -v staticcheck >/dev/null 2>&1 && staticcheck ./... || echo "no linter installed, skipping")

check: build vet test

coverage:
	GOWORK=off go test ./... -count=1 -coverprofile=coverage.out
	go tool cover -func=coverage.out

HG_PIPELINE_MK ?= $(or $(wildcard $(abspath $(CURDIR)/../dotfiles/make/pipeline.mk)),$(wildcard $(HOME)/hairglasses-studio/dotfiles/make/pipeline.mk))
-include $(HG_PIPELINE_MK)
