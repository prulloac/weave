#!/bin/sh
if [ "$1" = "--describe" ]; then
  printf '%s' '{"v":"weave.extension/v1","name":"git","version":"0.11.0","protocols":["weave.source/v1"],"source_providers":["git"],"capabilities":["repository","diff"]}'
  exit 0
fi
cat >/dev/null
case "$TEST_DIFF_MODE" in
  fail)
    echo "provider failed" >&2
    exit 1
    ;;
  timeout)
    sleep 2
    ;;
  malformed)
    printf '%s' '{"v":"weave.source/v1","type":"source_diff_response","from":"a","to":"b","changes":[{"kind":"teleported","path":"x.md"}]}'
    exit 0
    ;;
esac
printf '%s' '{"v":"weave.source/v1","type":"source_diff_response","from":"1111111","to":"2222222","changes":[{"kind":"added","path":"docs/new.md","patch":"diff --git a/docs/new.md b/docs/new.md\nnew file mode 100644\n--- /dev/null\n+++ b/docs/new.md\n@@ -0,0 +1 @@\n+new\n"},{"kind":"modified","path":"docs/changed.md","patch":"diff --git a/docs/changed.md b/docs/changed.md\n--- a/docs/changed.md\n+++ b/docs/changed.md\n@@ -1 +1 @@\n-alpha\n+beta\n"},{"kind":"renamed","path":"docs/new-name.md","previous_path":"docs/old-name.md","patch":"diff --git a/docs/old-name.md b/docs/new-name.md\nsimilarity index 100%%\nrename from docs/old-name.md\nrename to docs/new-name.md\n"},{"kind":"added","path":"assets/logo.bin","binary":true}]}'
