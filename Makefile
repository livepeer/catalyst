.PHONY: all build run dev shell
all: build

build:
	docker build -t livepeer/in-a-box --build-arg MIST_URL=${MIST_URL} .

run:
	docker run --rm -it -p 8080:80 -p 9000:9000 --name=box livepeer/in-a-box

dev: build run

shell:
	docker exec -it box /bin/bash
