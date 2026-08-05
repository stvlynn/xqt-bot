#!/usr/bin/env bash
# Creates the KV namespace used by xqt-bot and patches wrangler.toml.
set -euo pipefail

TOML="wrangler.toml"

create_ns() {
  local args=("$@")
  wrangler kv namespace create "${args[@]}" 2>&1
}

echo "Creating production KV namespace..."
prod_out="$(create_ns KV)"
prod_id="$(printf '%s' "$prod_out" | sed -n 's/.*id = "\([a-f0-9]\{32\}\)".*/\1/p' | head -1)"

echo "Creating preview KV namespace..."
prev_out="$(create_ns KV --preview)"
prev_id="$(printf '%s' "$prev_out" | sed -n 's/.*id = "\([a-f0-9]\{32\}\)".*/\1/p' | head -1)"

if [[ -z "$prod_id" || -z "$prev_id" ]]; then
  echo "Failed to parse namespace ids. Raw output:" >&2
  echo "$prod_out" >&2
  echo "$prev_out" >&2
  exit 1
fi

sed -i '' "s/REPLACE_WITH_KV_ID/$prod_id/" "$TOML"
sed -i '' "s/REPLACE_WITH_KV_PREVIEW_ID/$prev_id/" "$TOML"

echo "Patched $TOML:"
echo "  id         = $prod_id"
echo "  preview_id = $prev_id"
