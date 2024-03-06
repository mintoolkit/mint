#!/usr/bin/env bash

function uninstall_mint() {
  local VER=""

  # /usr/local/bin should be present on Linux and macOS hosts. Just be sure.
  if [ -d /usr/local/bin ]; then
    VER=$(mint --version | cut -d'|' -f3)
    echo " - Uninstalling version - ${VER}"

    echo " - Removing mint binaries from /usr/local/bin"
    rm /usr/local/bin/mint
    rm /usr/local/bin/mint-sensor
    rm /usr/local/bin/slim
    rm /usr/local/bin/docker-slim

    echo " - Removing local state directory"
    rm -rfv /tmp/mint-state

    echo " - Removing state volume"
    docker volume rm mint-state

    echo " - Removing sensor volume"
    docker volume rm mint-sensor.${VER}
  else
    echo "ERROR! /usr/local/bin is not present. Uninstall aborted."
    exit 1
  fi
}

echo "Mint scripted uninstall"

if [ "$(id -u)" -ne 0 ]; then
  echo "ERROR! You must run this script as root."
  exit 1
fi

uninstall_mint
