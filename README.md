# Catalyst

## Getting Started

go1.17 is recommended because building with < 1.17 (specifically 1.16) can result in issue on arm64 architectures.

From there:

```
# boots up Postgres, RabbitMQ and MinIO S3 dependencies
make docker-compose

# downloads or builds services as appropriate
make

# boot up all services under mist controller
make dev
```

After that, you should have a few web interfaces running:
- Livepeer Studio at [http://localhost:3012](http://localhost:3012)
- Mist at [http://localhost:4242](http://localhost:4242).
- MinIO S3 at [http://localhost:9000](http://localhost:9000)

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

MinIO S3 credentials:

````
username: minioadmin
password: minioadmin
````

## Manifest

The `manifest.yaml` file specifies which services are needed for the
box to bootup. The fields (with some description) are shown below:

```yaml
# Manifest versioning, should be a fixed value
version: 3.0

# Default release/tag for projects. Keeping it latest
# will have the script search for most recent tag using
# the github API
release: latest

# Services inside the box
box:
    # array of each service element
    - name: name

      # custom release value (if working on specific tag)
      # Can be the branch name for `bucket` strategy (see below)
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

      # Strategy to use for downloading artifacts: github or bucket
      # `bucket` - Uses `build.livepeer.live` bucket; works for branches of some projects
      # `github` - Uses github releases and tags (default value)
      strategy:
          download: github
          # `project` => github project/repo or bucket key for artifacts
          project: livepeer/project-name
          # `commit` => repository commit SHA (useful for bucket strategy)
          commit: 0000000000000000000000000000000000000000

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
