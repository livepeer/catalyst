#!/bin/bash
# start minio server
minio server /data --console-address ":9001"&
sleep 5
# download management utility
if [[ ! -e "/data/mc" ]]; then
  curl https://dl.min.io/client/mc/release/linux-amd64/mc --create-dirs -o /data/mc
  chmod +x /data/mc
else
  # wait a bit until the server is online
  sleep 5
fi
cd /data
# set credentials
./mc alias set local http://localhost:9000/ minioadmin minioadmin
# create test bucket
./mc mb test_bucket
# create service account
./mc admin user svcacct add                    \
     --access-key "xZ1cZfALGp32hxpP"          \
     --secret-key "Vw52yWwNnwabX7pHcLvxXddKlfoZ9L59"  \
   local minioadmin

sleep infinity
