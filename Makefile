PROC_COUNT+="$(shell nproc)"
CMAKE_INSTALL_PREFIX=$(shell realpath .)
GO_LDFLAG_VERSION := -X 'main.Version=$(shell git describe --all --dirty)'

.PHONY: ffmpeg
ffmpeg:
	mkdir -p build
	cd ../go-livepeer && ./install_ffmpeg.sh $(realpath ../livepeer-in-a-box/build)

.PHONY: build
build:
	go build -ldflags="$(GO_LDFLAG_VERSION)" -o build/downloader main.go

.PHONY: mist
mist:
	set -x \
	&& mkdir -p ./build/mist \
	&& cd ./build/mist \
	&& cmake ../../../DMS -DPERPETUAL=1 -DCMAKE_INSTALL_PREFIX=${CMAKE_INSTALL_PREFIX}  \
	&& make -j${PROC_COUNT}  \
	&& make install
