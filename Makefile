.PHONY: all build run dev shell pull clean
all: build

build:
	docker build -t livepeer/in-a-box --build-arg MIST_URL=${MIST_URL} .

run:
	docker run --rm -it -p 8080:80 -p 1935:1935 -p 4242:4242 -v ${PWD}/data:/data --name=box livepeer/in-a-box

clean:
	docker run --rm -it -v ${PWD}/data:/data livepeer/in-a-box bash -c "rm -r /data/*"

dev: build run

pull:
	docker build --pull -t livepeer/in-a-box .

shell:
	docker exec -it box /bin/bash
