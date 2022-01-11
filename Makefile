.PHONY: all
all: build

.PHONY: build
analyzer:
	PKG_CONFIG_PATH=./lib/pkgconfig go build -o ./bin/ ./cmd/MistOutLivepeerAnalyzer/MistOutLivepeerAnalyzer.go

.PHONY: ffmpeg
ffmpeg:
	mkdir -p build
	cd ../go-livepeer && ./install_ffmpeg.sh $(realpath ../livepeer-in-a-box/build)
