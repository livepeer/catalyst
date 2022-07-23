FROM	golang:1-bullseye	as	gobuild

ARG	GIT_VERSION=unknown

WORKDIR	/build

ADD	go.mod go.sum	./

RUN	go mod download

ADD	.	.

RUN	make

FROM	ubuntu:20.04

LABEL	maintainer="Amritanshu Varshney <amritanshu+github@livepeer.org>"

RUN	apt update && apt install -y \
	ca-certificates \
	musl \
	&& rm -rf /var/lib/apt/lists/*

COPY --from=gobuild	/build/bin/	/usr/bin/

EXPOSE	1935	4242	8080	8889/udp

CMD	["/usr/bin/MistController", "-c", "/etc/livepeer/catalyst.json"]
