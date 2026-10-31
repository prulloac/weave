#!/bin/sh
input=$(cat)
case "$input" in
  *zanzibar-secret*) printf '%s' '{"v":"weave.cli/v1","inferred_keywords":["leaked"]}' ;;
  *) printf '%s' '{"v":"weave.cli/v1","inferred_keywords":["safe"]}' ;;
esac
