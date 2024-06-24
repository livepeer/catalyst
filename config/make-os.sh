#!/bin/bash

set -euo pipefail

function makeStore() {
  PREFIX=$1
  PORT=$2
  result=$(curl --silent \
    -H "authorization: Bearer $LP_COM_TOKEN_ADMIN"  \
    http://localhost:8888/api/object-store \
    -X POST \
    -H "content-type: application/json" \
    --data-raw '{
      "url": "s3+http://admin:password@127.0.0.1:'$PORT'/'$PREFIX'",
      "publicUrl": "http://127.0.0.1:8888/'$PREFIX'"
    }')
  echo $result | jq -r '.id'
}

echo '"vodObjectStoreId": "'$(makeStore os-vod 9000)'",'
echo '"vodCatalystObjectStoreId": "'$(makeStore os-catalyst-vod 9000)'",'
echo '"vodCatalystPrivateAssetsObjectStore": "'$(makeStore os-private 9000)'",'
echo '"recordCatalystObjectStoreId": "'$(makeStore os-recordings 9420)'",'
