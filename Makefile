.PHONY: build test lint cover clean docker-build

BINARY := ocis-mcp-server
MODULE := github.com/owncloud/ocis-mcp-server
IMAGE  := owncloud/ocis-mcp-server

build:
	go build -trimpath -ldflags="-s -w" -o bin/$(BINARY) ./cmd/ocis-mcp-server

test:
	go test -race ./...

lint:
	golangci-lint run ./...

cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	@echo ""
	@echo "HTML report: go tool cover -html=coverage.out -o coverage.html"

clean:
	rm -rf bin/ coverage.out coverage.html

docker-build:
	docker build -t $(IMAGE):latest .
