# Build targets for mainframe CLI

BINARY := mf
VERSION := $(shell cat VERSION)
LDFLAGS := -ldflags "-X github.com/anywaye/mainframe/internal/cmd.version=$(VERSION)"

.PHONY: build test clean install


# Sync lib/ and assets/ to internal/assets/ before building
sync-assets:
	cp -r lib/* internal/assets/lib/
	cp -r assets/* internal/assets/assets/ 2>/dev/null || true
	cp -r audit/* internal/assets/audit/ 2>/dev/null || true
	cp -r modes/* internal/assets/modes/ 2>/dev/null || true
	cp -r projects/* internal/assets/projects/ 2>/dev/null || true
	@echo "Assets synced."

# Ensure assets are synced before build
build: sync-assets

build:
	go build -a $(LDFLAGS) -o $(BINARY) ./cmd/mf

test:
	go test ./...

clean:
	rm -f $(BINARY)

install: build
	cp $(BINARY) /usr/local/bin/$(BINARY)

# Cross-compile
build-linux:
	GOOS=linux GOARCH=amd64 go build -a $(LDFLAGS) -o $(BINARY)-linux-amd64 ./cmd/mf
	GOOS=linux GOARCH=arm64 go build -a $(LDFLAGS) -o $(BINARY)-linux-arm64 ./cmd/mf

build-darwin:
	GOOS=darwin GOARCH=arm64 go build -a $(LDFLAGS) -o $(BINARY)-darwin-arm64 ./cmd/mf
	GOOS=darwin GOARCH=amd64 go build -a $(LDFLAGS) -o $(BINARY)-darwin-amd64 ./cmd/mf

release: clean
	$(MAKE) build-linux build-darwin
	@echo "Release binaries built. Upload to GitHub releases."