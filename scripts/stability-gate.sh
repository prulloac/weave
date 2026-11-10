#!/usr/bin/env bash
# Report the 1.0 stability window for prulloac/weave.
# Usage: ./scripts/stability-gate.sh [window-start-date]
set -euo pipefail

repo="${WEAVE_REPO:-prulloac/weave}"
start="${1:-${STABILITY_START:-2026-11-10}}"
today="$(date -u +%F)"
start_epoch="$(date -u -d "$start" +%s)"
today_epoch="$(date -u -d "$today" +%s)"
days=$(( (today_epoch - start_epoch) / 86400 ))

open="$(gh issue list -R "$repo" --label confirmed-defect --state open --json number --jq 'length')"
echo "repository:   $repo"
echo "window start: $start"
echo "evaluated:    $today"
echo "days elapsed: $days"
echo "open confirmed defects: $open"

if [ "$open" -gt 0 ]; then
  echo "status: blocked — close or fix open confirmed defects first"
  exit 1
fi
if [ "$days" -lt 30 ]; then
  remaining=$(( 30 - days ))
  echo "status: not eligible — $remaining day(s) remaining"
  exit 1
fi
echo "status: eligible — confirm no private security defect reset the window"
