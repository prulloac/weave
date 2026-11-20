#!/bin/sh
if [ "$1" = "--describe" ]; then
printf '%s' '{"v":"weave.extension/v1","name":"satisfied","version":"0.1.0","requires":[{"name":"weave","min_version":"0.8.0"}]}'
exit 0
fi
cat >/dev/null
printf '%s' '{"v":"weave.cli/v1","inferred_keywords":["gate"]}'
