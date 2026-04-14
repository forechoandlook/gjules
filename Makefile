VERSION ?= 0.1.0
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
GIT_TAG ?= $(shell git describe --tags --always 2>/dev/null || echo dev)

LDFLAGS := -s -w -X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.GitTag=$(GIT_TAG)

.PHONY: build clean

build:
	cd cmd/gjlues && go build -ldflags "$(LDFLAGS)" -o ../../gjlues .

test: build
	./gjlues version

clean:
	rm -f gjlues cmd/gjlues/gjlues
