#!/bin/bash -x

if [ $# -eq 0 ]; then
    echo "Missing output directory as argument"
    exit 1
fi

# Get one of the container IDs
CID=$(docker ps | grep catalyst | awk '{ print $1 }' | head -1)

# Copy Mist tester binaries to CI host
docker cp "$CID":/usr/bin/MistLoadTest /$1
docker cp "$CID":/usr/bin/MistAnalyserHLS /$1 
