#!/bin/sh
if [ "$1" = "--describe" ]; then
  printf '%s' '{"v":"weave.extension/v1","name":"rerank","version":"0.11.0","protocols":["weave.cli/v1"],"roles":["rerank"]}'
  exit 0
fi
cat >/dev/null
printf '%s' '{"v":"weave.cli/v1","ranked_candidates":[{"id":"docs/adr-004.md","rerank_score":0.9},{"id":"docs/adr-001.md","rerank_score":0.1}]}'
