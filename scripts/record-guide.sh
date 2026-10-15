#!/usr/bin/env bash
set -euo pipefail

# Run from any directory; optional arguments are tape basenames without .tape.
repo_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd -- "$repo_dir"

for tool in go python3 vhs ffmpeg ttyd hx tmux task fish; do
    if ! command -v "$tool" >/dev/null; then
        printf 'Missing recording dependency: %s\n' "$tool" >&2
        exit 1
    fi
done

build_dir=$(mktemp -d -t hexai-guide-bin-XXXXXX)
trap 'rm -r -- "$build_dir"' EXIT
for binary in hexai hexai-lsp-server hexai-tmux-action ask; do
    go build -o "$build_dir/$binary" "./cmd/$binary"
done

python3 -B docs/guide/recordings/record.py "$build_dir" "$@"
