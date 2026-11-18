#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 The provider-stripe authors
#
# SPDX-License-Identifier: Apache-2.0

# usage: stripe-go-for-api.sh API_VERSION
set -euo pipefail

pinned=$(sed -n -e '/^require (/,/^)/p' -e '/^require [^(]/p' go.mod |
  sed -n 's|^[[:space:]]*\(require \)\{0,1\}github.com/stripe/stripe-go/\(v[0-9]*\) .*|\2|p')
if [ -z "$pinned" ]; then
  echo "go.mod does not require github.com/stripe/stripe-go" >&2
  exit 1
fi

tags=$(git ls-remote --tags --refs https://github.com/stripe/stripe-go |
  sed -n 's|.*refs/tags/\(v[0-9]*\.[0-9]*\.[0-9]*\)$|\1|p' | sort -rV)
if [ -z "$tags" ]; then
  echo "no stripe-go release tags found" >&2
  exit 1
fi
for tag in $tags; do
  api=$(curl -fsSL "https://raw.githubusercontent.com/stripe/stripe-go/$tag/api_version.go" |
    sed -n 's/^[[:space:]]*APIVersion[[:space:]].*"\(.*\)"$/\1/p')
  if [ -z "$api" ]; then
    echo "no APIVersion in stripe-go $tag api_version.go" >&2
    exit 1
  fi
  if [ "$api" = "$1" ]; then
    if [ "${tag%%.*}" != "$pinned" ]; then
      echo "stripe-go for APIVersion $1 is $tag, major ${tag%%.*} differs from pinned $pinned in go.mod" >&2
      exit 1
    fi
    echo "$tag"
    exit 0
  fi
  # API versions start with their release date, so once one sorts below the target no older release can match.
  if [[ "$api" < "$1" ]]; then
    break
  fi
done
echo "no stripe-go release has APIVersion $1" >&2
exit 1
