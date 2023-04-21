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

RUN	make livepeer-log

FROM	ubuntu:20.04	as	catalyst-full-build

WORKDIR	/opt/bin

COPY --from=gobuild	/src/bin/	/opt/bin/

FROM	ubuntu:20.04	as	catalyst-stripped-build

ENV	DEBIAN_FRONTEND=noninteractive

WORKDIR	/opt/bin

RUN	apt update && apt install -yqq build-essential

COPY --from=gobuild	/src/bin/	/opt/bin/

RUN	find /opt/bin -type f ! -name "*.sh" -exec strip -s {} \;

FROM	catalyst-${BUILD_TARGET}-build	as	catalyst-build

# Install livepeer-w3 required to use web3.storage
FROM	node:20.0.0 as node-build
ARG	LIVEPEER_W3_VERSION=v0.2.2
WORKDIR /app
RUN	git clone --depth 1 --branch ${LIVEPEER_W3_VERSION} https://github.com/livepeer/go-tools.git
RUN	npm install --prefix /app/go-tools/w3

FROM	ubuntu:20.04	AS	catalyst

ENV	DEBIAN_FRONTEND=noninteractive

LABEL	maintainer="Amritanshu Varshney <amritanshu+github@livepeer.org>"

ARG	BUILD_TARGET

RUN	apt update && apt install -yqq wget
RUN	wget -O - https://deb.nodesource.com/setup_18.x | bash
RUN	apt update && apt install -yqq \
	ca-certificates \
	musl \
	python3 \
	ffmpeg \
    	nodejs \
	"$(if [ "$BUILD_TARGET" != "stripped" ]; then echo "gdb"; fi)" \
	&& rm -rf /var/lib/apt/lists/*

COPY --from=catalyst-build	/opt/bin/		/usr/local/bin/
COPY --from=node-build		/app/go-tools/w3	/opt/local/lib/livepeer-w3
RUN	ln -s /opt/local/lib/livepeer-w3/livepeer-w3.js /usr/local/bin/livepeer-w3 && \
    	npm install -g ipfs-car

EXPOSE	1935	4242	8080	8889/udp

CMD	["/usr/local/bin/MistController", "-c", "/etc/livepeer/catalyst.json"]
