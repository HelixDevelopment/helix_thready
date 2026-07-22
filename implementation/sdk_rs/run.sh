#!/usr/bin/env bash
# Build the std-only Rust SDK test binary with rustc directly (NO cargo, NO
# crates) and run it. Physical evidence: a passing `test result: ok. N passed;
# 0 failed` line and exit 0.
set -euo pipefail
cd "$(dirname "$0")"
mkdir -p target
rustc --edition 2021 --test src/lib.rs -o target/testbin
./target/testbin
