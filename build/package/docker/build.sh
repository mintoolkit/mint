#!/usr/bin/env bash

set -e

docker build --squash --rm -t mint -f Dockerfile ../../..
docker image prune --filter label=build-role=ca-certs -f
docker image prune --filter label=app=mint -f