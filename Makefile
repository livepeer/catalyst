SHELL := /bin/bash
PROC_COUNT+=$(shell nproc)
CMAKE_INSTALL_PREFIX=$(shell realpath .)
# The -DCMAKE_OSX_ARCHITECTURES flag should be ignored on non-OSX platforms
CMAKE_OSX_ARCHITECTURES=$(shell uname -m)
GIT_VERSION?=$(shell git describe --always --long --abbrev=8 --dirty)
GO_LDFLAG_VERSION := -X 'main.Version=$(GIT_VERSION)'
MIST_COMMIT ?= catalyst
DOCKER_TAG ?= livepeer/catalyst
FROM_LOCAL_PARENT ?= scratch # for `make docker-local` and `make box-local`
DOCKER_TARGET ?= catalyst
BUILD_TARGET ?= full
STUDIO_DIR=$(if $(wildcard ../studio/packages/www/static-build/.),"../studio/packages/api","../studio")
export KILL ?= true
export GOOS ?= linux

ifeq ($(shell uname), Darwin)
	ifeq ($(GOOS), linux)
		CROSS_ARGS = "--cross-file=$(realpath ./config/mistserver-linux-arm64-cross.txt)"
	endif
endif

$(shell mkdir -p ./bin)
$(shell mkdir -p ./build)
$(shell mkdir -p ./data)
$(shell mkdir -p ./coredumps)
$(shell mkdir -p $(HOME)/.config/livepeer)
buildpath=$(realpath ./build)

.PHONY: all
all: download catalyst

.PHONY: ffmpeg
ffmpeg:
	mkdir -p build
	cd ../go-livepeer && ./install_ffmpeg.sh $(buildpath)

.PHONY: mistserver
mistserver:
	set -x \
	&& mkdir -p ./build/mistserver \
	&& cd ./build/mistserver \
	&& meson ../../../mistserver $(CROSS_ARGS) -DLOAD_BALANCE=true -Dprefix=${CMAKE_INSTALL_PREFIX} -Dbuildtype=debugoptimized --default-library static \
	&& ninja \
	&& ninja install

.PHONY: go-livepeer
go-livepeer:
	set -x \
	&& cd ../go-livepeer \
	&& PKG_CONFIG_PATH=$(buildpath)/compiled/lib/pkgconfig make livepeer livepeer_cli \
	&& cd - \
	&& mv ../go-livepeer/livepeer ./bin/livepeer \
	&& mv ../go-livepeer/livepeer_cli ./bin/livepeer-cli \
	&& $(MAKE) box-kill BIN=livepeer

.PHONY: livepeer-task-runner
livepeer-task-runner:
	set -x \
	&& cd ../task-runner \
	&& PKG_CONFIG_PATH=$(buildpath)/compiled/lib/pkgconfig make \
	&& cd - \
	&& mv ../task-runner/build/task-runner ./bin/livepeer-task-runner \
	&& $(MAKE) box-kill BIN=livepeer-task-runner

.PHONY: livepeer-catalyst-api
livepeer-catalyst-api:
	set -x \
	&& export GOOS=linux \
	&& cd ../catalyst-api \
	&& make build \
	&& cd - \
	&& mv ../catalyst-api/build/catalyst-api ./bin/livepeer-catalyst-api \
	&& mv ../catalyst-api/build/mist-cleanup.sh ./bin/mist-cleanup \
	&& $(MAKE) box-kill BIN=livepeer-catalyst-api

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
	&& mv ../livepeer-data/build/analyzer ./bin/livepeer-analyzer \
	&& $(MAKE) box-kill BIN=livepeer-analyzer

.PHONY: livepeer-api
livepeer-api:
	cd $(STUDIO_DIR) \
	&& yarn run esbuild \
	&& cd - \
	&& mv ../studio/packages/api/dist-esbuild/livepeer-api ./bin/livepeer-api \
	&& $(MAKE) box-kill BIN=livepeer-api

# same as livepeer-api but uses pkg instead of esbuild
# pros: bundles the frontend too
# cons: takes four minutes to build
.PHONY: livepeer-api-pkg
livepeer-api-pkg:
	cd ../studio \
	&& yarn run pkg:local \
	&& cd - \
	&& mv ../studio/packages/api/bin/api ./bin/livepeer-api \
	&& $(MAKE) box-kill BIN=livepeer-api

.PHONY: box-kill
box-kill:
	[[ "$$KILL" == "true" ]] && docker exec catalyst pkill -f /usr/local/bin/$(BIN) || echo "Not restarting $(BIN), use KILL=true if you want that"

