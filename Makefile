GOCACHE ?= /tmp/sing-box-smart-go-cache
export GOCACHE
.PHONY: run test build check
run:
	go run ./cmd/sing-box-smart
build:
	go build -o bin/sing-box-smart ./cmd/sing-box-smart
test:
	go test -race ./...
check:
	go vet ./...
