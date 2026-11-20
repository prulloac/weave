#!/bin/sh
if [ "$1" = "--describe" ]; then
  printf '%s' '{"v":"weave.extension/v1","name":"ident","kind":"codec","directions":["compress","decompress"]}'
  exit 0
fi
cat
