#!/usr/bin/env bats
# SPDX-FileCopyrightText: 2026 The provider-stripe authors
#
# SPDX-License-Identifier: Apache-2.0

setup() {
  ROOT=$(cd "$BATS_TEST_DIRNAME/../.." && pwd)
  export GIT_CONFIG_GLOBAL=$BATS_TEST_TMPDIR/gitconfig
  git config --global user.name test
  git config --global user.email test@example.org
  git init -q "$BATS_TEST_TMPDIR/repo"
  cd "$BATS_TEST_TMPDIR/repo"
}

patch() {
  for path in "$@"; do
    mkdir -p "$(dirname "$path")"
    echo changed > "$path"
  done
  git add -A
  git diff --cached --binary > "$BATS_TEST_TMPDIR/changes.patch"
}

@test "accepts a patch limited to the spec, generated code, CRDs, examples, delete modes and go modules" {
  patch openapi/spec3.json openapi/VERSION \
    package/crds/core.stripe.m.clifton.quest_products.yaml \
    apis/zz_generated.groups.go apis/v1beta1/zz_generated.deepcopy.go \
    apis/core/v1alpha1/product.go apis/core/v1alpha1/kit.go apis/core/v1alpha1/json.go \
    apis/core/v1alpha1/register.go apis/core/v1alpha1/zz_generated.deepcopy.go \
    internal/controller/zz_setup.go examples/namespaced/core/product.yaml \
    docs/delete-modes.md go.mod go.sum
  run "$ROOT/hack/patch-paths.sh" "$BATS_TEST_TMPDIR/changes.patch"
  [ "$status" -eq 0 ]
}

@test "refuses a patch touching other paths and names them" {
  patch openapi/spec3.json openapi/other.json apis/stripe.go apis/zz_generated.managed.go \
    apis/v1beta1/types.go apis/core/v1beta1/product.go apis/core/v1alpha1/sub/x.go hack/zz_generated.sh \
    internal/controller/kit/client.go examples/providerconfig.yaml docs/decisions.md .github/workflows/ci.yml
  run "$ROOT/hack/patch-paths.sh" "$BATS_TEST_TMPDIR/changes.patch"
  [ "$status" -ne 0 ]
  [ "$output" = "$(printf 'patch touches paths outside the generated set:\n.github/workflows/ci.yml\napis/core/v1alpha1/sub/x.go\napis/core/v1beta1/product.go\napis/stripe.go\napis/v1beta1/types.go\napis/zz_generated.managed.go\ndocs/decisions.md\nexamples/providerconfig.yaml\nhack/zz_generated.sh\ninternal/controller/kit/client.go\nopenapi/other.json')" ]
}

@test "refuses a rename out of an allowed path" {
  patch package/crds/a.yaml
  git commit -qm init
  mkdir hack
  git mv package/crds/a.yaml hack/a.sh
  git diff --cached -M --binary > "$BATS_TEST_TMPDIR/changes.patch"
  run "$ROOT/hack/patch-paths.sh" "$BATS_TEST_TMPDIR/changes.patch"
  [ "$status" -ne 0 ]
  [[ "$output" == *hack/a.sh* ]]
}

@test "refuses a path git quotes for its non-ASCII characters" {
  patch openapi/spec3.json "hack/$(printf '\303\274').sh"
  grep -qF '"a/hack/\303\274.sh"' "$BATS_TEST_TMPDIR/changes.patch"
  run "$ROOT/hack/patch-paths.sh" "$BATS_TEST_TMPDIR/changes.patch"
  [ "$status" -ne 0 ]
  [[ "$output" == *"hack/$(printf '\303\274').sh"* ]]
}

@test "fails when the patch is missing" {
  run "$ROOT/hack/patch-paths.sh" "$BATS_TEST_TMPDIR/missing.patch"
  [ "$status" -ne 0 ]
}

@test "fails when the patch is not a patch" {
  echo garbage > "$BATS_TEST_TMPDIR/changes.patch"
  run "$ROOT/hack/patch-paths.sh" "$BATS_TEST_TMPDIR/changes.patch"
  [ "$status" -ne 0 ]
}

@test "fails when the path filter errors" {
  patch openapi/spec3.json
  mkdir "$BATS_TEST_TMPDIR/bin"
  printf '#!/usr/bin/env bash\nexit 2\n' > "$BATS_TEST_TMPDIR/bin/grep"
  chmod +x "$BATS_TEST_TMPDIR/bin/grep"
  PATH=$BATS_TEST_TMPDIR/bin:$PATH run "$ROOT/hack/patch-paths.sh" "$BATS_TEST_TMPDIR/changes.patch"
  [ "$status" -eq 2 ]
}
