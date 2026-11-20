#!/bin/sh
if [ "$1" = "--describe" ]; then
printf '%s' '{"v":"weave.extension/v1","name":"helper"}'
exit 0
fi
cat >/dev/null
printf '%s' '{"v":"weave.cli/v1","inferred_keywords":["helper"]}'
