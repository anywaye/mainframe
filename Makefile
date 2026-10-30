# Build targets for mainframe CLI

BINARY := mf
VERSION := $(shell cat VERSION)
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

.PHONY: build test clean install

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/mf

test:
	go test ./...

clean:
	rm -f $(BINARY)

install: build
	cp $(BINARY) /usr/local/bin/$(BINARY)

# Cross-compile
build-linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-linux-amd64 ./cmd/mf
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY)-linux-arm64 ./cmd/mf

build-darwin:
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY)-darwin-arm64 ./cmd/mf
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-darwin-amd64 ./cmd/mf

release: clean
	$(MAKE) build-linux build-darwin
	@echo "Release binaries built. Upload to GitHub releases."