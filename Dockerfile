ARG	GIT_VERSION=unknown
ARG	BUILD_TARGET
ARG	FROM_LOCAL_PARENT

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

RUN	make livepeer-log catalyst

FROM	ubuntu:22.04	as	catalyst-full-build

WORKDIR	/opt/bin

COPY --from=gobuild	/src/bin/	/opt/bin/

FROM	ubuntu:22.04	as	catalyst-stripped-build

ENV	DEBIAN_FRONTEND=noninteractive

WORKDIR	/opt/bin

RUN	apt update && apt install -yqq build-essential

COPY --from=gobuild	/src/bin/	/opt/bin/

RUN	find /opt/bin -type f ! -name "*.sh" ! -name "livepeer-mist-bigquery-uploader" ! -name "livepeer-api" -exec strip -s {} \;

FROM	catalyst-${BUILD_TARGET}-build	as	catalyst-build

# Install livepeer-w3 required to use web3.storage
FROM	node:20.7.0 as node-build
ARG	LIVEPEER_W3_VERSION=v0.2.2
WORKDIR /app
RUN	git clone --depth 1 --branch ${LIVEPEER_W3_VERSION} https://github.com/livepeer/go-tools.git
RUN	npm install --prefix /app/go-tools/w3
# chown needed to make everything owned by one user for userspace podman execution
RUN	chown -R root:root /app/go-tools/w3

FROM	ubuntu:22.04	AS	catalyst

ENV	DEBIAN_FRONTEND=noninteractive

LABEL	maintainer="Amritanshu Varshney <amritanshu+github@livepeer.org>"

ARG	BUILD_TARGET

RUN	apt update && apt install -yqq wget
RUN	wget -O - https://deb.nodesource.com/setup_18.x | bash
RUN	apt update && apt install -yqq \
	curl \
	ca-certificates \
	musl \
	python3 \
	ffmpeg \
    	nodejs \
	gstreamer1.0-tools gstreamer1.0-plugins-good gstreamer1.0-plugins-base gstreamer1.0-plugins-bad \
	"$(if [ "$BUILD_TARGET" != "stripped" ]; then echo "gdb"; fi)" \
	&& rm -rf /var/lib/apt/lists/*

COPY --from=catalyst-build	/opt/bin/		/usr/local/bin/
COPY --from=node-build		/app/go-tools/w3	/opt/local/lib/livepeer-w3
RUN	ln -s /opt/local/lib/livepeer-w3/livepeer-w3.js /usr/local/bin/livepeer-w3 && \
    	npm install -g ipfs-car

EXPOSE	1935	4242	8080	8889/udp

CMD	["/usr/local/bin/MistController", "-c", "/etc/livepeer/catalyst.json"]

FROM catalyst AS livepeer-in-a-box

ARG TARGETARCH

RUN	apt update && apt install -yqq \
	rabbitmq-server \
	nginx \
	gdb \
	inotify-tools \
	file \
	# for `shasum`
	perl \
	coturn \
	&& rm -rf /var/lib/apt/lists/*

RUN curl -L -O https://binaries.cockroachdb.com/cockroach-v23.1.5.linux-$TARGETARCH.tgz \
	&& tar xzvf cockroach-v23.1.5.linux-$TARGETARCH.tgz \
	&& mv cockroach-v23.1.5.linux-$TARGETARCH/cockroach /usr/bin/cockroach \
	&& rm -rf cockroach-v23.1.5.linux-$TARGETARCH.tgz cockroach-v23.1.5.linux-$TARGETARCH \
	&& cockroach --version

RUN curl -o /usr/bin/minio https://dl.min.io/server/minio/release/linux-$TARGETARCH/minio \
	&& curl -o /usr/bin/mc https://dl.min.io/client/mc/release/linux-$TARGETARCH/mc \
	&& chmod +x /usr/bin/minio /usr/bin/mc \
	&& minio --version \
	&& mc --version

ADD ./scripts /usr/local/bin
ADD ./config/full-stack.json /etc/livepeer/full-stack.json

ENV CATALYST_DOWNLOADER_PATH=/usr/local/bin \
	CATALYST_DOWNLOADER_MANIFEST=/etc/livepeer/manifest.yaml \
	CATALYST_DOWNLOADER_UPDATE_MANIFEST=true \
	COCKROACH_DB_SNAPSHOT=https://github.com/iameli-streams/livepeer-in-a-box-database-snapshots/raw/2eb77195f64f22abf3f0de39e6f6930b82a4c098/livepeer-studio-bootstrap.tar.gz

RUN mkdir /data

CMD	["/usr/local/bin/catalyst", "--", "/usr/local/bin/MistController", "-c", "/etc/livepeer/full-stack.json"]

FROM	${FROM_LOCAL_PARENT} AS box-local

LABEL	maintainer="Amritanshu Varshney <amritanshu+github@livepeer.org>"

ADD	./bin	/usr/local/bin
