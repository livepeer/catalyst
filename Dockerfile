FROM	golang:1-bullseye	as	gobuild

WORKDIR	/build

ADD	go.mod go.sum	./
RUN	go mod download

ADD	Makefile manifest.yaml ./
ADD	cmd/downloader/ cmd/downloader/
RUN	make download

ADD	. .
ARG	GIT_VERSION=unknown
ENV	GIT_VERSION="${GIT_VERSION}"
RUN	make livepeer-log livepeer-catalyst-node

FROM	livepeer/mist-api-connector:v0.12.5-18-g30e54c1 as mapic
FROM	ubuntu:20.04

LABEL	maintainer="Amritanshu Varshney <amritanshu+github@livepeer.org>"

RUN	apt update && apt install -y \
	ca-certificates \
	musl \
	&& rm -rf /var/lib/apt/lists/*

COPY --from=gobuild	/build/bin/	/usr/bin/
COPY --from=mapic /root/mist-api-connector /usr/bin/livepeer-mist-api-connector

EXPOSE	1935	4242	8080	8889/udp

CMD	["/usr/bin/MistController", "-c", "/etc/livepeer/catalyst.json"]
