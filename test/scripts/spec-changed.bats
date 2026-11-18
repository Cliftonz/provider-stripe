#!/usr/bin/env bats
# SPDX-FileCopyrightText: 2026 The provider-stripe authors
#
# SPDX-License-Identifier: Apache-2.0

setup() {
  ROOT=$(cd "$BATS_TEST_DIRNAME/../.." && pwd)
  cd "$BATS_TEST_TMPDIR"
}

spec() {
  printf '{"openapi":"3.0.0","info":{"version":"%s"},"paths":{%s}}\n' "$1" "$2"
}

@test "reports unchanged when the candidate spec matches the pinned one" {
  spec 2026-09-30.endive > pinned.json
  spec 2026-09-30.endive > candidate.json
  run "$ROOT/hack/spec-changed.sh" pinned.json candidate.json
  [ "$status" -eq 0 ]
  [ "$output" = "$(printf 'changed=false\nold_version=2026-09-30.endive\nnew_version=2026-09-30.endive')" ]
}

@test "reports changed when the content differs under the same version" {
  spec 2026-09-30.endive > pinned.json
  spec 2026-09-30.endive '"/v1/products":{}' > candidate.json
  run "$ROOT/hack/spec-changed.sh" pinned.json candidate.json
  [ "$status" -eq 0 ]
  [ "$output" = "$(printf 'changed=true\nold_version=2026-09-30.endive\nnew_version=2026-09-30.endive')" ]
}

@test "reports changed with both versions when the version moves" {
  spec 2026-09-30.endive > pinned.json
  spec 2026-10-28.endive > candidate.json
  run "$ROOT/hack/spec-changed.sh" pinned.json candidate.json
  [ "$status" -eq 0 ]
  [ "$output" = "$(printf 'changed=true\nold_version=2026-09-30.endive\nnew_version=2026-10-28.endive')" ]
}

@test "fails without reporting when an input is missing" {
  spec 2026-09-30.endive > pinned.json
  run "$ROOT/hack/spec-changed.sh" pinned.json missing.json
  [ "$status" -ne 0 ]
  [[ "$output" != *changed=* ]]
}

@test "fails when a version cannot be read" {
  spec 2026-09-30.endive > pinned.json
  printf '{"info": [\n' > candidate.json
  run "$ROOT/hack/spec-changed.sh" pinned.json candidate.json
  [ "$status" -ne 0 ]
}
