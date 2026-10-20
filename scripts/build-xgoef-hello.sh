#!/usr/bin/env bash
# Build examples/hello. Toolchain env is set here; the cmake / xgo steps
# live in `xgoef build` so there is one link path.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

export PATH="/usr/local/bin:/usr/local/go-1.27/bin:/usr/lib/llvm-22/bin:${HOME}/go/bin:${PATH}"
export LLGO_ROOT="${LLGO_ROOT:-/opt/xgo-toolchain/llgo}"
export LIBRARY_PATH="/usr/lib/gcc/x86_64-linux-gnu/14:/usr/lib/x86_64-linux-gnu${LIBRARY_PATH:+:$LIBRARY_PATH}"
unset CGO_CFLAGS CGO_LDFLAGS CGO_CXXFLAGS CGO_CPPFLAGS CGO_FFLAGS
export CGO_ENABLED=0
export XGO_GOCMD="${XGO_GOCMD:-$ROOT/scripts/xgo-llgo-gocmd.sh}"

cd "$ROOT"
exec go run ./cmd/xgoef build --app "$ROOT/examples/hello" --out "$ROOT/build/hello"
