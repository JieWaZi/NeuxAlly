#!/bin/sh

set -eu

(
  cd server
  go generate ./constants/...
  go generate ./constants/status_error/...
  gofmt -w constants/*__generated.go constants/status_error/*__generated.go
  go generate ./cmd/ally-core
  mv cmd/ally-core/openapi.json openapi.json
)

node scripts/normalize-openapi.mjs server/openapi.json

ALLY_PUBLIC_SOURCE=$(mktemp)
trap 'rm -f "$ALLY_PUBLIC_SOURCE"' EXIT INT TERM
(cd server && go run ./cmd/integration-contract "$ALLY_PUBLIC_SOURCE")
node scripts/public-openapi.mjs "$ALLY_PUBLIC_SOURCE" server/integration-openapi.json
node scripts/normalize-openapi.mjs server/integration-openapi.json

pnpm --dir desktop api:generate
