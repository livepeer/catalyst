# livepeer-in-a-box

## Getting Started

You'll presently need the following repos cloned into the same
directory as livepeer-in-a-box:

 - [go-livepeer](https://github.com/livepeer/go-livepeer) (any branch, only there for `install_ffmpeg.sh`)
 - [mistserver](https://github.com/DDVTECH/mistserver) (`catalyst` branch)

From there:

```
# boots up Postgres and RabbitMQ dependencies
make docker-compose

# downloads or builds services as appropriate
make

# boot up all services under mist controller
make dev
```

You should then have a web interface running at
[http://localhost:3004](http://localhost:3004) and a Mist interface at
[http://localhost:4242](http://localhost:4242).

## Credentials/secrets

Livepeer credentials:

```
username: admin@livepeer.local
password: livepeer
```

Mist credentials:

```
username: test
password: test
```

## Manifest

The `manifest.yaml` file specifies which services are needed for the
box to bootup. The fields (with some description) are shown below:

```yaml
# Manifest versioning, should be a fixed value
version: 2.0

# Default release/tag for projects. Keeping it latest
# will have the script search for most recent tag using
# the github API
release: latest

# Services inside the box
box:
  # array of each service element
  - name: name

    # `project` => github project/repo
    project: livepeer/project-name

    # custom release value (if working on specific tag)
    release: v99.99.99

    # override artifact name generation pattern. default pattern is:
    # <name>-<platform>-<arch>.tar.gz for linux/macos
    # <name>-<platform>-<arch>.zip for windows
    # overrides to using `<binary>-<platform>-<arch>.<ext>`
    binary: livepeer-www

    # path inside the archive zip/tar which is needed for the service
    archivePath: victoria-metrics-prod

    # output name of the binary located at `archivePath`
    outputPath: livepeer-victoria-metrics

    # key-value for mapping platform to custom artifact on github release page
    # this bypasses default name generation pattern entirely
    srcFilenames:
      linux-amd64: victoria-metrics-amd64-v1.74.0.tar.gz
      linux-arm64: victoria-metrics-arm64-v1.74.0.tar.gz
      darwin-amd64: victoria-metrics-darwin-amd64-v1.74.0.tar.gz
      darwin-arm64: victoria-metrics-darwin-arm64-v1.74.0.tar.gz

    # Skip gpg signature verification step
    skipGpg: true

    # Skip sha digest check for artifact
    skipChecksum: true
```
