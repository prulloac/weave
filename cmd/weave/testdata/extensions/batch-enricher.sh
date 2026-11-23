#!/bin/sh
if [ "$1" = "--describe" ]; then
  printf '%s' '{"v":"weave.extension/v1","name":"batch","version":"0.11.0","capabilities":["batch"]}'
  exit 0
fi
input=$(cat)
count=$(printf '%s' "$input" | grep -o '"doc_id"' | wc -l | tr -d ' ')
if [ -n "$TEST_BATCH_MARKER" ]; then
  printf '%s\n' "$count" >> "$TEST_BATCH_MARKER"
fi
case "$TEST_BATCH_MODE" in
  timeout)
    sleep 2
    ;;
  malformed)
    printf '%s' '{"v":"weave.cli/v1","type":"batch_response","results":[]}'
    exit 0
    ;;
esac
printf '{"v":"weave.cli/v1","type":"batch_response","results":['
i=0
while [ "$i" -lt "$count" ]; do
  if [ "$i" -gt 0 ]; then
    printf ','
  fi
  if [ "$TEST_BATCH_MODE" = "item-error" ] && [ "$i" -eq 1 ]; then
    printf '{"ok":false,"error":{"code":"EXTENSION_FAILED","message":"second failed"}}'
  else
    printf '{"v":"weave.cli/v1","inferred_keywords":["batch"]}'
  fi
  i=$((i+1))
done
printf ']}'
