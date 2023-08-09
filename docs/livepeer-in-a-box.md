# Livepeer in a Box

Livepeer in a Box is our development environment for the full Livepeer stack,
including Livepeer Studio and Livepeer Catalyst. We currently support Linux and
macOS hosts.

## Dependencies

You'll need the following things installed locally:

-   Docker (or Podman, which works even better)
-   Go v1.20+
-   Node.js v18+ (Studio development only)
-   Make
-   git

You'll first need to clone the Catalyst repo if you haven't already.

```shell
git clone git@github.com:livepeer/catalyst.git
cd catalyst
```

Then you'll need to download/build all the binaries that you'll need. On MacOS,
you'll need to export a GOOS variable so that you're downloading and
cross-compiling the Linux versions of binaries.

```shell
export GOOS=linux # only if you're not already running linux
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

-   URL: [http://127.0.0.1:8888](http://127.0.0.1:8888)
-   Email: `admin@example.com`
-   Password: `livepeer`

## Making changes

TLDR: Use a command like this and the Makefile will take care of it for you:

```shell
make livepeer-catalyst-api KILL=true
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
make livepeer-task-runner KILL=true
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
| [MistServer]\*              | `make mistserver`                 |

\* Cross-compilation of MistServer from macOS to Linux is currently not
supported. If you need to make changes on a Mac, you may have to use a dev
server or a Linux VM for now.

[catalyst-api]: https://github.com/livepeer/catalyst-api
[catalyst-uploader]: https://github.com/livepeer/catalyst-uploader
[task-runner]: https://github.com/livepeer/catalyst-uploader
[analyzer]: https://github.com/livepeer/livepeer-data
[Studio Node.js API Server]: https://github.com/livepeer/studio
[MistServer]: https://github.com/livepeer/mistserver

## Connecting the Frontend

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
localStorage.setItem("LP_API_SERVER_OVERRIDE", "http://127.0.0.1:8888");
```

Reload the page and your frontend should be connecting to the box as an API
server.

Additional note: in the interest of build speed, `make livepeer-api` does not
package the frontend within the `livepeer-api` binary that it builds, so if you
experience your frontend suddently 404ing after you run `make livepeer-api` you
will have to use the above instructions to boot up the frontend on your host.

## Notes

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

# Video

[This intro video](https://lvpr.tv?v=98c42pmz87zmy5rh) goes over everything you
need to get started with the Livepeer development environment, as well as
covering some of the background. Some timestamps:

-   `1:15`: Getting started
-   `5:43`: Making changes to applications and bundling them back in the box
-   `8:07`: Embedded nginx, adding new routes, MistServer config file
-   `10:13`: ~~Development on the Livepeer Studio API Server~~ Out of date;
    `make livepeer-api` now works in three seconds.
-   `14:03`: Running the Livepeer Studio frontend development server
