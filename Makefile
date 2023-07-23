PROC_COUNT+="$(shell nproc)"
CMAKE_INSTALL_PREFIX=$(shell realpath .)
# The -DCMAKE_OSX_ARCHITECTURES flag should be ignored on non-OSX platforms
CMAKE_OSX_ARCHITECTURES=$(shell uname -m)
GIT_VERSION?=$(shell git describe --always --long --abbrev=8 --dirty)
GO_LDFLAG_VERSION := -X 'main.Version=$(GIT_VERSION)'
MIST_COMMIT ?= "catalyst"
DOCKER_TAG ?= "livepeer/catalyst"
BUILD_TARGET ?= "full"

$(shell mkdir -p ./bin)
$(shell mkdir -p ./build)
$(shell mkdir -p $(HOME)/.config/livepeer)
buildpath=$(realpath ./build)

.PHONY: all
all: download livepeer-log

.PHONY: ffmpeg
ffmpeg:
	mkdir -p build
	cd ../go-livepeer && ./install_ffmpeg.sh $(buildpath)

.PHONY: build
build:
	go build -ldflags="$(GO_LDFLAG_VERSION)" -o build/downloader cmd/downloader/downloader/downloader.go
	go build -ldflags="$(GO_LDFLAG_VERSION)" -o build/manifest cmd/downloader/manifest/manifest.go

.PHONY: mistserver
mistserver:
	set -x \
	&& mkdir -p ./build/mistserver \
	&& cd ./build/mistserver \
	&& meson ../../../mistserver -DLOAD_BALANCE=true -Dprefix=${CMAKE_INSTALL_PREFIX} -Dbuildtype=debugoptimized --default-library static \
	&& ninja \
	&& ninja install

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

.PHONY: livepeer-catalyst-api
livepeer-catalyst-api:
	set -x \
	&& cd ../catalyst-api \
	&& make build \
	&& cd - \
	&& mv ../catalyst-api/build/catalyst-api ./bin/livepeer-catalyst-api \
	&& mv ../catalyst-api/build/mist-cleanup.sh ./bin/mist-cleanup

.PHONY: livepeer-catalyst-uploader
livepeer-catalyst-uploader:
	set -x \
	&& cd ../catalyst-uploader \
	&& make \
	&& cd - \
	&& mv ../catalyst-uploader/build/catalyst-uploader ./bin/livepeer-catalyst-uploader

.PHONY: livepeer-analyzer
livepeer-analyzer:
	set -x \
	&& cd ../livepeer-data \
	&& make analyzer \
	&& cd - \
	&& mv ../livepeer-data/build/analyzer ./bin/livepeer-analyzer

.PHONY: livepeer-api
livepeer-api:
	set -x \
	&& cd ../studio \
	&& yarn run pkg:local \
	&& cd - \
	&& mv ../studio/packages/api/bin/api ./bin/livepeer-api

.PHONY: download
download:
	go run cmd/downloader/downloader/downloader.go -v=5 $(ARGS)

.PHONY: manifest
manifest:
	go run cmd/downloader/manifest/manifest.go -v=9 $(ARGS)

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
	&& export PATH=$$PATH:$$(pwd)/bin \
	&& stat $(HOME)/.config/livepeer/catalyst.json || cp ./config/catalyst-dev.json $(HOME)/.config/livepeer/catalyst.json \
	&& ./bin/MistController -c $(HOME)/.config/livepeer/catalyst.json

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
	mv $(HOME)/.config/livepeer/catalyst.json $(HOME)/.config/livepeer/catalyst-$$(date +%s)-dev.json || echo '' \
	&& echo "done"

.PHONY: docker
docker:
	docker build -t "$(DOCKER_TAG)" --build-arg=GIT_VERSION=$(GIT_VERSION) --build-arg=BUILD_TARGET=$(BUILD_TARGET) .

.PHONY: docker-local
docker-local: scripts
	tar ch ./bin Dockerfile.local ./scripts ./config | docker build -f Dockerfile.local -t "$(DOCKER_TAG)" --build-arg=GIT_VERSION=$(GIT_VERSION) --build-arg=BUILD_TARGET=$(BUILD_TARGET) -

.PHONY: box-local
box-local: DOCKER_TAG=livepeer/in-a-box
box-local:
	tar ch ./bin Dockerfile.local ./scripts ./config | docker build -f Dockerfile.local -t "$(DOCKER_TAG)" --build-arg=GIT_VERSION=$(GIT_VERSION) --build-arg=BUILD_TARGET=$(BUILD_TARGET) -

.PHONY: test
test: docker
	go test ./test/e2e/*.go -v --logtostderr

.PHONY: test-local
test-local: docker-local
	go test ./test/e2e/*.go -v --logtostderr

.PHONY: scripts
scripts:
	cp -Rv ./scripts/* ./bin

.PHONY: box-dev
box-dev: scripts
	exec docker run \
	-v $$HOME/code/livepeer-in-a-box-database-snapshots:/data \
	-v $$(realpath bin):/usr/local/bin \
	-v $$(realpath ../studio):/studio \
	-v $$(realpath config):/config:ro \
	--rm \
	-it \
	--name catalyst \
	--shm-size=4gb \
	-p 1935:1935 \
	-p 8888:8888 \
	-p 4242:4242 \
	-e ATHIEST=true livepeer/catalyst:latest MistController \
	-c /config/full-stack.json