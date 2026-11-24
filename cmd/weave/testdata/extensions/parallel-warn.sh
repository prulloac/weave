#!/bin/sh
if [ "$1" = "--describe" ]; then
  printf '%s' '{"v":"weave.extension/v1","name":"warn","version":"0.11.0"}'
  exit 0
fi
input=$(cat)
id=$(printf '%s' "$input" | sed -n 's/.*"doc_id":"\([^"]*\)".*/\1/p')
printf '{"v":"weave.cli/v1","inferred_keywords":["warn"],"warnings":["warn-%s"]}' "$id"
