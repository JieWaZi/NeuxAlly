#!/bin/sh

set -eu

# 使用 go.mod 锁定的生成器，避免开发机旧全局 toolx 静默遗漏路由。
ALLY_GENERATOR_DIR=$(mktemp -d)
ALLY_PUBLIC_SOURCE=$(mktemp)
trap 'rm -rf "$ALLY_GENERATOR_DIR"; rm -f "$ALLY_PUBLIC_SOURCE"' EXIT INT TERM
(
  cd server
  ALLY_TOOLX_VERSION=$(go list -m -f '{{.Version}}' github.com/shrewx/ginx)
  GOBIN="$ALLY_GENERATOR_DIR" go install "github.com/shrewx/ginx/pkg/toolx@$ALLY_TOOLX_VERSION"
)
PATH="$ALLY_GENERATOR_DIR:$PATH"
export PATH

(
  cd server
  go generate ./constants/...
  go generate ./constants/status_error/...
  gofmt -w constants/*__generated.go constants/status_error/*__generated.go
  go generate ./cmd/ally-core
  mv cmd/ally-core/openapi.json openapi.json
)

node scripts/normalize-openapi.mjs server/openapi.json

(cd server && go run ./cmd/integration-contract "$ALLY_PUBLIC_SOURCE")
node scripts/public-openapi.mjs "$ALLY_PUBLIC_SOURCE" server/integration-openapi.json
node scripts/normalize-openapi.mjs server/integration-openapi.json

pnpm --dir packages/frontend api:generate
