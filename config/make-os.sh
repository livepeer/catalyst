#!/bin/bash

set -euo pipefail

function makeStore() {
  PREFIX=$1
  result=$(curl --silent \
    -H "authorization: Bearer $LP_COM_TOKEN_ADMIN"  \
    http://localhost:8888/api/object-store \
    -X POST \
    -H "content-type: application/json" \
    --data-raw '{
      "url": "s3+http://admin:password@127.0.0.1:9000/'$PREFIX'",
      "publicUrl": "http://127.0.0.1:8888/'$PREFIX'"
    }')
  echo $result | jq -r '.id'
}

echo '"vodObjectStoreId": "'$(makeStore os-vod)'",'
echo '"vodCatalystObjectStoreId": "'$(makeStore os-catalyst-vod)'",'
echo '"vodCatalystPrivateAssetsObjectStore": "'$(makeStore os-private)'",'
echo '"recordCatalystObjectStoreId": "'$(makeStore os-recordings)'",'
