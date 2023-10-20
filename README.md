# Catalyst

The entire bundle needed for booting up Livepeer's decentralised media
server in place is termed as [Catalyst].

## Getting Started

Make sure you have the following installed on the machine before
proceeding.

> Note: Compiling Mistserver/go-livpeeer etc have another set of
> requirements, not listed here (but covered under compile from source
> section below).

### Mandatory requirements

-   go1.17± or newer
-   GNU make
-   about 2-3 GB of free disk space

### Optional

-   ffmpeg v5 or newer (can be compiled from [go-livepeer], by running
    `make ffmpeg`)
-   Docker version 19 or newer
-   cloning this repository alongside [go-livepeer], [MistServer] and
    other dependent projects of [Catalyst] in the same
    directory. Directories would look like:

        /path/to/projects
                    |- catalyst/
                    |- go-livepeer/
                    |- mistserver/

±: go1.17 is recommended because building with < 1.17 (specifically 1.16)
can result in issues on arm64 architectures.

## Setting up Catalyst system

### First run (or reset workflow)

Run `make full-reset` which cleans out any older config files, and
local copies of binaries. It also shuts down `docker-compose` system
which aren't needed for base Catalyst usage.

### Using prebuilt binaries (download-and-run)

This section covers using prebuilt binaries for setting up and booting
Catalyst system. This is done through using the configuration present
in `manifest.yaml` (section below covers its structure).

Run the following commands from the root of this repository directory:

    go run cmd/downloader/downloader.go

    # or, single command:
    make all

Other options available for downloader script can be checked with
using `--help`.

The above should download all prebuilt binaries to the `bin/`
directory.

### Compile from source (build-and-run)

This section covers building the dependent projects before booting up
[Catalyst]. There are quite a few applications needed for Catalyst as
listed below (but not limited to):

-   [go-livepeer]
-   [MistServer]
-   [catalyst-api]
-   [victoria-metrics] (recommended to just download a stable binary
    from github releases)

A few of these can be compiled using `Makefile` recipes, but others
need to be compiled as described in their respective project docs and
the compiled binary would need to be ported to `bin/` afterwards (can
be symlinked instead).

#### Requirements for compiling MistServer

-   git
-   gcc or clang
-   CMake
-   about 4 GB of free disk space

#### Requirements for compiling [go-livepeer]

-   go1.17 (for arm64 build support)
-   git
-   GNU make
-   CMake
-   gcc-multilib
-   autoconf
-   pkg-config

```sh
make mistserver
make go-livepeer
```

## Running Catalyst

> The command does some memory allocation on macOS too, but for linux
> it is as simple as launching `MistController` with path to
> configuration file.

Next is just booting up the Catalyst system:

    make dev

Which will create a copy of developer config file to
`~/.config/livepeer/catalyst.json` (default path) and use that to boot
up `MistController`; which controls and monitors setting up other
required sub-systems. An example of `MistController` starting up
Livepeer broadcaster is shown in logs as follows:

```text
[2022-09-22 15:32:30] MistController (2244175) CONF: Starting connector: {"broadcaster":true,"connector":"livepeer","metricsClientIP":true,"metricsPerStream":true,"monitor":true,"orchAddr":"localhost:8936","rtmpAddr":"127.0.0.1:1936"}
[2022-09-22 15:32:30] MistController (2244175) INFO: added logger for livepeer
[2022-09-22 15:32:30] livepeer (2244394) INFO: I0922 15:32:30.537623 2244406 starter.go:333] ***Livepeer is running on the offchain network***
[2022-09-22 15:32:30] livepeer (2244394) INFO: I0922 15:32:30.539038 2244406 census.go:323] Compiler: gc Arch amd64 OS linux Go version go1.17.13
[2022-09-22 15:32:30] livepeer (2244394) INFO: I0922 15:32:30.539078 2244406 census.go:324] Livepeer version: 0.5.34-80e70628
[2022-09-22 15:32:30] livepeer (2244394) INFO: I0922 15:32:30.539101 2244406 census.go:325] Node type bctr node ID livepeer.system
[2022-09-22 15:32:30] livepeer (2244394) INFO: I0922 15:32:30.539449 2244406 starter.go:491] ***Livepeer is in off-chain mode***
[2022-09-22 15:32:30] livepeer (2244394) INFO: I0922 15:32:30.539595 2244406 starter.go:1156] ***Livepeer Running in Broadcaster Mode***
[2022-09-22 15:32:30] livepeer (2244394) INFO: I0922 15:32:30.539667 2244406 starter.go:1157] Video Ingest Endpoint - rtmp://127.0.0.1:1936
```

You should then have a Mist interface accessible at
[http://localhost:4242](http://localhost:4242).

### Credentials/secrets

MistServer credentials:

```
username: test
password: test
```

## Hot reloading

If a binary is replaced in the `bin/` directory, no need to shutdown
the whole catalyst process, and simply running for eg. `killall
livepeer` for restarting livepeer binary works. `MistController` keeps
track of all sub-systems and boots them back up in case they exit.

## Streaming

This section will cover stream and playback using HLS to local
Catalyst system.

### Using `ffmpeg`

The RTMP stream endpoint is accessible at
`rtmp://localhost/live/video+<PLAYBACK_ID>`, so an example stream
could be started (using ffmpeg) as follows:

```sh
ffmpeg -re -stream_loop -1 \
  -i "/path/to/video/file.mp4" \
  -c:v copy -c:a copy -strict -2 \
  -f flv \
  rtmp://localhost/live/video+foo
```

where the `PLAYBACK_ID` is `foo`.

### Using OBS Studio

Under OBS Studio streaming settings, select "Service" as "Custom..."
which should display text input options asking for "Server" and
"Stream Key" (ref. screenshot below). Use following values for the
same:

```text
Server: rtmp://localhost/live
Stream Key: video+<PLAYBACK_ID>
```

Deselect the option to "Use authentication". The example (as shown in
screenshot) uses `video+foo` as the "Stream Key".

[![obs-studio]][obs-studio]

## Playback

Playback over HLS is available through m3u8 playlist file accessible
at `http://localhost:8080/hls/video+<PLAYBACK_ID>/index.m3u8`. For the
example stream above, playback can be started with:

```sh
ffplay http://localhost:8080/hls/video+foo/index.m3u8
```

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
[mistserver]: https://github.com/livepeer/mistserver
[catalyst-api]: https://github.com/livepeer/catalyst-api
[victoria-metrics]: https://github.com/VictoriaMetrics/VictoriaMetrics
[obs-studio]: .github/assets/obs-studio.png
