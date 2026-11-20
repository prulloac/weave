#!/bin/sh
if [ "$1" = "--describe" ]; then
  printf '%s' '{"v":"weave.extension/v1","name":"code","analysis_modes":["project"]}'
  exit 0
fi
cat >/dev/null
printf '%s' '{"v":"weave.cli/v1","documents":[],"relations":[]}'
