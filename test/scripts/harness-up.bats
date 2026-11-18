#!/usr/bin/env bats
# SPDX-FileCopyrightText: 2026 The provider-stripe authors
#
# SPDX-License-Identifier: Apache-2.0

setup() {
  ROOT=$(cd "$BATS_TEST_DIRNAME/../.." && pwd)
  cd "$BATS_TEST_TMPDIR"
  cp -r "$ROOT/test/harness" harness
  rm -rf harness/.out
  stub kind "echo \"\$*\" >> $PWD/kind.log"
  stub kubectl "echo \"\$*\" >> $PWD/kubectl.log; if [ \"\${*: -3}\" = 'apply -f -' ]; then cat >> $PWD/applied.yaml; fi"
}

stub() {
  mkdir -p bin
  printf '#!/usr/bin/env bash\n%s\n' "$2" > "bin/$1"
  chmod +x "bin/$1"
}

up() {
  PATH="$PWD/bin:$PATH" run env "$@" harness/up.sh
}

@test "kind mode deploys stripe-mock at the pinned image and points the provider at it" {
  up -u E2E_TARGET
  [ "$status" -eq 0 ]
  image=$(sed -n 's/^STRIPE_MOCK_IMAGE=//p' harness/images.env)
  grep -qx "          image: $image" applied.yaml
  grep -q 'create cluster --name stripe-e2e' kind.log
  [ "$(jq -c . harness/.out/credentials.json)" = '{"apiKey":"sk_test_123","baseURL":"http://stripe-mock.stripe-mock:12111"}' ]
}

@test "saas mode writes the test-mode key as the provider credentials without deploying stripe-mock" {
  up E2E_TARGET=saas STRIPE_API_KEY=sk_test_abc
  [ "$status" -eq 0 ]
  [ "$(jq -c . harness/.out/credentials.json)" = '{"apiKey":"sk_test_abc"}' ]
  [ ! -e applied.yaml ]
}

@test "saas mode accepts a restricted test-mode key" {
  up E2E_TARGET=saas STRIPE_API_KEY=rk_test_abc
  [ "$status" -eq 0 ]
  [ "$(jq -r .apiKey harness/.out/credentials.json)" = rk_test_abc ]
}

@test "saas mode refuses a live key before creating the cluster" {
  for key in sk_live_abc rk_live_abc pk_test_abc ""; do
    up E2E_TARGET=saas STRIPE_API_KEY="$key"
    [ "$status" -ne 0 ]
  done
  up -u STRIPE_API_KEY E2E_TARGET=saas
  [ "$status" -ne 0 ]
  [ ! -e kind.log ]
  [ ! -e harness/.out/credentials.json ]
}
