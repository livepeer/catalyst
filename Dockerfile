FROM livepeerci/api:master as api
FROM livepeerci/www:master as www
FROM livepeer/streamtester:mist-api-connector as mist-api-connector

# We want to inherit from a CUDA container for driver-support... so let's just use go-livepeer,
# which already has a CUDA environment.
FROM livepeer/go-livepeer:master

RUN apt update

RUN apt install -y \
  python3-pip \
  curl
RUN curl -fsSL https://deb.nodesource.com/setup_16.x | bash -
RUN apt install -y nodejs

RUN pip3 install supervisor

ENV DEBIAN_FRONTEND noninteractive
RUN apt install -y postgresql-all
RUN echo "listen_addresses='*'" >> /var/lib/postgresql/10/main/postgresql.conf
RUN echo "host all  all    0.0.0.0/0  trust" >> /var/lib/postgresql/10/main/pg_hba.conf

RUN curl --silent -L -o - https://github.com/traefik/traefik/releases/download/v2.4.8/traefik_v2.4.8_linux_amd64.tar.gz | tar -C /usr/bin/ -xvz

ARG MIST_URL
RUN curl -o - --silent $MIST_URL | tar -C /usr/bin/ -xvz
COPY mistserver.conf /etc/mistserver.conf

COPY --from=api /app /api

COPY supervisord.conf /usr/local/supervisord.conf
COPY traefik.toml /traefik.toml
COPY traefik-routes.toml /traefik-routes.toml

RUN npm install -g serve
COPY --from=www /www /www

ENTRYPOINT []
CMD supervisord -c /usr/local/supervisord.conf
