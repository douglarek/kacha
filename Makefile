BIN := kacha
LDFLAGS := -s -w
# The identity this machine signs with, or - for an ad hoc signature.
SIGN := $(shell sh scripts/sign.sh 2>/dev/null || echo -)

.PHONY: all build test vet fmt linux macos package icon clean

all: build

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) .

test:
	go test ./...

vet:
	go vet ./...
	GOOS=linux go vet ./...

fmt:
	gofmt -l -w .

linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN)-linux-amd64 .
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN)-linux-arm64 .

macos:
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN)-darwin-arm64 .

icon:
	go run ./tools/icongen

package:
	go tool mygo build -sign "$(SIGN)" -platform darwin/universal,linux/amd64,linux/arm64

clean:
	rm -f $(BIN) $(BIN)-linux-amd64 $(BIN)-linux-arm64 $(BIN)-darwin-arm64
	rm -rf dist
