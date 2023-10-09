FROM ubuntu:20.04

RUN apt update && apt install -y libc6-dev build-essential libgcc-9-dev
