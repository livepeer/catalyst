PROC_COUNT+="$(shell nproc)"
CMAKE_INSTALL_PREFIX=$(shell realpath .)

.PHONY: all
all: build

.PHONY: build
analyzer:
	PKG_CONFIG_PATH=./lib/pkgconfig go build -o ./bin/ ./cmd/MistOutLivepeerAnalyzer/MistOutLivepeerAnalyzer.go

.PHONY: ffmpeg
ffmpeg:
	mkdir -p build
	cd ../go-livepeer && ./install_ffmpeg.sh $(realpath ../livepeer-in-a-box/build)

.PHONY: mist
mist:
	set -x \
	&& mkdir -p ./build/mist \
	&& cd ./build/mist \
	&& cmake ../../../DMS -DPERPETUAL=1 -DCMAKE_INSTALL_PREFIX=${CMAKE_INSTALL_PREFIX}  \
	&& make -j${PROC_COUNT}  \
	&& make install
 