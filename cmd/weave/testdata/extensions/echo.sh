#!/bin/sh
cat >/dev/null
if [ -n "$WEAVE_EXT_MARKER" ]; then
  echo invoked >> "$WEAVE_EXT_MARKER"
fi
printf '%s' '{"v":"weave.cli/v1","inferred_keywords":["enriched","sqlite","ALPHA"],"custom_metadata":{"summary":"enriched"}}'
