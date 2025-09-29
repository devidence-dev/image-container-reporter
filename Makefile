.PHONY: build test clean run lint deps

BINARY_NAME=icr
BUILD_DIR=bin

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/$(BINARY_NAME)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/$(BINARY_NAME)

test:
	go test -v ./...

test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

clean:
	rm -rf $(BUILD_DIR)

run:
	go run ./cmd/$(BINARY_NAME)

lint:
	go vet ./...
	go fmt ./...
	golangci-lint run
	staticcheck ./...

deps:
	go mod tidy
	go mod download

security:
	govulncheck ./...