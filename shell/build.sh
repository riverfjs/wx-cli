#!/bin/bash
set -eo pipefail

cd "$(dirname "$0")/.."
echo "Building wx-cli..."
go build -ldflags="-s -w" -o bin/wx ./cmd/wx
echo "Done: bin/wx ($(du -h bin/wx | cut -f1))"
