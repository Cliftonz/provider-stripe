#!/usr/bin/env bats
# SPDX-FileCopyrightText: 2026 The provider-stripe authors
#
# SPDX-License-Identifier: Apache-2.0

setup() {
  ROOT=$(cd "$BATS_TEST_DIRNAME/../.." && pwd)
  cd "$BATS_TEST_TMPDIR"
  mkdir -p test/harness/.out
  cp "$ROOT/test/harness/e2e.sh" "$ROOT/test/harness/runtimeconfig.yaml" test/harness/
  echo '{"apiKey":"sk_test_abc"}' > test/harness/.out/credentials.json
  printf '#!/usr/bin/env bash\necho "$STRIPE_API_KEY $*" >> %s/sweep.log\n' "$PWD" > test/harness/sweep.sh
  chmod +x test/harness/sweep.sh
  example core product
  example billing meter
  stub kind 'exit 0'
  stub make "echo \"\$*\" >> $PWD/make.log"
  stub kubectl "echo \"\$*\" >> $PWD/kubectl.log; if [ \"\$1\" = apply ]; then cat >> $PWD/applied.yaml; fi"
}

stub() {
  mkdir -p bin
  printf '#!/usr/bin/env bash\n%s\n' "$2" > "bin/$1"
  chmod +x "bin/$1"
}

example() {
  mkdir -p "examples/namespaced/$1/v1alpha1"
  echo 'kind: X' > "examples/namespaced/$1/v1alpha1/$2.yaml"
}

e2e() {
  PATH="$PWD/bin:$PATH" E2E_PACKAGE=xpkg.upbound.io/cliftonz/provider-stripe:v9.9.9 run env "$@" test/harness/e2e.sh
}

@test "package mode installs the given package instead of building" {
  e2e -u E2E_TARGET
  [ "$status" -eq 0 ]
  grep -q 'package: xpkg.upbound.io/cliftonz/provider-stripe:v9.9.9' applied.yaml
  grep -q 'kind: DeploymentRuntimeConfig' applied.yaml
  grep -q 'wait provider.pkg provider-stripe --for condition=Healthy' kubectl.log
  ! grep -q '^build' make.log
}

@test "kind mode runs uptest on every example asserting Ready only, without sweeping" {
  e2e -u E2E_TARGET
  [ "$status" -eq 0 ]
  skip='UPTEST_SKIP_UPDATE=true UPTEST_SKIP_IMPORT=true UPTEST_SKIP_DELETE=true'
  [ "$(grep '^uptest' make.log)" = "$(printf 'uptest UPTEST_INPUT_MANIFESTS=examples/namespaced/billing/v1alpha1/meter.yaml %s\nuptest UPTEST_INPUT_MANIFESTS=examples/namespaced/core/v1alpha1/product.yaml %s' "$skip" "$skip")" ]
  [ ! -e sweep.log ]
}

@test "saas mode sweeps the example kinds with the harness key, then runs the full uptest lifecycle on every example" {
  e2e E2E_TARGET=saas
  [ "$status" -eq 0 ]
  [ "$(cat sweep.log)" = "sk_test_abc examples/namespaced/billing/v1alpha1/meter.yaml examples/namespaced/core/v1alpha1/product.yaml" ]
  [ "$(grep '^uptest' make.log)" = "$(printf 'uptest UPTEST_INPUT_MANIFESTS=examples/namespaced/billing/v1alpha1/meter.yaml\nuptest UPTEST_INPUT_MANIFESTS=examples/namespaced/core/v1alpha1/product.yaml')" ]
}

@test "saas mode stops before uptest when the sweep fails" {
  printf '#!/usr/bin/env bash\nexit 1\n' > test/harness/sweep.sh
  e2e E2E_TARGET=saas
  [ "$status" -ne 0 ]
  ! grep -q '^uptest' make.log
}

skipped_in_saas() {
  mkdir -p examples/namespaced/issuing/v1alpha1
  printf 'kind: Card\nmetadata:\n  annotations:\n    stripe.m.clifton.quest/e2e-saas: skip\n' > examples/namespaced/issuing/v1alpha1/card.yaml
}

@test "saas mode leaves examples annotated to skip SaaS out of the sweep and uptest" {
  skipped_in_saas
  e2e E2E_TARGET=saas
  [ "$status" -eq 0 ]
  [ "$(cat sweep.log)" = "sk_test_abc examples/namespaced/billing/v1alpha1/meter.yaml examples/namespaced/core/v1alpha1/product.yaml" ]
  ! grep -q card.yaml make.log
}

@test "kind mode still runs examples annotated to skip SaaS" {
  skipped_in_saas
  e2e -u E2E_TARGET
  [ "$status" -eq 0 ]
  grep -q '^uptest UPTEST_INPUT_MANIFESTS=examples/namespaced/issuing/v1alpha1/card.yaml ' make.log
}
