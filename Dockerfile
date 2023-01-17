ARG	GIT_VERSION=unknown
ARG	BUILD_TARGET

FROM	golang:1-bullseye	as	gobuild

WORKDIR	/src

ADD	go.mod go.sum	./
RUN	go mod download

ADD	Makefile manifest.yaml ./
ADD	cmd/downloader/ cmd/downloader/
RUN	make download

ADD	.	.

ARG	GIT_VERSION
ENV	GIT_VERSION="${GIT_VERSION}"

RUN	make livepeer-log livepeer-catalyst-node

FROM	ubuntu:20.04	as	catalyst-full-build

WORKDIR	/opt/bin

COPY --from=gobuild	/src/bin/	/opt/bin/

FROM	ubuntu:20.04	as	catalyst-stripped-build

ENV	DEBIAN_FRONTEND=noninteractive

WORKDIR	/opt/bin

RUN	apt update && apt install -yqq build-essential

COPY --from=gobuild	/src/bin/	/opt/bin/

RUN	strip -s /opt/bin/*

FROM	catalyst-${BUILD_TARGET}-build	as	catalyst-build

FROM	ubuntu:20.04	AS	catalyst

ENV	DEBIAN_FRONTEND=noninteractive

LABEL	maintainer="Amritanshu Varshney <amritanshu+github@livepeer.org>"

ARG	BUILD_TARGET

RUN	apt update && apt install -yqq \
	ca-certificates \
	musl \
	python3 \
	ffmpeg \
	"$(if [ "$BUILD_TARGET" != "stripped" ]; then echo "gdb"; fi)" \
	&& rm -rf /var/lib/apt/lists/*

COPY --from=catalyst-build	/opt/bin/	/usr/local/bin/

EXPOSE	1935	4242	8080	8889/udp

CMD	["/usr/local/bin/MistController", "-c", "/etc/livepeer/catalyst.json"]
