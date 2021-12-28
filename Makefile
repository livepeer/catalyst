.PHONY: all
all: build

.PHONY: build
build:
	PKG_CONFIG_PATH=${HOME}/compiled/lib/pkgconfig go build -o ./build/ ./cmd/livepeer/livepeer.go
