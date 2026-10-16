#!/bin/sh

set -eu

workspace_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
exec node "$workspace_dir/scripts/generate-app-icons.mjs"
