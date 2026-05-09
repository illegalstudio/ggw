.PHONY: build install test fmt vet clean

BINARY := ggw
PKG    := ./cmd/ggw

build:
	go build -o $(BINARY) $(PKG)

install:
	go install $(PKG)

test:
	go test ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

clean:
	rm -f $(BINARY)
