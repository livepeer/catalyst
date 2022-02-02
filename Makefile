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
	&& make -j${PROC_COUNT} \
	&& make -j${PROC_COUNT} MistProcLivepeer \
	&& make install

.PHONY: go-livepeer
go-livepeer:
	set -x \
	&& cd ../go-livepeer \
	&& PKG_CONFIG_PATH=$(buildpath)/compiled/lib/pkgconfig make livepeer livepeer_cli \
	&& cd - \
	&& cp ../go-livepeer/livepeer ./bin/livepeer \
	&& cp ../go-livepeer/livepeer_cli ./bin/livepeer-cli

.PHONY: livepeer-www
livepeer-www:
	set -x \
	&& cd ../livepeer-com/packages/www \
	&& yarn run pkg:local \
	&& cd - \
	&& mv ../livepeer-com/packages/www/bin/www ./bin/livepeer-www

.PHONY: livepeer-api
livepeer-api:
	set -x \
	&& cd ../livepeer-com/packages/api \
	&& yarn run pkg:local \
	&& cd - \
	&& mv ../livepeer-com/packages/api/bin/api ./bin/livepeer-api

.PHONY: download
download:
	go run main.go -v=5

.PHONY: mac-dev
mac-dev:
	set -x \
	&& rm -rf /Volumes/RAMDisk/mist \
	&& TMP=/Volumes/RAMDisk ./bin/MistController -c mist.conf -g 4

.PHONY: livepeer-log
livepeer-log:
	go build -o ./bin/livepeer-log ./cmd/livepeer-log/livepeer-log.go
