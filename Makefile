BINARY_NAME=bd-claim
LOCAL_BIN=bin/$(BINARY_NAME)

.PHONY: all build build-all clean run sync-beads test

all: build

build:
	@mkdir -p bin
	go build -mod=mod -o $(LOCAL_BIN) ./cmd

build-all:
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 go build -mod=mod -o bin/$(BINARY_NAME)-linux-amd64 ./cmd
	GOOS=linux GOARCH=arm64 go build -mod=mod -o bin/$(BINARY_NAME)-linux-arm64 ./cmd
	GOOS=darwin GOARCH=amd64 go build -mod=mod -o bin/$(BINARY_NAME)-darwin-amd64 ./cmd
	GOOS=darwin GOARCH=arm64 go build -mod=mod -o bin/$(BINARY_NAME)-darwin-arm64 ./cmd
	GOOS=windows GOARCH=amd64 go build -mod=mod -o bin/$(BINARY_NAME)-windows-amd64.exe ./cmd
	GOOS=windows GOARCH=arm64 go build -mod=mod -o bin/$(BINARY_NAME)-windows-arm64.exe ./cmd

clean:
	go clean
	rm -rf bin/
	rm -f coverage.out coverage.html

run: build
	./$(LOCAL_BIN)

sync-beads:
	git submodule update --remote vendor/beads
	./scripts/gather_beads_context.sh

test:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Checking coverage..."
	@go tool cover -func=coverage.out | grep total | awk '{print "Total coverage: " $$3} {if ($$3 != "100.0%") {print "Error: Coverage is not 100.0%"; exit 1}}'

