#!/bin/sh
if [ "$1" = "--describe" ]; then
  printf '%s' '{"v":"weave.extension/v1","name":"single","version":"0.11.0"}'
  exit 0
fi
cat >/dev/null
if [ -n "$TEST_BATCH_MARKER" ]; then
  printf '1\n' >> "$TEST_BATCH_MARKER"
fi
printf '%s' '{"v":"weave.cli/v1","inferred_keywords":["single"]}'
