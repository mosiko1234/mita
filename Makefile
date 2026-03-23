BINARY_NAME=mita
VERSION=$(shell git describe --tags --always 2>/dev/null || echo "dev")

.PHONY: build-all build-darwin build-windows clean build test vet tidy

build:
	go build -ldflags "-s -w -X main.version=$(VERSION)" -o dist/$(BINARY_NAME) ./cmd/mita/

build-all: build-darwin build-windows

build-darwin:
	GOOS=darwin GOARCH=amd64 go build \
	  -ldflags "-s -w -X main.version=$(VERSION)" \
	  -o dist/$(BINARY_NAME)-darwin-amd64 ./cmd/mita/
	GOOS=darwin GOARCH=arm64 go build \
	  -ldflags "-s -w -X main.version=$(VERSION)" \
	  -o dist/$(BINARY_NAME)-darwin-arm64 ./cmd/mita/

build-windows:
	GOOS=windows GOARCH=amd64 go build \
	  -ldflags "-s -w -X main.version=$(VERSION)" \
	  -o dist/$(BINARY_NAME)-windows-amd64.exe ./cmd/mita/

test:
	go test ./... -v

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -rf dist/
