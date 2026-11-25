#!/bin/sh
if [ "$1" = "--describe" ]; then
  cache_key=""
  if [ -n "$TEST_CACHE_KEY" ]; then
    cache_key=",\"cache_key\":\"$TEST_CACHE_KEY\""
  fi
  version=${TEST_CACHE_VERSION:-0.11.0}
  printf '%s' "{\"v\":\"weave.extension/v1\",\"name\":\"cache\",\"version\":\"$version\"$cache_key}"
  exit 0
fi
input=$(cat)
if [ -n "$TEST_CACHE_MARKER" ]; then
  printf '1\n' >> "$TEST_CACHE_MARKER"
fi
if [ "$TEST_CACHE_FAIL" = "1" ]; then
  echo "cache fixture failed" >&2
  exit 1
fi
workspace=$(printf '%s' "$input" | sed -n 's/.*"workspace":"\([^"]*\)".*/\1/p')
if [ "$TEST_CACHE_FILES" = "1" ] && [ -n "$workspace" ]; then
  mkdir -p "$workspace/files"
  printf 'cache-file\n' > "$workspace/files/data.bin"
  printf '%s' '{"v":"weave.extension-data/v1","items":[{"key":"data.bin","path":"files/data.bin","media_type":"application/octet-stream"}]}' > "$workspace/manifest.json"
fi
printf '%s' '{"v":"weave.cli/v1","inferred_title":"Cached Title","inferred_keywords":["cache"],"custom_metadata":{"source":"cache"}}'
