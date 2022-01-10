.PHONY: all
all: build

.PHONY: build
build:
	PKG_CONFIG_PATH=./lib/pkgconfig go build -o ./build/ ./cmd/livepeer/livepeer.go

.PHONY: ffmpeg
ffmpeg:
	mkdir -p build
	cd ../go-livepeer && ./install_ffmpeg.sh $(realpath ../livepeer-in-a-box/build)
