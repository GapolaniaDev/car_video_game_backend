#!/usr/bin/env bash
# scripts/gen-proto.sh — regenerate Go code from protocol/game.proto.
#
# Requirements:
#   * protoc        — https://grpc.io/docs/protoc-installation/
#   * protoc-gen-go — go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
#
# Generated code lands in protocol/gamepb/game.pb.go and is committed
# to source control so that contributors do not need protoc locally to
# build.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$ROOT"

if ! command -v protoc >/dev/null 2>&1; then
    echo "protoc not found in PATH" >&2
    exit 1
fi

if ! command -v protoc-gen-go >/dev/null 2>&1; then
    echo "protoc-gen-go not found in PATH (go install google.golang.org/protobuf/cmd/protoc-gen-go@latest)" >&2
    exit 1
fi

mkdir -p protocol/gamepb

protoc \
    --proto_path=protocol \
    --go_out=protocol/gamepb \
    --go_opt=paths=source_relative \
    game.proto

echo "regenerated protocol/gamepb/game.pb.go"