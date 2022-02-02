PROC_COUNT+="$(shell nproc)"
CMAKE_INSTALL_PREFIX=$(shell realpath .)
GO_LDFLAG_VERSION := -X 'main.Version=$(shell git describe --all --dirty)'

buildpath=$(realpath ./build)
$(shell mkdir -p ./bin)
$(shell mkdir -p ./build)

.PHONY: all
all: ffmpeg go-livepeer mistserver

.PHONY: ffmpeg
ffmpeg:
	mkdir -p build
	cd ../go-livepeer && ./install_ffmpeg.sh $(buildpath)

.PHONY: build
build:
	go build -ldflags="$(GO_LDFLAG_VERSION)" -o build/downloader main.go

.PHONY: mistserver
mistserver:
	set -x \
	&& mkdir -p ./build/mistserver \
	&& cd ./build/mistserver \
	&& cmake ../../../DMS -DPERPETUAL=1 -DCMAKE_INSTALL_PREFIX=${CMAKE_INSTALL_PREFIX}  \
	&& make -j${PROC_COUNT}  \
	&& make install

.PHONY: go-livepeer
go-livepeer:
	set -x \
	&& cd ../go-livepeer \
	&& PKG_CONFIG_PATH=$(buildpath)/compiled/lib/pkgconfig make livepeer livepeer_cli \
	&& cd - \
	&& cp ../go-livepeer/livepeer ./bin/livepeer \
	&& cp ../go-livepeer/livepeer_cli ./bin/livepeer-cli

.PHONY: download
download:
	go run main.go -v=5
