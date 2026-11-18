#!/usr/bin/env bats
# SPDX-FileCopyrightText: 2026 The provider-stripe authors
#
# SPDX-License-Identifier: Apache-2.0

setup() {
  ROOT=$(cd "$BATS_TEST_DIRNAME/../.." && pwd)
  mkdir "$BATS_TEST_TMPDIR/bin"
  PATH=$BATS_TEST_TMPDIR/bin:$PATH
}

stub_curl() {
  printf '#!/usr/bin/env bash\necho "$*" > "%s/curl-args"\n%s\n' "$BATS_TEST_TMPDIR" "$1" > "$BATS_TEST_TMPDIR/bin/curl"
  chmod +x "$BATS_TEST_TMPDIR/bin/curl"
}

@test "writes the Stripe spec at the given tag to the given path" {
  stub_curl "printf '{\"openapi\":\"3.0.0\"}\n'"
  run "$ROOT/hack/spec-fetch.sh" v2700 "$BATS_TEST_TMPDIR/spec3.json"
  [ "$status" -eq 0 ]
  [ "$(cat "$BATS_TEST_TMPDIR/spec3.json")" = '{"openapi":"3.0.0"}' ]
  grep -qF https://raw.githubusercontent.com/stripe/openapi/v2700/openapi/spec3.json "$BATS_TEST_TMPDIR/curl-args"
}

@test "fails when the download fails, leaving the existing file untouched and no temp file" {
  printf '{}\n' > "$BATS_TEST_TMPDIR/spec3.json"
  stub_curl "printf partial; exit 22"
  run "$ROOT/hack/spec-fetch.sh" v2700 "$BATS_TEST_TMPDIR/spec3.json"
  [ "$status" -eq 22 ]
  [ "$(cat "$BATS_TEST_TMPDIR/spec3.json")" = '{}' ]
  [ -z "$(compgen -G "$BATS_TEST_TMPDIR/spec3.json.*")" ]
}
