#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 The provider-stripe authors
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

cd "$(dirname "$0")"
set -a
# shellcheck source=images.env
. images.env
set +a

CLUSTER="${E2E_CLUSTER:-stripe-e2e}"
if [ "${E2E_TARGET:-kind}" = saas ] && ! [[ ${STRIPE_API_KEY:-} =~ ^(sk|rk)_test_ ]]; then
  echo "STRIPE_API_KEY must be a Stripe test-mode key (sk_test_ or rk_test_)" >&2
  exit 1
fi

mkdir -p .out
if ! kind get clusters | grep -qx "$CLUSTER"; then
  kind create cluster --name "$CLUSTER" --image "$KIND_NODE_IMAGE"
fi

if [ "${E2E_TARGET:-kind}" = saas ]; then
  jq -n --arg apiKey "$STRIPE_API_KEY" '{$apiKey}' > .out/credentials.json
  exit
fi

envsubst '${STRIPE_MOCK_IMAGE}' < manifests/stripe-mock.yaml | kubectl --context "kind-$CLUSTER" apply -f -
kubectl --context "kind-$CLUSTER" --namespace stripe-mock wait --for=condition=Available deployment/stripe-mock --timeout=5m
jq -n '{apiKey: "sk_test_123", baseURL: "http://stripe-mock.stripe-mock:12111"}' > .out/credentials.json
echo "wrote $(pwd)/.out/credentials.json"
