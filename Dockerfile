FROM livepeerci/api:master as api
FROM livepeerci/www:master as www
FROM livepeer/streamtester:mist-api-connector as mist-api-connector
FROM livepeer/data:latest as analyzer

FROM golang as unpack
WORKDIR /app

# unpack-box script
ADD go.mod go.mod
ADD go.sum go.sum
RUN go mod download
ADD unpack-box.go unpack-box.go
RUN go build unpack-box.go

# We want to inherit from a CUDA container for driver-support... so let's just use go-livepeer,
# which already has a CUDA environment.
FROM livepeer/go-livepeer:master

# dependencies
ENV DEBIAN_FRONTEND noninteractive
RUN apt update && apt install -y \
  python3-pip \
  curl \
  musl \
  postgresql-all \
  sudo \
  rsync

# Node.js
RUN curl -fsSL https://deb.nodesource.com/setup_16.x | bash -
RUN apt install -y nodejs

# Supervisord
RUN pip3 install supervisor

# Traefik
RUN curl --silent -L -o - https://github.com/traefik/traefik/releases/download/v2.4.8/traefik_v2.4.8_linux_amd64.tar.gz | tar -C /usr/bin/ -xvz

# MistServer
ARG MIST_URL
RUN curl -o - --silent $MIST_URL | tar -C /usr/bin/ -xvz

# etcd
ENV ETCD_VER v3.5.0
RUN curl -L https://github.com/etcd-io/etcd/releases/download/${ETCD_VER}/etcd-${ETCD_VER}-linux-amd64.tar.gz -o /tmp/etcd-${ETCD_VER}-linux-amd64.tar.gz \
  && tar xzvf /tmp/etcd-${ETCD_VER}-linux-amd64.tar.gz -C /usr/bin --strip-components=1 \
  && rm -f /tmp/etcd-${ETCD_VER}-linux-amd64.tar.gz \
  && etcd --version

# RabbitMQ
ENV RABBITMQ_DATA_DIR=/data/rabbitmq
ENV RABBITMQ_MNESIA_DIR /data/rabbitmq
ENV RABBITMQ_NODENAME rabbit@localhost
ENV RABBITMQ_LOGS "-"
COPY ./install_rabbitmq.sh ./install_rabbitmq.sh
RUN ./install_rabbitmq.sh

# mist-api-connector
COPY --from=mist-api-connector /root/mist-api-connector /usr/bin/mist-api-connector

# livepeer-analyzer
COPY --from=analyzer /app/analyzer /usr/bin/analyzer

WORKDIR /data

# Below this line, code copying and conf only

RUN echo "listen_addresses='*'" >> /var/lib/postgresql/10/main/postgresql.conf
RUN echo "data_directory = '/data/postgres'" >> /var/lib/postgresql/10/main/postgresql.conf
RUN echo "host all  all    0.0.0.0/0  trust" >> /var/lib/postgresql/10/main/pg_hba.conf

COPY --from=www /app /www
COPY --from=api /app /api

COPY mistserver.conf /etc/mistserver.conf

COPY supervisord.conf /usr/local/supervisord.conf
COPY traefik.toml /traefik.toml
COPY traefik-routes.toml /traefik-routes.toml

COPY --from=unpack /app/unpack-box /usr/bin/unpack-box

ENTRYPOINT []
CMD supervisord -c /usr/local/supervisord.conf
