#!/bin/sh
if [ "$1" = "--describe" ]; then
printf '%s' '{"v":"weave.extension/v1","name":"helper","version":"0.2.0"}'
exit 0
fi
cat >/dev/null
printf '%s' '{"v":"weave.cli/v1","inferred_keywords":["helper"]}'
