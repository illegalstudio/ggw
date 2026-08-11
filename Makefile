.PHONY: build install test fmt vet clean deploy

BINARY := ggw
PKG    := ./cmd/ggw

VERSION := $(shell git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || echo dev)
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
LDFLAGS := -s -w \
           -X github.com/illegalstudio/ggw/internal/version.Version=$(VERSION) \
           -X github.com/illegalstudio/ggw/internal/version.Commit=$(COMMIT)

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(PKG)

install:
	go install -ldflags "$(LDFLAGS)" $(PKG)

test:
	go test ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

clean:
	rm -f $(BINARY)

# Interactive release: propose next semver tag, then tag + push to origin.
deploy:
	@scripts/deploy.sh
