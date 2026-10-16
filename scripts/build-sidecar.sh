#!/bin/sh
set -eu

ally_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
exec node "$ally_root/scripts/build-sidecar.mjs" "$@"
