VERSION ?= $(shell git describe --tags --always 2>/dev/null)
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
GIT_TAG ?= $(shell git describe --tags --always 2>/dev/null || echo dev)

LDFLAGS := -s -w -X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.GitTag=$(GIT_TAG)

.PHONY: build clean

build:
	cd cmd/gjules && go build -ldflags "$(LDFLAGS)" -o ../../gjules .

test: build
	./gjules version

clean:
	rm -f gjules cmd/gjules/gjules
