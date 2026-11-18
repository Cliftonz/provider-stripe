#!/usr/bin/env bats
# SPDX-FileCopyrightText: 2026 The provider-stripe authors
#
# SPDX-License-Identifier: Apache-2.0

setup() {
  ROOT=$(cd "$BATS_TEST_DIRNAME/../.." && pwd)
  mkdir "$BATS_TEST_TMPDIR/bin" "$BATS_TEST_TMPDIR/api"
  PATH=$BATS_TEST_TMPDIR/bin:$PATH
  CALLS=$BATS_TEST_TMPDIR/calls
  : > "$CALLS"
  printf '#!/usr/bin/env bash\necho "git $*" >> "%s"\ncat "%s/tags"\n' "$CALLS" "$BATS_TEST_TMPDIR" > "$BATS_TEST_TMPDIR/bin/git"
  printf '#!/usr/bin/env bash\necho "curl $*" >> "%s"\n%s\n' "$CALLS" "url=\${!#}; tag=\${url#*/stripe-go/}; tag=\${tag%%/*}
    printf 'package stripe\n\nconst (\n\tAPIVersion      string = \"%s\"\n\tAPIMajorVersion string = \"endive\"\n)\n' \"\$(cat '$BATS_TEST_TMPDIR/api/'\$tag)\"" > "$BATS_TEST_TMPDIR/bin/curl"
  chmod +x "$BATS_TEST_TMPDIR/bin/git" "$BATS_TEST_TMPDIR/bin/curl"
  cd "$BATS_TEST_TMPDIR"
  printf 'module example.org/p\n\nrequire github.com/stripe/stripe-go/v87 v87.0.0\n' > go.mod
}

releases() {
  : > "$BATS_TEST_TMPDIR/tags"
  while [ $# -gt 0 ]; do
    printf '%040d\trefs/tags/%s\n' 0 "$1" >> "$BATS_TEST_TMPDIR/tags"
    echo "$2" > "$BATS_TEST_TMPDIR/api/$1"
    shift 2
  done
}

@test "prints the newest stripe-go release pinned to the given API version when stripe-go is ahead of the spec" {
  releases v9.0.0 2019-01-01 v87.0.0 2026-09-30.endive v87.1.0 2026-10-28.endive \
    v87.1.1 2026-10-28.endive v87.2.0 2026-11-17.endive v87.10.0 2026-11-30.endive v87.3.0-beta.1 2026-11-17.preview
  run "$ROOT/hack/stripe-go-for-api.sh" 2026-10-28.endive
  [ "$status" -eq 0 ]
  [ "$output" = v87.1.1 ]
  grep -qxF "git ls-remote --tags --refs https://github.com/stripe/stripe-go" "$CALLS"
  grep -qF "https://raw.githubusercontent.com/stripe/stripe-go/v87.10.0/api_version.go" "$CALLS"
  ! grep -q "beta" "$CALLS"
}

@test "fails naming the API version when no stripe-go release is pinned to it" {
  releases v87.0.0 2026-09-30.endive v87.1.0 2026-10-28.endive
  run "$ROOT/hack/stripe-go-for-api.sh" 2026-10-15.endive
  [ "$status" -ne 0 ]
  [ "$output" = "no stripe-go release has APIVersion 2026-10-15.endive" ]
}

@test "fails when an api_version.go download fails" {
  releases v87.0.0 2026-09-30.endive
  printf '#!/usr/bin/env bash\nexit 22\n' > "$BATS_TEST_TMPDIR/bin/curl"
  run "$ROOT/hack/stripe-go-for-api.sh" 2026-09-30.endive
  [ "$status" -eq 22 ]
}

@test "fails naming the API version and the new major when the matching release moves to a newer major" {
  releases v87.1.0 2026-10-28.endive v88.0.0 2027-03-31.fig v88.1.0 2027-04-30.fig
  run "$ROOT/hack/stripe-go-for-api.sh" 2027-03-31.fig
  [ "$status" -ne 0 ]
  [ "$output" = "stripe-go for APIVersion 2027-03-31.fig is v88.0.0, major v88 differs from pinned v87 in go.mod" ]
}

@test "fails naming the API version and the other major when the matching release is on an older major" {
  releases v86.5.0 2026-08-27.dahlia v87.0.0 2026-09-30.endive
  run "$ROOT/hack/stripe-go-for-api.sh" 2026-08-27.dahlia
  [ "$status" -ne 0 ]
  [ "$output" = "stripe-go for APIVersion 2026-08-27.dahlia is v86.5.0, major v86 differs from pinned v87 in go.mod" ]
}

@test "takes the pinned major from the require line when go.mod also mentions stripe-go elsewhere" {
  releases v87.0.0 2026-09-30.endive
  printf 'module example.org/p\n\nrequire (\n\tgithub.com/stripe/stripe-go/v87 v87.0.0\n)\n\nreplace github.com/stripe/stripe-go/v86 v86.0.0 => ./fork\n' > go.mod
  run "$ROOT/hack/stripe-go-for-api.sh" 2026-09-30.endive
  [ "$status" -eq 0 ]
  [ "$output" = v87.0.0 ]
}

@test "fails when no stripe-go release tags are listed" {
  releases
  run "$ROOT/hack/stripe-go-for-api.sh" 2026-09-30.endive
  [ "$status" -ne 0 ]
  [ "$output" = "no stripe-go release tags found" ]
}

@test "fails before looking up releases when go.mod does not require stripe-go" {
  releases v87.0.0 2026-09-30.endive
  printf 'module example.org/p\n\nreplace github.com/stripe/stripe-go/v87 v87.0.0 => ./fork\n' > go.mod
  run "$ROOT/hack/stripe-go-for-api.sh" 2026-09-30.endive
  [ "$status" -ne 0 ]
  [ "$output" = "go.mod does not require github.com/stripe/stripe-go" ]
  [ ! -s "$CALLS" ]
}

@test "fails naming the tag when its api_version.go has no APIVersion" {
  releases v87.0.0 2026-09-30.endive
  printf '#!/usr/bin/env bash\nprintf "package stripe\\n"\n' > "$BATS_TEST_TMPDIR/bin/curl"
  run "$ROOT/hack/stripe-go-for-api.sh" 2026-09-30.endive
  [ "$status" -ne 0 ]
  [ "$output" = "no APIVersion in stripe-go v87.0.0 api_version.go" ]
}
