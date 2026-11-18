#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 The provider-stripe authors
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

CLUSTER="${E2E_CLUSTER:-stripe-e2e}"
UPTEST_CLOUD_CREDENTIALS="$(cat test/harness/.out/credentials.json)"
export UPTEST_CLOUD_CREDENTIALS
mkdir -p _output
export KUBECONFIG="$PWD/_output/$CLUSTER.kubeconfig"
kind export kubeconfig --name "$CLUSTER" --kubeconfig "$KUBECONFIG"

make controlplane.up KIND_CLUSTER_NAME="$CLUSTER"
if [ -n "${E2E_PACKAGE:-}" ]; then
  crds=(crd/providers.pkg.crossplane.io crd/deploymentruntimeconfigs.pkg.crossplane.io)
  kubectl -n crossplane-system wait --for=condition=Available deployment --all --timeout=5m
  kubectl wait --for condition=Established --timeout=5m "${crds[@]}"
  {
    cat test/harness/runtimeconfig.yaml
    cat <<EOF
---
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-stripe
spec:
  package: $E2E_PACKAGE
  runtimeConfigRef:
    name: runtimeconfig-provider-stripe
EOF
  } | kubectl apply -f -
  kubectl wait provider.pkg provider-stripe --for condition=Healthy --timeout=5m
else
  # A content digest gives each source state its own package revision, so the wait below passes only once this build runs.
  hash="$(find apis internal cmd/provider package cluster/images go.mod go.sum -type f -print0 | sort -z | xargs -0 sha256sum | sha256sum | cut -d' ' -f1)"
  make build PLATFORMS=linux_amd64
  make e2e.provider KIND_CLUSTER_NAME="$CLUSTER" LOCAL_XPKG_DIGEST="sha256:$hash" DRC_FILE=test/harness/runtimeconfig.yaml
  kubectl wait provider.pkg provider-stripe --for=jsonpath='{.status.currentIdentifier}'="xpkg.crossplane.internal/dev/provider-stripe@sha256:$hash" --timeout=5m
fi
kubectl -n crossplane-system wait --for=condition=Available deployment --all --timeout=5m

mapfile -t examples < <(find examples/namespaced -path '*/v1alpha1/*.yaml' | sort)
if [ "${E2E_TARGET:-kind}" = saas ]; then
  mapfile -t examples < <(grep -LF 'stripe.m.clifton.quest/e2e-saas: skip' "${examples[@]}")
  STRIPE_API_KEY="$(jq -r .apiKey test/harness/.out/credentials.json)" test/harness/sweep.sh "${examples[@]}"
  lifecycle=()
else
  lifecycle=(UPTEST_SKIP_UPDATE=true UPTEST_SKIP_IMPORT=true UPTEST_SKIP_DELETE=true)
fi
for example in "${examples[@]}"; do
  make uptest UPTEST_INPUT_MANIFESTS="$example" "${lifecycle[@]}"
done
