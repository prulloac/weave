#!/bin/sh
cat >/dev/null
printf '%s' '{"v":"weave.cli/v1","ranked_candidates":[{"id":"c","rerank_score":1.0},{"id":"b","rerank_score":0.5},{"id":"a","rerank_score":0.0}]}'
