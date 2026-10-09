.PHONY: build check fmt
build:
	go build -o herdr-background-activity ./cmd/herdr-background-activity
fmt:
	gofmt -w cmd internal
check:
	@test -z "$$(gofmt -l cmd internal)" || (echo 'Run make fmt'; exit 1)
	go vet ./...
	go test -race ./...
	go build ./...
