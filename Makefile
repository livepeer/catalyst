.PHONY: all
all:
	docker build -t livepeer/in-a-box --build-arg MIST_URL=${MIST_URL} .
