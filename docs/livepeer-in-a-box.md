# Livepeer in a Box

Livepeer in a Box allows you to run the full Livepeer Studio stack as a single
Docker image. This stack includes Catalyst, MistServer, the Studio API server,
and a packaged Studio frontend.

What the box does:

-   Boots up a full-stack Livepeer experience on your laptop with a single
    command
-   Facilitates easy development of various components of the Livepeer stack
-   Allows for development of applications against the Livepeer Studio API
    locally that can then transfer to the hosted version at
    [livepeer.studio](https://livepeer.studio) when you're ready to go to
    production
-   Bundles a fully-local offchain go-livepeer broadcaster and orchestrator, so
    that you may test transcoding with no external dependencies

What the box doesn't do (yet):

-   Allow for easy deployment to a server. There are presently many hardcoded
    references to `localhost`, and these things will break in any other
    environment.
-   No multi-user streaming (Livekit) integration
-   No usage or billing data
-   No GPU transcoding support. We recommend using very low-bitrate test files,
    especially if running the box using Docker for Mac or Docker for Windows.
    The built-in profiles for livestream transcoding use a single 240p
    low-quality rendition.

## Running the Box

First, select a directory for persisting your database and video content; in
this example we will be using `$HOME/livepeer-in-a-box`.

```shell
BOX_DIR="$HOME/livepeer-in-a-box"
mkdir -p $BOX_DIR
docker run \
	-v $BOX_DIR:/data \
	--rm \
	-it \
	--name box \
	--shm-size=4gb \
	-p 8888:8888 \
	-p 5432:5432 \
	-p 1935:1935 \
	-p 4242:4242 \
	-p 3478:3478 \
	-p 3478:3478/udp \
	-p 5349:5349 \
	-p 40000-40100:40000-40100/udp \
	livepeer/in-a-box
```

You will be greeted with a very large amount of spam — give it a minute or so to
boot up. You can then connect to your local box instance:

Address: [https://localhost:8888](https://localhost:8888)  
Email: `admin@example.com`  
Password: `livepeer`

To get you started, the database snapshot includes a few predefined streams.

| Stream           | Stream Key          | Playback ID  | Recording enabled? |
| ---------------- | ------------------- | ------------ | ------------------ |
| [tiny-transcode] | 2222-2222-2222-2222 | 222222222222 | No                 |
| [tiny-recording] | 4444-4444-4444-4444 | 444444444444 | Yes                |

[tiny-transcode]:
    http://localhost:8888/dashboard/streams/22222222-2222-2222-2222-222222222222
[tiny-recording]:
    http://localhost:8888/dashboard/streams/44444444-4444-4444-4444-444444444444

For properly testing a livestream input comparable to OBS output, you will want
a low-bitrate test file with no B-Frames and a short GOP length.
[Here's a sample appropriately-formatted Big Buck Bunny file you can use](BBB).
To stream in to your local box, you can use an `ffmpeg` command such as:

```shell
curl -LO https://test-harness-gcp.livepeer.fish/Big_Buck_Bunny_360p_1sGOP_NoBFrames.mp4
ffmpeg -stream_loop -1 -re -i Big_Buck_Bunny_360p_1sGOP_NoBFrames.mp4 -c copy -f flv rtmp://localhost/live/2222-2222-2222-2222
```

[BBB]:
    https://test-harness-gcp.livepeer.fish/Big_Buck_Bunny_360p_1sGOP_NoBFrames.mp4

## Developing with the Box

Developing using the box is currently supported on macOS and Linux.

### Dependencies

You'll need the following things installed locally:

-   Docker (or Podman 4.6.0+, which works even better)
-   Buildah (for `docker buildx`, included with Podman and Docker for Mac)
-   Go v1.20+
-   Node.js v18+ (Studio development only)
-   Make
-   git
-   llvm (`brew install llvm` for compiling MistServer on MacOS only)

You'll first need to clone the Catalyst repo if you haven't already.

```shell
git clone https://github.com/livepeer/catalyst.git
cd catalyst
```

Then you'll need to download/build all the binaries that you'll need.

```shell
make              # downloads all the external binaries & builds the local ones
make box          # builds the livepeer/in-a-box Docker image
```

## Development

Now you're ready to boot up your development environment!

```shell
make box-dev
```

Lots and lots of logs will print out while all the dependencies finish booting
up. After that, you'll now have a fully-functioning full-stack Livepeer Studio +
Catalyst environment running locally! You can access it like so:

-   URL: [http://localhost:8888](http://localhost:8888)
-   Email: `admin@example.com`
-   Password: `livepeer`

### Making changes

TLDR: Use a command like this and the Makefile will take care of it for you:

```shell
make livepeer-catalyst-api
```

The general Livepeer in a Box development cycle works like this:

1. Make changes to code on your local filesystem
2. Build a Linux binary from that code
3. Move that Linux binary into the `bin` directory of `catalyst`, which is
   mounted by `make box-dev`
4. Kill the old version of your binary and allow MistController to bring it back
   up.

Thankfully, this entire process has been automated. All you need to do is have
the project you're working on cloned in a directory adjacent to `catalyst`. For
example, if you're hacking on `task-runner`, you might have

```
/home/user/code/catalyst
/home/user/code/task-runner
```

The catalyst Makefile is aware of of the common paths for all of the other
projects that go into the full stack. All that's necessary to build a new
binary, package it in the container, and trigger a restart is a single command:

```shell
make livepeer-task-runner
```

Note that the names of all subprojects are prefixed with `livepeer`, just like
the resulting binaries within the Catalyst container. This yields the following
commands:

| Project                     | Command                           |
| --------------------------- | --------------------------------- |
| [catalyst-api]              | `make livepeer-catalyst-api`      |
| [catalyst-uploader]         | `make livepeer-catalyst-uploader` |
| [task-runner]               | `make livepeer-task-runner`       |
| [analyzer]                  | `make livepeer-analyzer`          |
| [Studio Node.js API Server] | `make livepeer-api`               |
| [MistServer]                | `make mistserver`                 |

[catalyst-api]: https://github.com/livepeer/catalyst-api
[catalyst-uploader]: https://github.com/livepeer/catalyst-uploader
[task-runner]: https://github.com/livepeer/catalyst-uploader
[analyzer]: https://github.com/livepeer/livepeer-data
[Studio Node.js API Server]: https://github.com/livepeer/studio
[MistServer]: https://github.com/livepeer/mistserver

### Connecting the Frontend

Livepeer in a Box comes with a [pkg](https://github.com/vercel/pkg)-bundled
version of the Livepeer Studio API server and frontend, but does not include a
full development environment for that frontend. If you are making changes to the
frontend, you can boot it up as you usually would:

```
cd studio/packages/www
yarn run dev
```

To connect it to the box; there's a hidden `localStorage` variable you can use
to override the API server URL. Open your browser console and type in the
following:

```javascript
localStorage.setItem("LP_API_SERVER_OVERRIDE", "http://localhost:8888");
```

Reload the page and your frontend should be connecting to the box as an API
server.

Additional note: in the interest of build speed, `make livepeer-api` does not
package the frontend within the `livepeer-api` binary that it builds, so if you
experience your frontend suddently 404ing after you run `make livepeer-api` you
will have to use the above instructions to boot up the frontend on your host.

You can also build the full API server with a bundled frontend using
`make livepeer-api-pkg`, but be aware this frequently takes 3-4 minutes to
complete.

### Notes

-   Your CockroachDB (Postgres) database and your Minio (S3) object store will
    be saved in the `data` subdirectory of your Catalyst installation. If you
    want to start from scratch again with the `admin@example.com` database
    snapshot, shut down your box and `rm -rf data`.
-   You can press `Ctrl+C` to trigger a graceful shutdown of the container. If
    you're impatient, following it up with a `Ctrl+\` can uncleanly shut things
    down a bit more cleanly.
-   Sometimes the rate of logs produced by Catalyst somehow overwhelms Make and
    log output simply stops. You'll know if you get in this state because you'll
    press Ctrl+C and control will return immediately to your terminal instead of
    shutting down the Docker image. You can start everything back up with
    `docker rm -f catalyst` and `make box-dev`.

## Changelog

### 2023-08-12

-   Changed the hardcoded streams in the database snapshots to have
    easy-to-remember stream keys like `2222-2222-2222-2222`
-   Changed the built-in streams to use the H264ConstrainedHigh profile so there
    are no B-Frames in the output
-   Moved all references from `127.0.0.1` to `localhost`; this is needed for
    WebRTC/Coturn to work properly
-   Removed outdated references to `GOOS=linux` and `KILL=true`; these are the
    defaults now
