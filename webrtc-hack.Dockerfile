FROM livepeerci/catalyst:latest

RUN apt update && apt install -y libsrtp2-1

COPY ./bin/MistOutWebRTC /usr/local/bin/MistOutWebRTC
