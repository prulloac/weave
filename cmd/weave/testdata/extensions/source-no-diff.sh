#!/bin/sh
if [ "$1" = "--describe" ]; then
  printf '%s' '{"v":"weave.extension/v1","name":"git","version":"0.11.0","protocols":["weave.source/v1"],"source_providers":["git"],"capabilities":["repository"]}'
  exit 0
fi
cat >/dev/null
printf '%s' '{"v":"weave.source/v1","type":"source_diff_response","from":"a","to":"b","changes":[]}'
