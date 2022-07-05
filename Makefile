cmd ?= catalyst-node
version ?= $(shell git describe --tag --dirty)
ldflags := -X 'main.Version=$(version)'
builddir := build

.PHONY: all build

all: build

build:
	go build -o $(builddir)/catalyst-node -ldflags="$(ldflags)" cmd/catalyst-node/catalyst-node.go

run:
	$(builddir)/catalyst-node
