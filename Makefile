VERSION ?= dev

.PHONY: build fmt-check vet test ci
build:
	CGO_ENABLED=0 go build -trimpath -ldflags '-s -w -X main.version=$(VERSION)' -o bin/websearch ./cmd/websearch

fmt-check:
	@test -z "$$(gofmt -l cmd internal)" || (gofmt -l cmd internal; exit 1)

vet:
	go vet ./...

test:
	go test -race ./...

ci: fmt-check vet test build