.PHONY: catalyst
catalyst:
	go build -o ./bin/catalyst ./cmd/catalyst/catalyst.go

.PHONY: download
download:
	GOOS="" GOARCH="" go build -o ./build/downloader cmd/downloader/downloader.go \
	&& ./build/downloader

.PHONY: manifest
manifest:
	GOOS="" GOARCH="" go run cmd/downloader/downloader.go -update-manifest=true -download=false $(ARGS)

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
	docker buildx build \
		--load \
		-t "$(DOCKER_TAG)" \
		-t "$(DOCKER_TAG):parent" \
		--target=$(DOCKER_TARGET) \
		--build-arg=GIT_VERSION=$(GIT_VERSION) \
		--build-arg=BUILD_TARGET=$(BUILD_TARGET) \
		--build-arg=FROM_LOCAL_PARENT=$(FROM_LOCAL_PARENT) \
		$(ARGS) \
		.

.PHONY: docker-local
docker-local: scripts
	tar ch ./bin ./config \
	| docker buildx build \
		--load \
		-t "$(DOCKER_TAG)" \
		--build-arg=GIT_VERSION=$(GIT_VERSION) \
		--build-arg=BUILD_TARGET=$(BUILD_TARGET) \
		--build-arg=FROM_LOCAL_PARENT=$(FROM_LOCAL_PARENT) \
		$(ARGS) \
		-

.PHONY: box
box: scripts
	$(MAKE) docker DOCKER_TAG=livepeer/in-a-box DOCKER_TARGET=livepeer-in-a-box

.PHONY: box-local
box-local: scripts
	$(MAKE) docker-local DOCKER_TAG=livepeer/in-a-box DOCKER_TARGET=livepeer-in-a-box FROM_LOCAL_PARENT=livepeer/in-a-box:parent

.PHONY: test
test: docker box
	GOOS="" GOARCH="" go test ./test/e2e/*.go -v --logtostderr

.PHONY: test-local
test-local: docker-local box-local
	GOOS="" GOARCH="" go test ./test/e2e/*.go -v --logtostderr

.PHONY: scripts
scripts:
	cp -Rv ./scripts/* ./bin

.PHONY: box-dev
box-dev: scripts
	ulimit -c unlimited \
	&& exec docker run \
	-v $$(realpath bin):/usr/local/bin \
	-v $$(realpath data):/data \
	-v $$(realpath config):/etc/livepeer:ro \
	-v $$(realpath ./coredumps):$$(realpath ./coredumps) \
	-e CORE_DUMP_DIR=$$(realpath ./coredumps) \
	$(shell for line in $$(cat .env 2>/dev/null || echo ''); do printf -- "-e $$line "; done) \
	--rm \
	-it \
	--name catalyst \
	--shm-size=4gb \
	-p 8888:8888 \
	-p 8989:8989 \
	-p 5432:5432 \
	-p 1935:1935 \
	-p 4242:4242 \
	-p 7979:7979 \
	-p 3478:3478 \
	-p 3478:3478/udp \
	-p 5349:5349 \
	-p 9001:9001 \
	-p 9090:9090 \
	-p 40000-40100:40000-40100/udp \
	livepeer/in-a-box

build/sysroot-aarch64-gnu-linux: sysroot.Dockerfile
	rm -rf ./build/tmp-sysroot-aarch64-gnu-linux \
	&& mkdir -p ./build/tmp-sysroot-aarch64-gnu-linux \
	&& docker build -t sysroot-aarch64-gnu-linux -f sysroot.Dockerfile . \
	&& docker run --rm -v $$(realpath build):/build --platform linux/arm64 sysroot-aarch64-gnu-linux tar cf - /lib /usr/include /usr/lib /usr/local/lib /usr/local/include \
	| tar xf - -C $$(realpath build)/tmp-sysroot-aarch64-gnu-linux \
	&& rm -rf ./build/sysroot-aarch64-gnu-linux \
	&& mv ./build/tmp-sysroot-aarch64-gnu-linux ./build/sysroot-aarch64-gnu-linux \
	&& ln -s $$(realpath ./build/sysroot-aarch64-gnu-linux) /tmp/sysroot-aarch64-gnu-linux

.PHONY: snapshot
snapshot:
	rm -rf livepeer-studio-bootstrap.tar.gz \
	&& cd data \
	&& rm -rf cockroach/auxiliary/EMERGENCY_BALLAST \
	&& tar czvf ../livepeer-studio-bootstrap.tar.gz cockroach
