PROC_COUNT+="$(shell nproc)"
CMAKE_INSTALL_PREFIX=$(shell realpath .)
GO_LDFLAG_VERSION := -X 'main.Version=$(shell git describe --all --dirty)'

$(shell mkdir -p ./bin)
$(shell mkdir -p ./build)
$(shell mkdir -p $(HOME)/.config/livepeer-in-a-box)
buildpath=$(realpath ./build)

.PHONY: all
all: download mistserver livepeer-log ffmpeg livepeer-task-runner

.PHONY: ffmpeg
ffmpeg:
	mkdir -p build
	cd ../go-livepeer && ./install_ffmpeg.sh $(buildpath)

.PHONY: build
build:
	go build -ldflags="$(GO_LDFLAG_VERSION)" -o build/downloader main.go

build/compiled/lib/libmbedtls.a:
	export PKG_CONFIG_PATH=$(buildpath)/compiled/lib/pkgconfig \
	&& export LD_LIBRARY_PATH=$(buildpath)/compiled/lib \
	&& export C_INCLUDE_PATH=$(buildpath)/compiled/include \
	&& git clone -b dtls_srtp_support --depth=1 https://github.com/livepeer/mbedtls.git $(buildpath)/mbedtls \
  && cd $(buildpath)/mbedtls \
  && mkdir build \
  && cd build \
  && cmake -DCMAKE_INSTALL_PREFIX=$(buildpath)/compiled .. \
  && make -j$(nproc) install

build/compiled/lib/libsrtp2.a:
	git clone https://github.com/cisco/libsrtp.git $(buildpath)/libsrtp \
  && cd $(buildpath)/libsrtp \
  && mkdir build \
  && cd build \
  && cmake -DCMAKE_INSTALL_PREFIX=$(buildpath)/compiled .. \
  && make -j$(nproc) install

build/compiled/lib/libsrt.a: build/compiled/lib/libmbedtls.a
build/compiled/lib/libsrt.a:
	git clone https://github.com/Haivision/srt.git $(buildpath)/srt \
  && cd $(buildpath)/srt \
  && mkdir build \
  && cd build \
  && cmake .. -DCMAKE_INSTALL_PREFIX=$(buildpath)/compiled -D USE_ENCLIB=mbedtls -D ENABLE_SHARED=false \
  && make -j$(nproc) install

mistserver: build/compiled/lib/libmbedtls.a build/compiled/lib/libsrtp2.a build/compiled/lib/libsrt.a

.PHONY: mistserver
mistserver:
	set -x \
	export PKG_CONFIG_PATH=$(buildpath)/compiled/lib/pkgconfig \
	export LD_LIBRARY_PATH=~$(buildpath)/compiled/lib \
	export C_INCLUDE_PATH=~$(buildpath)/compiled/include \
	&& mkdir -p ./build/mistserver \
	&& cd ./build/mistserver \
	&& cmake ../../../mistserver -DPERPETUAL=1 -DLOAD_BALANCE=1 -DCMAKE_INSTALL_PREFIX=${CMAKE_INSTALL_PREFIX} -DCMAKE_PREFIX_PATH=$(buildpath)/compiled -DCMAKE_BUILD_TYPE=RelWithDebInfo \
	&& make -j${PROC_COUNT} \
	&& make install

.PHONY: go-livepeer
go-livepeer:
	set -x \
	&& cd ../go-livepeer \
	&& PKG_CONFIG_PATH=$(buildpath)/compiled/lib/pkgconfig make livepeer livepeer_cli \
	&& cd - \
	&& mv ../go-livepeer/livepeer ./bin/livepeer \
	&& mv ../go-livepeer/livepeer_cli ./bin/livepeer-cli

.PHONY: livepeer-task-runner
livepeer-task-runner:
	set -x \
	&& cd ../task-runner \
	&& PKG_CONFIG_PATH=$(buildpath)/compiled/lib/pkgconfig make \
	&& cd - \
	&& mv ../task-runner/build/task-runner ./bin/livepeer-task-runner

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

.PHONY: livepeer-mist-api-connector
livepeer-mist-api-connector:
	set -x \
	&& cd ../stream-tester \
	&& make connector \
	&& cd - \
	&& cp ../stream-tester/build/mist-api-connector ./bin/livepeer-mist-api-connector

.PHONY: livepeer-catalyst-node
livepeer-catalyst-node:
	set -x \
	&& cd ../catalyst-node \
	&& make \
	&& cd - \
	&& cp ../catalyst-node/build/catalyst-node ./bin/livepeer-catalyst-node

.PHONY: download
download:
	go run main.go -v=5 $(ARGS)

.PHONY: dev
dev:
	if [ $$(uname) == "Darwin" ]; then \
		if [ ! -d "/Volumes/RAMDisk/" ]; then \
			disk=$$(hdiutil attach -nomount ram://4194304) \
			&& sleep 3 \
			&& diskutil erasevolume HFS+ "RAMDisk" $$disk \
			&& echo "Created /Volumes/RAMDisk from $$disk"; \
		fi \
		&& rm -rf /Volumes/RAMDisk/mist \
		&& export TMP=/Volumes/RAMDisk; \
	fi \
	&& stat $(HOME)/.config/livepeer-in-a-box/mistserver.dev.conf || cp ./config/mistserver.dev.conf $(HOME)/.config/livepeer-in-a-box/mistserver.dev.conf \
	&& ./bin/MistController -c $(HOME)/.config/livepeer-in-a-box/mistserver.dev.conf

.PHONY: livepeer-log
livepeer-log:
	go build -o ./bin/livepeer-log ./cmd/livepeer-log/livepeer-log.go

.PHONY: catalyst
catalyst:
	go build -o ./bin/catalyst ./cmd/catalyst/catalyst.go

.PHONY: clean
clean:
	git clean -ffdx && mkdir -p bin build

.PHONY: docker-compose
docker-compose:
	mkdir -p .docker/rabbitmq/data .docker/postgres/data \
	&& docker-compose up -d

.PHONY: docker-compose-rm
docker-compose-rm:
	docker-compose stop; docker-compose rm -f

.PHONY: full-reset
full-reset: docker-compose-rm clean all
	mv $(HOME)/.config/livepeer-in-a-box/mistserver.dev.conf $(HOME)/.config/livepeer-in-a-box/mistserver-$$(date +%s).dev.conf || echo '' \
	&& echo "done"
