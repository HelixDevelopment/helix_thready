#!/usr/bin/env bash
# Build the Helix Thready Java SDK and run its self-contained test runner.
# JDK 21 standard library only — no build tool, no external jars.
set -euo pipefail

cd "$(dirname "$0")"

rm -rf out
mkdir -p out

# Compile every source file (client + models + test runner) into out/.
javac -d out $(find src -name '*.java')

# Run the in-file assertion runner; it exits non-zero if any test failed.
java -cp out digital.vasic.thready.ThreadyClientTest
