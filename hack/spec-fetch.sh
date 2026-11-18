#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 The provider-stripe authors
#
# SPDX-License-Identifier: Apache-2.0

# usage: spec-fetch.sh TAG OUT
set -euo pipefail

tmp=$(mktemp "$2.XXXXXX")
trap 'rm -f "$tmp"' EXIT
curl -fsSL "https://raw.githubusercontent.com/stripe/openapi/$1/openapi/spec3.json" > "$tmp"
mv "$tmp" "$2"
