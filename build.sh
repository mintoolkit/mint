#!/usr/bin/env bash

set -e

SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
BDIR="$( cd -P "$( dirname "$SOURCE" )" && pwd )"

export CGO_ENABLED=0

BUILD_TIME="$(date -u '+%Y-%m-%d_%I:%M:%S%p')"
TAG="current"
REVISION="current"
if hash git 2>/dev/null && [ -e $BDIR/.git ]; then
  TAG="$(git describe --tags --always)"
  REVISION="$(git rev-parse HEAD)"
fi

LD_FLAGS="-s -w -X github.com/mintoolkit/mint/pkg/version.appVersionTag=${TAG} -X github.com/mintoolkit/mint/pkg/version.appVersionRev=${REVISION} -X github.com/mintoolkit/mint/pkg/version.appVersionTime=${BUILD_TIME}"

pushd ${BDIR}/cmd/mint
GOOS=linux GOARCH=amd64 go build -mod=vendor -trimpath -ldflags="${LD_FLAGS}" -a -tags 'remote netgo osusergo containers_image_openpgp containers_image_docker_daemon_stub containers_image_fulcio_stub containers_image_rekor_stub' -o "${BDIR}/bin/linux/mint"
GOOS=darwin GOARCH=amd64 go build -mod=vendor -trimpath -ldflags="${LD_FLAGS}" -a -tags 'remote netgo osusergo containers_image_openpgp containers_image_docker_daemon_stub containers_image_fulcio_stub containers_image_rekor_stub' -o "${BDIR}/bin/mac/mint"
GOOS=linux GOARCH=arm go build -mod=vendor -trimpath -ldflags="${LD_FLAGS}" -a -tags 'remote netgo osusergo containers_image_openpgp containers_image_docker_daemon_stub containers_image_fulcio_stub containers_image_rekor_stub' -o "$BDIR/bin/linux_arm/mint"
GOOS=linux GOARCH=arm64 go build -mod=vendor -trimpath -ldflags="${LD_FLAGS}" -a -tags 'remote netgo osusergo containers_image_openpgp containers_image_docker_daemon_stub containers_image_fulcio_stub containers_image_rekor_stub' -o "$BDIR/bin/linux_arm64/mint"
popd

pushd ${BDIR}/cmd/mint-sensor
GOOS=linux GOARCH=amd64 go build -mod=vendor -trimpath -ldflags="${LD_FLAGS}" -a -tags 'netgo osusergo containers_image_openpgp containers_image_docker_daemon_stub containers_image_fulcio_stub containers_image_rekor_stub' -o "${BDIR}/bin/linux/mint-sensor"
GOOS=linux GOARCH=arm go build -mod=vendor -trimpath -ldflags="${LD_FLAGS}" -a -tags 'netgo osusergo containers_image_openpgp containers_image_docker_daemon_stub containers_image_fulcio_stub containers_image_rekor_stub' -o "$BDIR/bin/linux_arm/mint-sensor"
GOOS=linux GOARCH=arm64 go build -mod=vendor -trimpath -ldflags="${LD_FLAGS}" -a -tags 'netgo osusergo containers_image_openpgp containers_image_docker_daemon_stub containers_image_fulcio_stub containers_image_rekor_stub' -o "$BDIR/bin/linux_arm64/mint-sensor"
popd
