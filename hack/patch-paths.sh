#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 The provider-stripe authors
#
# SPDX-License-Identifier: Apache-2.0

# usage: patch-paths.sh PATCH
set -euo pipefail

allowed='^(apis/[^/]+/v1alpha1/[^/]+\.go|apis/zz_generated\.groups\.go|apis/v1beta1/zz_generated\.[^/]+|package/crds/.+|internal/controller/zz_setup\.go|examples/namespaced/.+|docs/delete-modes\.md|openapi/spec3\.json|openapi/VERSION|go\.mod|go\.sum)$'

paths=$(mktemp)
trap 'rm -f "$paths"' EXIT
git apply --numstat -z "$1" | sed -z 's/^[0-9-]*\t[0-9-]*\t//; /^$/d' | sort -zu > "$paths"

status=0
outside=$(grep -zEv "$allowed" "$paths" | tr '\0' '\n') || status=$?
case $status in
  0)
    echo "patch touches paths outside the generated set:"
    echo "$outside"
    exit 1
    ;;
  1) ;;
  *) exit "$status" ;;
esac
