#!/usr/bin/env bash

set -e

docker build --platform linux/arm64 -t mint-arm -f Dockerfile.arm ../../..
docker image prune --filter label=build-role=ca-certs -f
docker image prune --filter label=app=mint -f