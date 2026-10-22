#!/bin/sh

set -eu

ALLY_API_TOOL_DIRECTORY=$(mktemp -d)
cleanup() {
  rm -rf "$ALLY_API_TOOL_DIRECTORY"
}
trap cleanup EXIT HUP INT TERM

(
  cd server
  # 生成器跟随仓库锁定的 ginx 版本，避免机器上的旧 toolx 漏识别路由。
  ALLY_API_GINX_VERSION=$(go list -m -f '{{.Version}}' github.com/shrewx/ginx)
  ALLY_API_GO_VERSION=$(go env GOVERSION)
  GOBIN="$ALLY_API_TOOL_DIRECTORY" GOTOOLCHAIN="$ALLY_API_GO_VERSION" \
    go install "github.com/shrewx/ginx/pkg/toolx@$ALLY_API_GINX_VERSION"
  PATH="$ALLY_API_TOOL_DIRECTORY:$PATH"
  export PATH
  go generate ./constants/...
  go generate ./constants/status_error/...
  gofmt -w constants/*__generated.go constants/status_error/*__generated.go
  go generate ./cmd/ally-core
  mv cmd/ally-core/openapi.json openapi.json
)

node scripts/normalize-openapi.mjs server/openapi.json

pnpm --dir desktop api:generate
