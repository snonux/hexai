#!/usr/bin/env bash
set -euo pipefail

# Simple coverage helper focusing on new/modified packages.
# Usage: scripts/coverage.sh [packages...]

pkgs=("$@")
if [ ${#pkgs[@]} -eq 0 ]; then
  pkgs=(
    "codeberg.org/snonux/hexai/internal/tmux"
    "codeberg.org/snonux/hexai/internal/hexaiaction"
  )
fi

cover_dir="$(mktemp -d)"
trap 'rm -rf "$cover_dir"' EXIT

echo "Running coverage for packages:" "${pkgs[@]}"

total=0
for p in "${pkgs[@]}"; do
  out="$cover_dir/$(echo "$p" | tr '/' '_').out"
  go test -coverprofile="$out" -covermode=atomic "$p"
  echo "--- $p ---"
  go tool cover -func="$out" | tail -n1
done

echo
echo "Hint: combine coverage across all packages with:"
echo "  go test ./... -coverprofile=cover.out && go tool cover -func=cover.out"
