# livepeer-in-a-box

### Getting Started

You'll presently need the following repos cloned into the same directory as livepeer-in-a-box:

-   [go-livepeer](https://github.com/livepeer/go-livepeer) (any branch, only there for `install_ffmpeg.sh`)
-   [mistserver](https://github.com/DDVTECH/mistserver) (`livepeer-in-a-box` branch)
-   [task-runner](https://github.com/livepeer/task-runner) (whichever branch you want to use)

From there:

```
make docker-compose # boots up Postgres and RabbitMQ dependencies
make # downloads or builds services as appropriate
make dev
```

You should then have a web interface running at [http://localhost:3004](http://localhost:3004) and a Mist interface at [http://localhost:4242](http://localhost:4242).

Livepeer credentials: User `admin@livepeer.local`, password `livepeer`.
Mist credentials: User `test`, password `test`
