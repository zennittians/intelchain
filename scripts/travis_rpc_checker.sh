#!/usr/bin/env bash
set -e
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
docker pull intelchainitc/localnet-test
docker run -v "$DIR/../:/go/src/github.com/zennittians/intelchain" intelchainitc/localnet-test -n