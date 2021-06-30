FROM livepeerci/api:master as api
FROM livepeerci/www:master as www
FROM livepeer/streamtester:mist-api-connector as mist-api-connector

FROM golang as unpack
WORKDIR /app
ADD go.mod go.mod
ADD go.sum go.sum
RUN go mod download
ADD unpack-box.go unpack-box.go
RUN go build unpack-box.go

# We want to inherit from a CUDA container for driver-support... so let's just use go-livepeer,
# which already has a CUDA environment.
FROM livepeer/go-livepeer:master

ENV DEBIAN_FRONTEND noninteractive
RUN apt update && apt install -y \
  python3-pip \
  curl \
  musl \
  postgresql-all \
  rabbitmq-server
RUN rabbitmq-plugins enable rabbitmq_management
ENV RABBITMQ_LOGS "-"
ENV RABBITMQ_DATA_DIR=/data/rabbitmq
ENV LP_AMQP_URL amqp://localhost:5672/
RUN curl -fsSL https://deb.nodesource.com/setup_16.x | bash -
RUN apt install -y nodejs

RUN pip3 install supervisor

RUN curl --silent -L -o - https://github.com/traefik/traefik/releases/download/v2.4.8/traefik_v2.4.8_linux_amd64.tar.gz | tar -C /usr/bin/ -xvz

RUN npm install -g serve
ARG MIST_URL
RUN curl -o - --silent $MIST_URL | tar -C /usr/bin/ -xvz

COPY --from=mist-api-connector /root/mist-api-connector /usr/bin/mist-api-connector

WORKDIR /data

# Below this line, code copying and conf only

RUN echo "listen_addresses='*'" >> /var/lib/postgresql/10/main/postgresql.conf
RUN echo "data_directory = '/data/postgres'" >> /var/lib/postgresql/10/main/postgresql.conf
RUN echo "host all  all    0.0.0.0/0  trust" >> /var/lib/postgresql/10/main/pg_hba.conf
RUN apt install -y sudo rsync

COPY mistserver.conf /etc/mistserver.conf

COPY --from=api /app /api

COPY supervisord.conf /usr/local/supervisord.conf
COPY traefik.toml /traefik.toml
COPY traefik-routes.toml /traefik-routes.toml

COPY --from=www /www /www
COPY --from=unpack /app/unpack-box /usr/bin/unpack-box

ENTRYPOINT []
CMD supervisord -c /usr/local/supervisord.conf
