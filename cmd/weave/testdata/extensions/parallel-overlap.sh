#!/bin/sh
if [ "$1" = "--describe" ]; then
  printf '%s' '{"v":"weave.extension/v1","name":"overlap","version":"0.11.0"}'
  exit 0
fi
cat >/dev/null
mkdir -p "$TEST_ACTIVE"
touch "$TEST_ACTIVE/$$"
ls "$TEST_ACTIVE" | wc -l >> "$TEST_MAX"
sleep 0.2
rm -f "$TEST_ACTIVE/$$"
printf '%s' '{"v":"weave.cli/v1","inferred_keywords":["overlap"]}'
