#!/bin/sh

echo ">> Building contract"

near-sdk-js build src/contract.ts build/db3_near.wasm
