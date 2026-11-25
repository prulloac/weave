#!/bin/sh
name=$(basename "$0")
name=${name#weave-}
if [ "$1" = "--describe" ]; then
  version=${TEST_TOKENIZER_VERSION:-0.13.0}
  cache=""
  if [ -n "$TEST_TOKENIZER_CACHE_KEY" ]; then
    cache=",\"cache_key\":\"$TEST_TOKENIZER_CACHE_KEY\""
  fi
  printf '{"v":"weave.extension/v1","name":"%s","version":"%s"%s,"roles":["tokenizer"]}' \
    "$name" "$version" "$cache"
  exit 0
fi
input=$(cat)
if [ -n "$TEST_TOKENIZER_MARKER" ]; then
  printf '%s\n' "$input" >> "$TEST_TOKENIZER_MARKER"
fi
if [ "$TEST_TOKENIZER_SLEEP" = "1" ]; then
  sleep 3
fi
if [ "$TEST_TOKENIZER_INVALID" = "1" ]; then
  printf '%s' '{"v":"weave.cli/v1","type":"tokenizer_response"}'
  exit 0
fi
mode=$(printf '%s' "$input" | sed -n 's/.*"mode":"\([^"]*\)".*/\1/p')
text=$(printf '%s' "$input" | sed -n 's/.*"text":"\([^"]*\)".*/\1/p')
words=$(printf '%s' "$text" | tr '[:upper:]' '[:lower:]' | tr -cs '[:alnum:]' ' ')
terms=""
for word in $words; do
  case "$word" in
    running) word=run ;;
    mice) word=mouse ;;
  esac
  if [ -n "$terms" ]; then
    terms="$terms,"
  fi
  terms="$terms\"$word\""
done
expansions=""
if [ "$mode" = "query" ]; then
  case " $words " in
    *" db "*) expansions=',"expansions":{"db":["database"]}' ;;
  esac
fi
printf '{"v":"weave.cli/v1","type":"tokenizer_response","terms":[%s]%s}' "$terms" "$expansions"
