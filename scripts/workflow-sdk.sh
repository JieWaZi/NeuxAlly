#!/bin/sh
set -eu
cd "$(dirname "$0")/../server"
go run ./cmd/workflow-sdk "$@"
