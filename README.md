# Catalyst

The entire bundle of application needed for booting up a livepeer
system in place is termed as [catalyst].

## Getting Started

Make sure you have the following installed on the machine before
proceeding:

  - go1.17± or newer
  - ffmpeg v5 (or newer)
  - GNU make (not required, but good to have for using `Makefile`
    commands)
  - docker version 19 (or newer) (not mandatory for basic catalyst
    operations)
  - cloning this repository alongside [go-livepeer], [mistserver] and
    other dependent projects of [catalyst] in the same directory
  - about 2-3 GB of free disk space

±: go1.17 is recommended because building with < 1.17 (specifically 1.16)
can result in issues on arm64 architectures.

Note: Compiling mistserver has another set of requirements, not
covered by this readme.

## Setting up catalyst system

### First run (or reset workflow)

Run `make full-reset` which cleans out any older config files, and
local copies of binaries. It also shuts down `docker-compose` system
which aren't needed for base catalyst usage.

### Direct usage (download-and-run)

This section covers using prebuilt binaries for setting up and booting
catalyst system. This is done through using the configuration present
in `manifest.yaml` (section below covers its structure).

Run the following command from the root of this repository directory
(the `-v=9` is just for most verbose output, and can be skipped):

    go run cmd/downloader/downloader/downloader.go -v=9

    # or
    make download ARGS='-v=9'

Other options available for downloader script can be checked with
using `--help`.

The above should download all prebuilt binaries to the `bin/`
directory.

### Indirect usage (build-and-run)

This section covers building the dependent projects before booting up
[catalyst]. There are quite a few applications needed for catalyst as
listed below (but not limited to):

  - [go-livepeer]
  - [mistserver]
  - [catalyst] (for `livepeer-log`, `catalyst-node`)
  - [catalyst-api]
  - [victoria-metrics] (recommended to just download a stable binary
    from github releases)

A few of these can be compiled using `Makefile` recipes, but others
need to be compiled as described in their respective project docs and
the compiled binary would need to be ported to `bin/` afterwards (can
be symlinked instead).

```sh
make mistserver
make go-livepeer
make livepeer-log
make livepeer-catalyst-node
```

## Booting up catalyst

Next is just booting up the catalyst system:

    make dev

Which will create a copy of developer config file to
`~/.config/livepeer/catalyst.json` (default path) and use that to boot
up `MistController`; which controls and monitors setting up other
required sub-systems.

The above command does some memory allocation on macOS too, but for
linux it is as simple as launching `MistController` with path to
configuration file.

You should then have a Mist interface accessible at
[http://localhost:4242](http://localhost:4242).

## Credentials/secrets

Mist credentials:

```
username: test
password: test
```

## Streaming and playback



## Manifest

The `manifest.yaml` file specifies which services are needed for the
system to bootup. The fields (with some description) are shown below:

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

      # Skip checking for newer releases (default is false)
      skipManifestUpdate: true
```

  [go-livepeer]: https://github.com/livepeer/go-livepeer
  [catalyst]: https://github.com/livepeer/catalyst
  [mistserver]: https://github.com/DDVTECH/mistserver
  [catalyst-api]: https://github.com/livepeer/catalyst-api
  [victoria-metrics]: https://github.com/VictoriaMetrics/VictoriaMetrics
